package main

import (
	"log/slog"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/google/nftables"
)

func sysctl(config *Config) {
	exec.Command("sysctl", "-w", "net.ipv6.conf.all.forwarding=1").Run()
}

var NFTInitOnce = sync.Once{}

func nftInit() {
	exec.Command("nft", "add", "table", "inet", "filter").Run()
	exec.Command("nft", "add", "chain", "inet", "filter", "postrouting", "{ type nat hook postrouting priority 100 ; }").Run()
	exec.Command("nft", "add", "rule", "inet", "filter", "postrouting", "oifname", "eth0", "masquerade").Run()
}

func nftSync(config *Config) {
	NFTInitOnce.Do(nftInit)

	var ruleNameToDestinations = make(map[string][]net.IPNet)
	for _, rr := range config.Rules {

		nets := []net.IPNet{}
		for _, d := range rr.Spec.Destinations {
			_, ipnet, err := net.ParseCIDR(d)
			if err != nil {
				slog.Error(err.Error())
				continue
			}
			nets = append(nets, *ipnet)
		}
		ruleNameToDestinations[rr.Metadata.Name] = nets
	}

	nft, err := nftables.New()
	if err != nil {
		panic(err)
	}
	defer nft.Flush()

	table, err := checkOrCreateTable(nft)
	if err != nil {
		panic(err)
	}

	chain, err := checkOrCreateWGAIngressChain(nft, table, DEVICENAME)
	if err != nil {
		panic(err)
	}

	rules, err := nft.GetRules(table, chain)
	if err != nil {
		panic(err)
	}

	ruleMap := make(map[string]*nftables.Rule)
	for _, r := range rules {
		ruleMap[string(r.UserData)] = r
	}

	for _, peer := range config.Peers {
		if peer.Status == nil {
			slog.Warn("peer has no status", "peer", peer.Metadata.Name)
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

				cmd := exec.Command("nft", "add", "rule", "netdev", "wga", DEVICENAME,
					"ip6", "saddr", snet.String(),
					"ip6", "daddr", dnet.String(),
					"counter", "accept", "comment", comment)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				err = cmd.Run()
				if err != nil {
					slog.Error(err.Error(), "destination", dnet.String(), "peer", peer.Metadata.Name)
					continue
				}
			}

		}
	}

	for _, stale := range ruleMap {
		nft.DelRule(stale)
	}

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

func checkOrCreateWGAIngressChain(nft *nftables.Conn, table *nftables.Table, device string) (
	*nftables.Chain, error) {

	chains, err := nft.ListChains()
	if err != nil {
		return nil, err
	}

	for _, c := range chains {
		if c.Name == device && c.Table.Name == table.Name {
			return c, nil
		}
	}

	cmd := exec.Command("nft", "add", "chain", "netdev", DEVICENAME, device, "{ type filter hook ingress device "+device+" priority 0 ; policy drop; }")
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
		if c.Name == device && c.Table.Name == table.Name {
			return c, nil
		}
	}

	return nil, nil

}
