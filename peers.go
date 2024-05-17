package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/kraudcloud/wga/operator"
	"github.com/kraudcloud/wga/pkgs/apis/v1beta"
	"github.com/spf13/cobra"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func peerCmd() *cobra.Command {
	rules := []string{}
	dns := []net.IP{
		// v4
		net.ParseIP("1.1.1.1"),
		// v6
		net.ParseIP("2606:4700:4700::1111"),
	}

	cmd := &cobra.Command{
		Use:     "peer",
		Short:   "handle WireguardAccessPeers",
		Aliases: []string{"p", "peers"},
	}

	var formatAsWGC string
	add := &cobra.Command{
		Use:   "add [name]",
		Short: "add a WireguardAccessPeer",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			err := newPeer(ctx, args[0], rules, dns, formatAsWGC, clientConfig())
			if err != nil {
				slog.Error(err.Error())
				os.Exit(1)
			}
		},
		Aliases: []string{"new"},
	}
	add.Flags().StringSliceVarP(&rules, "rules", "r", rules, "rules to apply to this peer")
	add.Flags().StringVar(&formatAsWGC, "wgc", "", "format as a WireguardClusterClient.yaml")

	cmd.AddCommand(add)
	return cmd
}

func newPeer(ctx context.Context, name string, rules []string, dns []net.IP, formatAsWGC string, config *rest.Config) error {
	keyset, err := wgtypes.GenerateKey()
	if err != nil {
		return fmt.Errorf("wgtypes.NewKey: %w", err)
	}

	pskset, err := wgtypes.GenerateKey()
	if err != nil {
		return fmt.Errorf("wgtypes.NewKey: %w", err)
	}

	peerValue := v1beta.WireguardAccessPeer{
		ObjectMeta: v1.ObjectMeta{
			Name: name,
		},
		TypeMeta: v1.TypeMeta{
			Kind:       "WireguardAccessPeer",
			APIVersion: "wga.kraudcloud.com/v1beta",
		},
		Spec: v1beta.WireguardAccessPeerSpec{
			AccessRules:  rules,
			PublicKey:    keyset.PublicKey().String(),
			PreSharedKey: pskset.String(),
		},
	}

	c, err := client.NewWithWatch(config, client.Options{})
	if err != nil {
		return fmt.Errorf("cannot create client: %w", err)
	}

	err = c.Create(ctx, &peerValue)
	if err != nil {
		return fmt.Errorf("cannot create peer: %w", err)
	}

	w, err := c.Watch(ctx, &v1beta.WireguardAccessPeerList{}, client.MatchingFieldsSelector{
		Selector: fields.AndSelectors(fields.OneTermEqualSelector("metadata.name", name)),
	})
	if err != nil {
		return fmt.Errorf("cannot watch peer: %w", err)
	}

	var populatedPeer *v1beta.WireguardAccessPeer
	for event := range w.ResultChan() {
		if event.Object == nil {
			continue
		}
		peer, ok := event.Object.(*v1beta.WireguardAccessPeer)
		if !ok {
			continue
		}

		if peer.Status != nil {
			populatedPeer = peer
			w.Stop()
			break
		}
	}

	if populatedPeer == nil {
		return fmt.Errorf("peer has no status, there was probably an error with the wga server")
	}

	return fmtPeer(*populatedPeer, dns, keyset, pskset, formatAsWGC)
}

func fmtPeer(peer v1beta.WireguardAccessPeer, dns []net.IP, pk, psk wgtypes.Key, formatAsWGC string) error {
	peers := []wgtypes.Peer{}
	for _, peer := range peer.Status.Peers {
		publicKey, err := wgtypes.ParseKey(peer.PublicKey)
		if err != nil {
			return fmt.Errorf("cannot parse public key: %w", err)
		}

		ips := []net.IPNet{}
		for _, ip := range peer.AllowedIPs {
			_, cidr, err := net.ParseCIDR(ip)
			if err != nil {
				return fmt.Errorf("cannot parse allowed ip: %w", err)
			}

			ips = append(ips, *cidr)
		}

		endpoint, err := netip.ParseAddrPort(peer.Endpoint)
		if err != nil {
			return fmt.Errorf("cannot parse endpoint: %w", err)
		}

		if len(peer.PreSharedKey) != 0 {
			psk, err = wgtypes.ParseKey(peer.PreSharedKey)
			if err != nil {
				return fmt.Errorf("cannot parse preshared key: %w", err)
			}
		}

		peers = append(peers, wgtypes.Peer{
			PublicKey:                   publicKey,
			AllowedIPs:                  ips,
			Endpoint:                    net.UDPAddrFromAddrPort(endpoint),
			PresharedKey:                psk,
			PersistentKeepaliveInterval: time.Second * 60,
		})
	}

	ip := net.ParseIP(peer.Status.Address)

	oubuf := &strings.Builder{}
	err := Format(oubuf, ConfigFile{
		Name: formatAsWGC,
		Address: &net.IPNet{
			IP:   ip,
			Mask: operator.FullMask(ip),
		},
		DNS: dns,
		Device: wgtypes.Device{
			Name:       peer.Name,
			PrivateKey: pk,
			ListenPort: 51820,
			Peers:      peers,
		},
	})
	if err != nil {
		return fmt.Errorf("Format: %w", err)
	}

	fmt.Printf("%s\n", oubuf.String())
	return nil
}

func newPassword() string {
	buf := make([]byte, 2)
	rand.Read(buf)
	return fmt.Sprintf("%04d", binary.BigEndian.Uint16(buf))
}
