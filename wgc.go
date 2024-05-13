package main

import (
	"fmt"
	"github.com/kraudcloud/wga/apis/generated/clientset/versioned"
	"github.com/kraudcloud/wga/apis/wga.kraudcloud.com/v1beta"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"time"
)

func wgcSync(wgc []v1beta.WireguardClusterClient, client *versioned.Clientset) error {

	// list existing interfaces
	lnks, err := netlink.LinkList()
	if err != nil {
		return err
	}

	var existing = make(map[string]netlink.Link)
	for _, lnk := range lnks {
		if lnk.Type() != "wireguard" {
			continue
		}

		if !strings.HasPrefix(lnk.Attrs().Name, "wgc-") {
			continue
		}

		existing[lnk.Attrs().Name] = lnk
	}

	// sync
	for _, wgc := range wgc {
		ifname := "wgc-" + wgc.Name

		if _, ok := existing[ifname]; ok {
			delete(existing, ifname)
		} else {
			wirelink := &netlink.GenericLink{
				LinkAttrs: netlink.LinkAttrs{
					Name: ifname,
				},
				LinkType: "wireguard",
			}
			err = netlink.LinkAdd(wirelink)
			if err != nil {
				return fmt.Errorf("cannot create wg interface: %w", err)
			}
		}

		privk, err := wgtypes.ParseKey(wgc.Spec.PrivateKeySecretRef.Value)
		if err != nil {
			return fmt.Errorf("error parsing key: %w", err)
		}

		link, err := netlink.LinkByName(ifname)
		if err != nil {
			return fmt.Errorf("cannot get wg interface: %w", err)
		}

		wg, err := wgctrl.New()
		if err != nil {
			return fmt.Errorf("wgctrl.New: %w", err)
		}
		defer wg.Close()

		WGConfig.PrivateKey = &privk

		pk, err := wgtypes.ParseKey(wgc.Spec.Server.PublicKey)
		if err != nil {
			return fmt.Errorf("error parsing public key: %w", err)
		}

		routes := []net.IPNet{}
		for _, r := range wgc.Spec.Routes {
			_, snet, err := net.ParseCIDR(r)
			if err != nil {
				return fmt.Errorf("error parsing route: %w", err)
			}
			routes = append(routes, *snet)
		}

		epa, err := netip.ParseAddrPort(wgc.Spec.Server.Endpoint)
		if err != nil {
			return fmt.Errorf("error parsing endpoint: %w", err)
		}

		pc := wgtypes.PeerConfig{
			ReplaceAllowedIPs: true,
			PublicKey:         pk,
			AllowedIPs:        routes,
			Endpoint:          net.UDPAddrFromAddrPort(epa),
		}

		if wgc.Spec.Server.PreSharedKey != "" {
			sk, err := wgtypes.ParseKey(wgc.Spec.Server.PreSharedKey)
			if err != nil {
				return fmt.Errorf("error parsing key: %w", err)
			}
			pc.PresharedKey = &sk

		}

		if wgc.Spec.PersistentKeepalive != 0 {
			ka := time.Second * time.Duration(wgc.Spec.PersistentKeepalive)
			pc.PersistentKeepaliveInterval = &ka
		}

		WGConfig.Peers = append(WGConfig.Peers, pc)

		err = wg.ConfigureDevice(ifname, WGConfig)
		if err != nil {
			return fmt.Errorf("wgctrl.ConfigureDevice: %w", err)
		}

		err = netlink.LinkSetUp(link)
		if err != nil {
			return fmt.Errorf("link up: %w", err)
		}

		var addr *net.IPNet
		ip := net.ParseIP(wgc.Spec.Address)
		if ip != nil {
			addr = &net.IPNet{
				IP:   ip,
				Mask: mask(ip),
			}
		} else {
			ip, net, err := net.ParseCIDR(wgc.Spec.Address)
			if err != nil {
				return fmt.Errorf("error parsing address: %w", err)
			}
			addr = net
			addr.IP = ip
		}

		err = netlink.AddrReplace(link, &netlink.Addr{
			IPNet: addr,
		})

		if err != nil {
			return fmt.Errorf("cannot add address: %w", err)
		}

		// if addr not in wgc.Spec.Addresses, delete it
		addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
		for _, addr2 := range addrs {
			if addr.String() != addr2.IPNet.String() {
				if err := netlink.AddrDel(link, &addr2); err != nil {
					slog.Error("Error deleting old address", "addr", addr, "error", err)
					return err
				}
			}
		}

		for _, dst := range routes {
			err = netlink.RouteReplace(&netlink.Route{
				LinkIndex: link.Attrs().Index,
				Dst:       &dst,
			})
			if err != nil {
				return fmt.Errorf("cannot add route: %w", err)
			}
		}

		//get existing routes
		hasRoutes, err := netlink.RouteList(link, netlink.FAMILY_ALL)
		if err != nil {
			return fmt.Errorf("cannot get routes: %w", err)
		}

		for _, hasRoute := range hasRoutes {
			var delete = true
			for _, route := range routes {
				if hasRoute.Dst.String() == route.String() {
					delete = false
				}
			}

			if delete {
				if err := netlink.RouteDel(&hasRoute); err != nil {
					slog.Error("Error deleting old route", "route", hasRoute, "error", err)
					return err
				}
			}

		}

	}

	// delete leftovers
	for n, lnk := range existing {
		if err := netlink.LinkDel(lnk); err != nil {
			slog.Error("Error deleting old wg interface", "if", n, "error", err)
			return err
		}
	}

	return nil

}
