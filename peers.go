package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"strings"

	"github.com/kraudcloud/wga/wgav1beta"
	"github.com/spf13/cobra"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

func peerCmd() *cobra.Command {
	rules := []string{}
	serverPrivateKey := ""
	privateKey := wgtypes.Key{}

	cmd := &cobra.Command{
		Use:     "peer",
		Short:   "handle WireguardAccessPeers",
		Aliases: []string{"p", "peers"},
	}

	add := &cobra.Command{
		Use:   "add [name]",
		Short: "add a WireguardAccessPeer",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if len(serverPrivateKey) == 0 {
				// try to load from k8s
				pkstr, err := os.ReadFile("/etc/wga/endpoint/privateKey")
				if err != nil {
					return fmt.Errorf("cannot read private key from /etc/wga/endpoint/privateKey: %w", err)
				}
				serverPrivateKey = string(pkstr)
			}

			sk, err := wgtypes.ParseKey(strings.TrimSpace(string(serverPrivateKey)))
			if err != nil {
				return fmt.Errorf("cannot parse private key: %w", err)
			}

			privateKey = sk
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			err := newPeer(args[0], rules, privateKey)
			if err != nil {
				slog.Error(err.Error())
				os.Exit(1)
			}
		},
		Aliases: []string{"new"},
	}
	add.Flags().StringSliceVarP(&rules, "rules", "r", rules, "rules to apply to this peer")
	add.Flags().StringVarP(&serverPrivateKey, "server-private-key", "s", serverPrivateKey, "server private key")

	cmd.AddCommand(add)
	return cmd
}

func newPeer(name string, rules []string, serverKey wgtypes.Key) error {
	keyset, err := wgtypes.GenerateKey()
	if err != nil {
		return fmt.Errorf("wgtypes.NewKey: %w", err)
	}

	pskset, err := wgtypes.GenerateKey()
	if err != nil {
		return fmt.Errorf("wgtypes.NewKey: %w", err)
	}

	peerValue := wgav1beta.WireguardAccessPeer{
		Metadata: v1.ObjectMeta{
			Name: name,
		},
		TypeMeta: v1.TypeMeta{
			Kind:       "WireguardAccessPeer",
			APIVersion: "wga.kraudcloud.com/v1beta",
		},
		Spec: wgav1beta.WireguardAccessPeerSpec{
			AccessRules:  rules,
			PublicKey:    keyset.PublicKey().String(),
			PreSharedKey: pskset.String(),
		},
	}

	client, err := clientConfig()
	if err != nil {
		return fmt.Errorf("cannot get client config: %w", err)
	}

	c, err := wgav1beta.NewForConfig(client)
	if err != nil {
		return fmt.Errorf("cannot create CRD client: %w", err)
	}

	_, err = c.CreateWireguardAccessPeer(context.Background(), peerValue)
	if err != nil {
		return fmt.Errorf("cannot create peer: %w", err)
	}

	w, err := c.WatchWireguardAccessPeers(context.Background(), v1.ListOptions{
		Watch:         true,
		FieldSelector: fmt.Sprintf("metadata.name=%s", name),
	})
	if err != nil {
		return fmt.Errorf("cannot watch peer: %w", err)
	}

	var populatedPeer wgav1beta.WireguardAccessPeer
	for event := range w.ResultChan() {
		if event.Type != watch.Modified {
			continue
		}
		if event.Object == nil {
			continue
		}
		peer, ok := event.Object.(*wgav1beta.WireguardAccessPeer)
		if !ok {
			continue
		}

		populatedPeer = *peer
		w.Stop()
		break
	}

	peers := []wgtypes.Peer{}
	for _, peer := range populatedPeer.Status.Peers {
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

		var psk wgtypes.Key
		if len(peer.PreSharedKey) != 0 {
			psk, err = wgtypes.ParseKey(peer.PreSharedKey)
			if err != nil {
				return fmt.Errorf("cannot parse preshared key: %w", err)
			}
		}

		peers = append(peers, wgtypes.Peer{
			PublicKey:    publicKey,
			AllowedIPs:   ips,
			Endpoint:     net.UDPAddrFromAddrPort(endpoint),
			PresharedKey: psk,
		})
	}

	ip := net.ParseIP(populatedPeer.Status.Address)

	oubuf := &strings.Builder{}
	err = Format(oubuf, ConfigFile{
		Address: &net.IPNet{
			IP:   ip,
			Mask: mask(ip),
		},
		Device: wgtypes.Device{
			Name:       name,
			PrivateKey: keyset,
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

func mask(ip net.IP) net.IPMask {
	return net.IPMask(bytes.Repeat([]byte{0xff}, len(ip)))
}

func newPassword() string {
	buf := make([]byte, 2)
	rand.Read(buf)
	return fmt.Sprintf("%04d", binary.BigEndian.Uint16(buf))
}
