package operator

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
)

func sysctl(ctx context.Context, log *slog.Logger) {
	cmd := exec.CommandContext(ctx, "sysctl", "-w", "net.ipv6.conf.all.forwarding=1")
	if err := cmd.Run(); err != nil {
		log.Error("failed to run sysctl", "error", err)
	}
}

var NFTInitOnce = sync.Once{}

func nftInit() {
	cmd := exec.Command("nft", "add", "table", "inet", "filter")
	if err := cmd.Run(); err != nil {
		slog.Error("failed to add table inet filter", "error", err)
	}

	cmd = exec.Command("nft", "add", "chain", "inet", "filter", "postrouting", "{ type nat hook postrouting priority 100 ; }")
	if err := cmd.Run(); err != nil {
		slog.Error("failed to add chain inet filter postrouting", "error", err)
	}

	cmd = exec.Command("nft", "add", "rule", "inet", "filter", "postrouting", "oifname", "eth0", "masquerade")
	if err := cmd.Run(); err != nil {
		slog.Error("failed to add rule inet filter postrouting oifname eth0 masquerade", "error", err)
	}
}

func nftSync(ctx context.Context, log *slog.Logger, config *Config, deviceName string) {
	ruleNameToDestinations := make(map[string][]net.IPNet)
	for _, rr := range config.Rules {

		nets := []net.IPNet{}
		for _, d := range rr.Spec.Destinations {
			_, ipnet, err := net.ParseCIDR(d)
			if err != nil {
				log.Error(err.Error())
				continue
			}
			nets = append(nets, *ipnet)
		}
		ruleNameToDestinations[rr.Name] = nets
	}

	log.Debug("ruleNameToDestinations created")

	nft, err := nftables.New()
	if err != nil {
		panic(err)
	}
	defer nft.Flush()

	table, err := checkOrCreateTable(nft)
	if err != nil {
		panic(err)
	}

	log.Debug("table checked or created")

	chain, err := checkOrCreateWGAIngressChain(ctx, nft, table, deviceName)
	if err != nil {
		panic(err)
	}

	log.Debug("chain checked or created")

	rules, err := nft.GetRules(table, chain)
	if err != nil {
		panic(err)
	}

	ruleMap := make(map[string]*nftables.Rule)
	for _, r := range rules {
		ruleMap[string(r.UserData)] = r
	}

	log.Debug("ruleMap created")

	for _, peer := range config.Peers {
		if peer.Status == nil {
			// will be reconciled later
			continue
		}

		snet := net.IPNet{
			IP:   net.ParseIP(peer.Status.Address),
			Mask: net.CIDRMask(128, 128),
		}

		for _, name := range peer.Spec.AccessRules {
			for _, dnet := range ruleNameToDestinations[name] {

				comment := "r" + strip(snet.String()+dnet.String())

				exists := false
				for ud := range ruleMap {
					if strings.Contains(ud, comment) {
						delete(ruleMap, ud)
						exists = true
					}
				}
				if exists {
					continue
				}

				if len(peer.Status.DNS) == 0 {
					log.ErrorContext(ctx, "peer has no DNS", "peer", peer.Name)
					continue
				}

				nft.AddRule(routingRule(table, chain, snet, dnet, comment))
				nft.AddRule(dnsRule(table, chain, peer.Status.DNS[0], snet, comment))
				nft.AddRule(httpRule(table, chain, peer.Status.DNS[0], snet, comment))
				err = nft.Flush()
				if err != nil {
					log.WarnContext(ctx, "error flushing nftables", "err", err)
					continue
				}

				log.Debug("rules added")
			}
		}
	}

	log.Debug("rules added")

	for _, stale := range ruleMap {
		err := nft.DelRule(stale)
		if err != nil {
			log.WarnContext(ctx, "error deleting stale rule", "err", err, "rule", stale)
		}
	}

	log.Debug("stale rules deleted")
}

func strip(s string) string {
	var result strings.Builder
	for i := 0; i < len(s); i++ {
		b := s[i]
		if ('a' <= b && b <= 'z') ||
			('A' <= b && b <= 'Z') ||
			('0' <= b && b <= '9') {
			result.WriteByte(b)
		}
	}
	return result.String()
}

func checkOrCreateTable(nft *nftables.Conn) (*nftables.Table, error) {
	tables, err := nft.ListTables()
	if err != nil {
		return nil, err
	}

	for _, t := range tables {
		if t.Name == "wga" {
			return t, nil
		}
	}

	return nft.AddTable(&nftables.Table{
		Family: nftables.TableFamilyNetdev,
		Name:   "wga",
	}), nft.Flush()
}

func checkOrCreateWGAIngressChain(ctx context.Context, nft *nftables.Conn, table *nftables.Table, deviceName string) (
	*nftables.Chain, error,
) {
	chains, err := nft.ListChains()
	if err != nil {
		return nil, err
	}

	for _, c := range chains {
		if c.Name == deviceName && c.Table.Name == table.Name {
			return c, nil
		}
	}

	cmd := exec.CommandContext(ctx, "nft", "add", "chain", "netdev", deviceName, deviceName, "{ type filter hook ingress device "+deviceName+" priority 0 ; policy drop; }")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return nil, err
	}

	chains, err = nft.ListChains()
	if err != nil {
		return nil, err
	}

	for _, c := range chains {
		if c.Name == deviceName && c.Table.Name == table.Name {
			return c, nil
		}
	}

	return nil, nil
}

// dnsRule is equivalent to
//
//	nft add rule netdev <table_name> <chain_name> \
//	    ip6 saddr <source_ip> \
//	    ip6 daddr <dns_ip> \
//	    meta l4proto udp \
//	    udp dport 53 \
//	    counter \
//	    accept \
//	    comment "<comment>"
func dnsRule(table *nftables.Table, chain *nftables.Chain, DNS string, snet net.IPNet, comment string) *nftables.Rule {
	return &nftables.Rule{
		Table: table,
		Chain: chain,
		Exprs: []expr.Any{
			// Match source IP
			&expr.Payload{
				DestRegister: 1,
				Base:         expr.PayloadBaseNetworkHeader,
				Offset:       8,
				Len:          16,
			},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     snet.IP,
			},
			// Match destination IP
			&expr.Payload{
				DestRegister: 1,
				Base:         expr.PayloadBaseNetworkHeader,
				Offset:       24,
				Len:          16,
			},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     net.ParseIP(DNS).To16(),
			},
			// Match UDP protocol
			&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     []byte{17}, // 17 is UDP
			},
			// Match destination port 53
			&expr.Payload{
				DestRegister: 1,
				Base:         expr.PayloadBaseTransportHeader,
				Offset:       2,
				Len:          2,
			},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     []byte{0, 53}, // Port 53 in big-endian
			},
			&expr.Counter{},
			&expr.Verdict{Kind: expr.VerdictAccept},
		},
		UserData: []byte(comment),
	}
}

// httpRule is equivalent to
//
//	nft add rule netdev <table_name> <chain_name> \
//	    ip6 saddr <source_ip> \
//	    ip6 daddr <dns_ip> \
//	    meta l4proto tcp \
//	    tcp dport { 80, 443 } \
//	    counter \
//	    accept \
//	    comment "<comment>"
func httpRule(table *nftables.Table, chain *nftables.Chain, DNS string, snet net.IPNet, comment string) *nftables.Rule {
	return &nftables.Rule{
		Table: table,
		Chain: chain,
		Exprs: []expr.Any{
			// Match source IP
			&expr.Payload{
				DestRegister: 1,
				Base:         expr.PayloadBaseNetworkHeader,
				Offset:       8,
				Len:          16,
			},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     snet.IP,
			},
			// Match destination IP
			&expr.Payload{
				DestRegister: 1,
				Base:         expr.PayloadBaseNetworkHeader,
				Offset:       24,
				Len:          16,
			},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     net.ParseIP(DNS).To16(),
			},
			// Match TCP protocol
			&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     []byte{6}, // 6 is TCP
			},
			// Match destination ports 80 or 443
			&expr.Payload{
				DestRegister: 1,
				Base:         expr.PayloadBaseTransportHeader,
				Offset:       2,
				Len:          2,
			},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     []byte{0, 80}, // Port 80 in big-endian
			},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     []byte{1, 187}, // Port 443 in big-endian
			},
			&expr.Counter{},
			&expr.Verdict{Kind: expr.VerdictAccept},
		},
		UserData: []byte(comment),
	}
}

// routingRule is equivalent to
//
//	nft add rule netdev <table_name> <chain_name> \
//	    ip6 saddr <source_ip> \
//	    ip6 daddr <destination_ip> \
//	    counter \
//	    accept \
//	    comment "<comment>"
func routingRule(table *nftables.Table, chain *nftables.Chain, snet net.IPNet, dnet net.IPNet, comment string) *nftables.Rule {
	return &nftables.Rule{
		Table: table,
		Chain: chain,
		Exprs: []expr.Any{
			// Match source IP
			&expr.Payload{
				DestRegister: 1,
				Base:         expr.PayloadBaseNetworkHeader,
				Offset:       8,  // Source address field in IPv6 header
				Len:          16, // IPv6 address length
			},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     snet.IP,
			},

			// Match destination IP
			&expr.Payload{
				DestRegister: 1,
				Base:         expr.PayloadBaseNetworkHeader,
				Offset:       24, // Destination address field in IPv6 header
				Len:          16, // IPv6 address length
			},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     dnet.IP.To16(),
			},

			// Counter
			&expr.Counter{},

			// Accept
			&expr.Verdict{
				Kind: expr.VerdictAccept,
			},
		},
		UserData: []byte(comment),
	}
}
