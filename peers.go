package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func peerCmd() *cobra.Command {
	rules := []string{}
	dryRun := false
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
			err := newPeer(args[0], rules, dryRun, privateKey)
			if err != nil {
				slog.Error(err.Error())
				os.Exit(1)
			}
		},
		Aliases: []string{"new"},
	}
	add.Flags().StringSliceVarP(&rules, "rules", "r", rules, "rules to apply to this peer")
	add.Flags().BoolVarP(&dryRun, "dry-run", "d", dryRun, "don't actually create peer")
	add.Flags().StringVarP(&serverPrivateKey, "server-private-key", "s", serverPrivateKey, "server private key")

	cmd.AddCommand(add)
	return cmd
}

func newPeer(name string, rules []string, dryRun bool, serverKey wgtypes.Key) error {
	index := uint64(0)
	buf := make([]byte, 2)
	rand.Read(buf)
	// 32 bits time + 16 bits is rand
	index = index<<32 | uint64(time.Now().Unix())
	index = index<<16 | uint64(binary.BigEndian.Uint16(buf))

	keybuf := make([]byte, wgtypes.KeyLen)
	rand.Read(keybuf)

	keyset, err := wgtypes.NewKey(keybuf)
	if err != nil {
		return fmt.Errorf("wgtypes.NewKey: %w", err)
	}

	pskbuf := make([]byte, wgtypes.KeyLen)
	rand.Read(pskbuf)
	pskset, err := wgtypes.NewKey(pskbuf)
	if err != nil {
		return fmt.Errorf("wgtypes.NewKey: %w", err)
	}

	peerValue := &WireguardAccessPeer{
		Metadata: v1.ObjectMeta{
			Name: name,
		},
		TypeMeta: v1.TypeMeta{
			Kind:       "WireguardAccessPeer",
			APIVersion: "wga.kraudcloud.com/v1beta",
		},
		Spec: WireguardAccessPeerSpec{
			Index:     int(index),
			Rules:     rules,
			PublicKey: keyset.PublicKey().String(),
			PSK:       pskset.String(),
		},
	}

	if !dryRun {
		config, err := clientConfig()
		if err != nil {
			return fmt.Errorf("clientConfig: %w", err)
		}

		clientset, err := dynamic.NewForConfig(config)
		if err != nil {
			slog.Error("Error creating Kubernetes client", "error", err)
			os.Exit(1)
		}

		// encode object to JSON
		data, _ := json.Marshal(peerValue)
		// create object
		o := unstructured.Unstructured{}
		json.Unmarshal(data, &o)

		r, err := clientset.Resource(schema.GroupVersionResource{
			Group:    "wga.kraudcloud.com",
			Version:  "v1beta",
			Resource: "wireguardaccesspeers",
		}).Create(context.Background(), &o, v1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("cannot create peer: %w", err)
		}

		fmt.Printf("\nCreated WireguardAccessPeer %s\n", r.GetName())
	} else {
		json.NewEncoder(os.Stderr).Encode(peerValue)
		return nil
	}

	oubuf := &strings.Builder{}
	err = Format(oubuf, WireguardConfig{
		ConfigName: name,
		PrivateKey: keyset.String(),
		Address:    "10.0.0.1/24", // FIXME: I know we need to build this one from prefix + index but idk where prefix is.
		Peers: []WireguardConfigPeer{
			{
				Endpoint:  "10.0.0.1:51820", // FIXME: How do I read this one?
				PublicKey: serverKey.PublicKey().String(),
				AllowedIPs: []string{
					"10.0.0.0/24", // FIXME: how do I get those?
				},
			},
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
