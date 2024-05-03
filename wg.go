package main

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const DEVICENAME = "wga"

// WGConfig is readonly after `wgInit` is called.
var (
	WGConfig   = wgtypes.Config{}
	WGInitOnce = sync.Once{}
)

func wgInit() error {
	slog.Info("create wg", "interface", DEVICENAME)

	// delete old link
	link, _ := netlink.LinkByName(DEVICENAME)
	if link != nil {
		slog.Info("delete old wg", "interface", DEVICENAME)
		netlink.LinkDel(link)
	}

	wirelink := &netlink.GenericLink{
		LinkAttrs: netlink.LinkAttrs{
			Name: DEVICENAME,
		},
		LinkType: "wireguard",
	}
	err := netlink.LinkAdd(wirelink)
	if err != nil {
		return fmt.Errorf("cannot create wg interface: %w", err)
	}
	link, _ = netlink.LinkByName(DEVICENAME)

	// bring up wg
	wg, err := wgctrl.New()
	if err != nil {
		return fmt.Errorf("wgctrl.New: %w", err)
	}
	defer wg.Close()

	sk, err := readKey()
	if err != nil {
		return fmt.Errorf("cannot read wg private key: %w", err)
	}

	WGConfig.PrivateKey = &sk
	port := 51820
	WGConfig.ListenPort = &port

	err = wg.ConfigureDevice(DEVICENAME, WGConfig)
	if err != nil {
		return fmt.Errorf("wgctrl.ConfigureDevice: %w", err)
	}

	err = netlink.LinkSetUp(link)
	if err != nil {
		return fmt.Errorf("link up: %w", err)
	}

	clientCIDRstr := os.Getenv("WGA_CLIENT_CIDR")
	_, clientCIDR, err := net.ParseCIDR(clientCIDRstr)
	if err != nil {
		slog.Error("cannot parse client cidr", "WGA_CLIENT_CIDR", clientCIDRstr, "err", err.Error())
		panic(err)
	}

	err = netlink.RouteAdd(&netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       clientCIDR,
	})
	if err != nil {
		return fmt.Errorf("cannot add route: %w", err)
	}

	return nil
}

func wgSync(log *slog.Logger, config *Config) error {
	shouldPeers := make(map[string]wgtypes.PeerConfig, 0)
	// find out what peers have no `status` and generate their status
	// this should probably be in the watcher rather than here.
	log.Debug("syncing peers")
	for _, peer := range config.Peers {
		log.Info("syncing peer", "peer", peer.Name, "address", peer.Status.Address)
		snet := net.IPNet{
			IP:   net.ParseIP(peer.Status.Address),
			Mask: net.CIDRMask(128, 128),
		}

		var psk wgtypes.Key
		var err error
		if peer.Spec.PreSharedKey != "" {
			psk, err = wgtypes.ParseKey(peer.Spec.PreSharedKey)
			if err != nil {
				log.Error(err.Error(), "presharedKey", "<redacted>", "peer", peer.Name)
				continue
			}
		}

		pub, err := wgtypes.ParseKey(peer.Spec.PublicKey)
		if err != nil {
			log.Error(err.Error(), "publicKey", peer.Spec.PublicKey, "peer", peer.Name)
			continue
		}

		keepalive := 60 * time.Second
		pc := wgtypes.PeerConfig{
			PersistentKeepaliveInterval: &keepalive,
			ReplaceAllowedIPs:           true,
			PresharedKey:                &psk,
			PublicKey:                   pub,
			AllowedIPs:                  []net.IPNet{snet},
		}

		shouldPeers[pub.String()] = pc
	}

	log.Debug("creating wgctrl client")
	wg, err := wgctrl.New()
	if err != nil {
		return fmt.Errorf("wgctrl.New: %w", err)
	}
	defer wg.Close()

	log.Debug("getting existing device")
	existing_device, err := wg.Device(DEVICENAME)
	if err != nil {
		return fmt.Errorf("wg.Device(%s): %w", DEVICENAME, err)
	}

	havePeers := make(map[string]*wgtypes.Peer, 0)
	for _, v := range existing_device.Peers {
		vclone := v
		havePeers[v.PublicKey.String()] = &vclone
	}

	nuconfig := wgtypes.Config{
		ReplacePeers: false,
	}

	log.Debug("comparing existing and desired peers")
	for k, old := range havePeers {
		if nu, ok := shouldPeers[k]; ok {
			changed := false

			if nu.PresharedKey != nil && *nu.PresharedKey != old.PresharedKey {
				log.Info("# psk changed ", "peer", k)
				changed = true
			}
			if len(nu.AllowedIPs) != len(old.AllowedIPs) {
				log.Info("# allowedips changed", "peer", k, "from", len(old.AllowedIPs), "to", len(nu.AllowedIPs))
				changed = true
			} else {
				for i := range nu.AllowedIPs {
					if !nu.AllowedIPs[i].IP.Equal(old.AllowedIPs[i].IP) {
						log.Info("# allowedips changed ", "peer", k, "ip", i, "from", old.AllowedIPs[i].IP, "to", nu.AllowedIPs[i].IP)
						changed = true
					}
					if !bytes.Equal(nu.AllowedIPs[i].Mask, old.AllowedIPs[i].Mask) {
						log.Info("# allowedips changed ", "peer", k, "ip", i, "from", old.AllowedIPs[i].Mask, "to", nu.AllowedIPs[i].Mask)
						changed = true
					}
				}
			}

			if !changed {
				// log.Println("# unchanged ")
				delete(shouldPeers, k)
				continue
			}

			nu.UpdateOnly = true
			nu.ReplaceAllowedIPs = true
			nuconfig.Peers = append(nuconfig.Peers, nu)

			log.Info("# update ")
			delete(shouldPeers, k)

		} else {

			// remove peers that are no longer in the new config
			nuconfig.Peers = append(nuconfig.Peers, wgtypes.PeerConfig{
				Remove:    true,
				PublicKey: old.PublicKey,
			})
			log.Info("# remove ", "peer", k)
		}
	}

	// add the rest that is not yet there
	for k, v := range shouldPeers {
		log.Info("# add", "pk", k)
		nuconfig.Peers = append(nuconfig.Peers, v)
	}

	log.Debug("configuring device")
	err = wg.ConfigureDevice(DEVICENAME, nuconfig)
	if err != nil {
		return fmt.Errorf("wg.ConfigureDevice: %w", err)
	}

	return nil
}

func generateIndex(cidr *net.IPNet) int {
	// a /16 means we have either 112 or 16 bits of network. figure out which
	// and use that to generate the index.
	bits, _ := cidr.Mask.Size()
	if len(cidr.IP) == net.IPv4len {
		bits = 32 - bits
	} else {
		bits = 128 - bits
	}

	// fill time as much as possible (up to 64 bits)
	// add 16 bits of randomness
	bits = min(bits, 64)

	// make sure we take the most significant bits of the time.
	index := 0
	if bits > 16 {
		index = int(time.Now().UnixNano() >> (64 - bits))
	}

	randbuf := make([]byte, 2)
	_, err := rand.Read(randbuf)
	if err != nil {
		return index
	}

	if bits > 8 {
		index = index | (int(randbuf[0]) << 8)
	}

	index = index | (int(randbuf[1]))
	return index
}

func readKey() (wgtypes.Key, error) {
	pkstr, err := os.ReadFile("/etc/wga/endpoint/privateKey")
	if err != nil {
		return wgtypes.Key{}, fmt.Errorf("cannot read private key from /etc/wga/endpoint/privateKey: %w", err)
	}

	return wgtypes.ParseKey(strings.TrimSpace(string(pkstr)))
}
