package main

import (
	"os"
	"os/exec"

	"github.com/google/nftables"
	"log/slog"
	"net"
	"strings"
)

func sysctl(config *Config) {
	exec.Command("sysctl", "-w", "net.ipv6.conf.all.forwarding=1").Run()
}

func nftSync(config *Config) {

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

		_, snet, err := net.ParseCIDR(peer.Source)
		if err != nil {
			slog.Error(err.Error(), "source", peer.Source, "peer", peer.Name)
			continue
		}

		for _, addr := range peer.Destinations {

			_, dnet, err := net.ParseCIDR(addr)
			if err != nil {
				slog.Error(err.Error(), "destination", addr, "peer", peer.Name)
				continue
			}

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
				slog.Error(err.Error(), "destination", addr, "peer", peer.Name)
				continue
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
	}), nil
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

	cmd := exec.Command("nft", "add", "chain", "netdev", "wga", device, "{ type filter hook ingress device "+device+" priority 0 ; policy drop; }")
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
