package operator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/google/nftables"
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

//TODO: this doesnt scale and should be replaced with a map

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

		if len(peer.Status.Addresses) == 0 {
			peer.Status.Addresses = []string{peer.Status.Address}
		}

		for _, addr := range peer.Status.Addresses {

			isV6 := true
			ip := net.ParseIP(addr)
			mask := net.CIDRMask(128, 128)
			if ip.To4() == nil {
				mask = net.CIDRMask(128, 128)
			} else {
				mask = net.CIDRMask(32, 32)
				isV6 = false
			}

			snet := net.IPNet{
				IP:   ip,
				Mask: mask,
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

					err = routingRule(ctx, table, chain, isV6, snet, dnet, comment)
					if err != nil {
						log.ErrorContext(ctx, "failed to add routing rule", "peer", peer.Name, "err", err)
						continue
					}

					log.Debug("rules added")
				}
			}

			comment := "r" + strip(snet.String()+peer.Status.DNS[0])
			exists := false
			for ud := range ruleMap {
				if strings.Contains(ud, comment) {
					delete(ruleMap, ud)
					exists = true
				}
			}
			if !exists {
				err = dnsRule(ctx, table, chain, peer.Status.DNS[0], snet, comment)
				if err != nil {
					log.ErrorContext(ctx, "failed to add dns rule", "peer", peer.Name, "err", err)
					continue
				}
				err = httpRule(ctx, table, chain, peer.Status.DNS[0], snet, comment)
				if err != nil {
					log.ErrorContext(ctx, "failed to add http rule", "peer", peer.Name, "err", err)
					continue
				}
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

func dnsRule(ctx context.Context, table *nftables.Table, chain *nftables.Chain, DNS string, snet net.IPNet, comment string) error {
	err := exec.CommandContext(
		ctx, "nft", "add", "rule", "netdev", table.Name, chain.Name,
		"ip6", "saddr", snet.String(),
		"ip6", "daddr", DNS,
		"udp", "dport", "53",
		"counter",
		"accept",
		"comment", comment,
	).Run()
	if err != nil {
		exitError := &exec.ExitError{}
		if errors.As(err, &exitError) {
			return fmt.Errorf("nftables error: %s", string(exitError.Stderr))
		}

		return err
	}

	return nil
}

func httpRule(ctx context.Context, table *nftables.Table, chain *nftables.Chain, DNS string, snet net.IPNet, comment string) error {
	err := exec.CommandContext(
		ctx, "nft", "add", "rule", "netdev", table.Name, chain.Name,
		"ip6", "saddr", snet.String(),
		"ip6", "daddr", DNS,
		"tcp", "dport", "{ 80, 443 }",
		"counter",
		"accept",
		"comment", comment,
	).Run()
	if err != nil {
		exitError := &exec.ExitError{}
		if errors.As(err, &exitError) {
			return fmt.Errorf("nftables error: %s", string(exitError.Stderr))
		}

		return err
	}

	return nil
}

func routingRule(ctx context.Context, table *nftables.Table, chain *nftables.Chain, isV6 bool, snet net.IPNet, dnet net.IPNet, comment string) error {

	ipp := "ip"
	if isV6 {
		ipp = "ip6"
	}

	cmd := exec.CommandContext(
		ctx, "nft", "add", "rule", "netdev", table.Name, chain.Name,
		ipp, "saddr", snet.String(),
		ipp, "daddr", dnet.String(),
		"counter",
		"accept",
		"comment", comment,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		exitError := &exec.ExitError{}
		if errors.As(err, &exitError) {
			return fmt.Errorf("nftables error: %s", string(exitError.Stderr))
		}

		return err
	}

	return nil
}
