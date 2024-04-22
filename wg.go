package main

import (
	"bytes"
	"fmt"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"log/slog"
	"net"
	"time"
)

const DEVICENAME = "wga"

func wgInit(epConfig *EndpointConfig) error {

	slog.Info("create wg", "interface", DEVICENAME)

	sk, err := wgtypes.ParseKey(PRIVATEKEY)
	if err != nil {
		return fmt.Errorf("cannot parse private key: %w", err)
	}

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
	err = netlink.LinkAdd(wirelink)
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

	wgconfig := wgtypes.Config{}

	wgconfig.PrivateKey = &sk

	var port = 51820
	wgconfig.ListenPort = &port

	err = wg.ConfigureDevice(DEVICENAME, wgconfig)
	if err != nil {
		return fmt.Errorf("wgctrl.ConfigureDevice: %w", err)
	}

	err = netlink.LinkSetUp(link)
	if err != nil {
		return fmt.Errorf("link up: %w", err)
	}

	for _, cidr := range epConfig.Darknet.CIDRs {
		_, snet, err := net.ParseCIDR(cidr)
		if err != nil {
			slog.Error(err.Error(), "darknet.cidr", cidr)
			continue
		}

		err = netlink.RouteAdd(&netlink.Route{
			LinkIndex: link.Attrs().Index,
			Dst:       snet,
		})

		if err != nil {
			slog.Error(err.Error(), "darknet.cidr", cidr)
			continue
		}
	}

	return nil
}

func wgSync(config *Config) error {

	slog.Info("sync wg", "interface", DEVICENAME)

	var err error

	shouldPeers := make(map[string]wgtypes.PeerConfig, 0)

	for _, peer := range config.Peers {

		slog.Info("  sync ", "peer", peer.Name)

		_, snet, err := net.ParseCIDR(peer.Source)
		if err != nil {
			slog.Error(err.Error(), "source", peer.Source, "peer.Name", peer.Name)
			continue
		}

		psk, err := wgtypes.ParseKey(peer.PSK)
		if err != nil {
			slog.Error(err.Error(), "presharedKey", "<recacted>", "peer", peer.Name)
			continue
		}

		pub, err := wgtypes.ParseKey(peer.PublicKey)
		if err != nil {
			slog.Error(err.Error(), "publicKey", peer.PublicKey, "peer", peer.Name)
			continue
		}

		keepalive := 58 * time.Second

		pc := wgtypes.PeerConfig{
			PersistentKeepaliveInterval: &keepalive,
			ReplaceAllowedIPs:           true,
			PresharedKey:                &psk,
			PublicKey:                   pub,
			AllowedIPs:                  []net.IPNet{*snet},
		}

		shouldPeers[pub.String()] = pc

	}

	wg, err := wgctrl.New()
	if err != nil {
		return fmt.Errorf("wgctrl.New: %w", err)
	}
	defer wg.Close()

	existing_device, err := wg.Device(DEVICENAME)
	if err != nil {
		return fmt.Errorf("wg.Device(%s): %w", DEVICENAME, err)
	}

	havePeers := make(map[string]*wgtypes.Peer, 0)
	for _, v := range existing_device.Peers {
		var vclone = v
		havePeers[v.PublicKey.String()] = &vclone
	}

	nuconfig := wgtypes.Config{
		ReplacePeers: false,
	}

	for k, old := range havePeers {

		if nu, ok := shouldPeers[k]; ok {
			var changed = false

			if nu.PresharedKey != nil && *nu.PresharedKey != old.PresharedKey {
				slog.Info("# psk changed ", "peer", k)
				changed = true
			}
			if len(nu.AllowedIPs) != len(old.AllowedIPs) {
				slog.Info("# allowedips changed", "peer", k, "from", len(old.AllowedIPs), "to", len(nu.AllowedIPs))
				changed = true
			} else {
				for i, _ := range nu.AllowedIPs {
					if !nu.AllowedIPs[i].IP.Equal(old.AllowedIPs[i].IP) {
						slog.Info("# allowedips changed ", "peer", k, "ip", i, "from", old.AllowedIPs[i].IP, "to", nu.AllowedIPs[i].IP)
						changed = true
					}
					if !bytes.Equal(nu.AllowedIPs[i].Mask, old.AllowedIPs[i].Mask) {
						slog.Info("# allowedips changed ", "peer", k, "ip", i, "from", old.AllowedIPs[i].Mask, "to", nu.AllowedIPs[i].Mask)
						changed = true
					}
				}
			}

			if !changed {
				//log.Println("# unchanged ")
				delete(shouldPeers, k)
				continue
			}

			nu.UpdateOnly = true
			nu.ReplaceAllowedIPs = true
			nuconfig.Peers = append(nuconfig.Peers, nu)

			slog.Info("# update ")
			delete(shouldPeers, k)

		} else {

			//remove peers that are no longer in the new config
			nuconfig.Peers = append(nuconfig.Peers, wgtypes.PeerConfig{
				Remove:    true,
				PublicKey: old.PublicKey,
			})
			slog.Info("# remove ", "peer", k)
		}
	}

	// add the rest that is not yet there
	for k, v := range shouldPeers {
		slog.Info("# add", "pk", k)
		nuconfig.Peers = append(nuconfig.Peers, v)
	}

	err = wg.ConfigureDevice(DEVICENAME, nuconfig)
	if err != nil {
		return fmt.Errorf("wg.ConfigureDevice: %w", err)
	}

	return nil
}
