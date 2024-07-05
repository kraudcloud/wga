package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
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
	"golang.org/x/sync/errgroup"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func peerCmd() *cobra.Command {
	rules := []string{}

	cmd := &cobra.Command{
		Use:     "peer",
		Short:   "handle WireguardAccessPeers",
		Aliases: []string{"p", "peers"},
	}

	add := &cobra.Command{
		Use:   "add [name]",
		Short: "add a WireguardAccessPeer",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			pk, err := wgtypes.GenerateKey()
			if err != nil {
				exit("unable to generate key", "err", err)
			}

			psk, err := wgtypes.GenerateKey()
			if err != nil {
				exit("unable to generate psk", "err", err)
			}

			peer, err := NewWGAPeer(ctx, args[0], rules, pk, psk, clientConfig())
			if err != nil {
				exit("unable to create peer", "err", err)
			}

			FormatPeerIni(*peer, peer.Status.DNS, pk, psk)
		},
		Aliases: []string{"new"},
	}
	add.Flags().StringSliceVarP(&rules, "rules", "r", rules, "rules to apply to this peer")
	cmd.AddCommand(add)

	wgcNodes := []string{}
	wgc := &cobra.Command{
		Use:   "wgc",
		Short: "generate a configuration for a WireguardAccessClient",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			if len(wgcNodes) == 0 {
				exit("no wgc nodes specified")
			}

			client := clientConfig()

			nodes := make([]v1beta.WireguardClusterClientNode, len(wgcNodes))
			peers := make([]v1beta.WireguardAccessPeer, len(wgcNodes))
			group := errgroup.Group{}
			for i := range wgcNodes {
				i := i
				group.Go(func() error {
					pk, err := wgtypes.GenerateKey()
					if err != nil {
						exit("unable to generate key", "err", err)
					}

					psk, err := wgtypes.GenerateKey()
					if err != nil {
						exit("unable to generate psk", "err", err)
					}

					peer, err := NewWGAPeer(ctx, fmt.Sprintf("wgc-%s-%s", args[0], wgcNodes[i]), rules, pk, psk, client)
					if err != nil {
						return err
					}

					peers[i] = *peer
					nodes[i] = v1beta.WireguardClusterClientNode{
						NodeName:     wgcNodes[i],
						PreSharedKey: psk.String(),
						PrivateKey: v1beta.WireguardClusterClientNodePrivateKey{
							Value: ptr(pk.String()),
						},
						Address: peer.Status.Address,
					}
					return nil
				})
			}

			err := group.Wait()
			if err != nil {
				exit("unable to create peers", "err", err)
			}

			p := peers[0].Status.Peers[0]

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")

			// can't marshal yaml because k8s doesn't have proper yaml field tags
			enc.Encode(v1beta.WireguardClusterClient{
				TypeMeta: v1.TypeMeta{
					Kind:       "WireguardClusterClient",
					APIVersion: "wga.kraudcloud.com/v1beta",
				},
				ObjectMeta: v1.ObjectMeta{
					Name: args[0],
				},
				Spec: v1beta.WireguardClusterClientSpec{
					Nodes:  nodes,
					Routes: p.AllowedIPs,
					Server: v1beta.WireguardClusterClientSpecServer{
						Endpoint:  p.Endpoint,
						PublicKey: p.PublicKey,
					},
					PersistentKeepalive: 60,
				},
			})
		},
	}

	wgc.Flags().StringSliceVarP(&wgcNodes, "nodes", "n", wgcNodes, "list of WireguardClusterClient node names to connect to")
	cmd.AddCommand(wgc)

	return cmd
}

func NewWGAPeer(ctx context.Context, name string, rules []string, keyset, pskset wgtypes.Key, config *rest.Config) (*v1beta.WireguardAccessPeer, error) {
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
		return nil, fmt.Errorf("cannot create client: %w", err)
	}

	err = c.Create(ctx, &peerValue)
	if err != nil {
		return nil, fmt.Errorf("cannot create peer: %w", err)
	}

	w, err := c.Watch(ctx, &v1beta.WireguardAccessPeerList{}, client.MatchingFieldsSelector{
		Selector: fields.AndSelectors(fields.OneTermEqualSelector("metadata.name", name)),
	})
	if err != nil {
		return nil, fmt.Errorf("cannot watch peer: %w", err)
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
		return nil, fmt.Errorf("peer has no status, there was probably an error with the wga server")
	}

	return populatedPeer, nil
}

func FormatPeerIni(peer v1beta.WireguardAccessPeer, dns []string, pk, psk wgtypes.Key) error {
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
		Name: peer.Name,
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

func exit(message string, args ...any) {
	slog.Error(message, args...)
	os.Exit(1)
}

func ptr[T any](v T) *T {
	return &v
}
