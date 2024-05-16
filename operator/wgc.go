package operator

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/kraudcloud/wga/apis/wga.kraudcloud.com/v1beta"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var clientPredicate = &predicate.TypedFuncs[client.Object]{
	UpdateFunc: func(tue event.UpdateEvent) bool {
		return true
	},
	DeleteFunc: func(tde event.DeleteEvent) bool {
		return true
	},
	GenericFunc: func(ge event.GenericEvent) bool {
		return false
	},
	CreateFunc: func(ce event.CreateEvent) bool {
		return true
	},
}

func RunWGC(
	ctx context.Context,
	config *rest.Config,
) {
	mgr, err := manager.New(config, manager.Options{})
	if err != nil {
		slog.Error("unable to create new manager", "err", err)
		os.Exit(1)
	}

	log.SetLogger(logr.FromSlogHandler(slog.With("component", "wgc-controller").Handler()))

	registerClusterClientReconciler(mgr)

	if err = mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		slog.Error("unable to set up health check", "err", err)
		os.Exit(1)
	}

	if err = mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		slog.Error("unable to set up ready check", "err", err)
		os.Exit(1)
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		slog.Error("unable to start manager", "err", err)
		os.Exit(1)
	}
}

func registerClusterClientReconciler(mgr ctrl.Manager) {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta.WireguardClusterClient{}, builder.WithPredicates(clientPredicate)).
		WithEventFilter(clientPredicate).
		Owns(&v1beta.WireguardClusterClient{}).
		Watches(&v1beta.WireguardClusterClient{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
			return []reconcile.Request{{NamespacedName: client.ObjectKeyFromObject(o)}}
		}), builder.WithPredicates(clientPredicate)).
		Complete(reconcile.AsReconciler(mgr.GetClient(), &ClusterClientReconciler{
			client: mgr.GetClient(),
			log:    slog.With("component", "wgc-reconciler"),
		}))
	if err != nil {
		slog.Error("unable to create controller", "err", err)
		os.Exit(1)
	}
}

type ClusterClientReconciler struct {
	client client.Client
	log    *slog.Logger
}

func (r *ClusterClientReconciler) Reconcile(ctx context.Context, c *v1beta.WireguardClusterClient) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	wgcs := new(v1beta.WireguardClusterClientList)
	err := r.client.List(ctx, wgcs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error listing peers: %w", err)
	}

	for i, wg := range wgcs.Items {
		r.log.Info("WireguardClusterClient", "name", wg.Name)

		updateWgc := false
		skName := wg.Spec.PrivateKeySecretRef.Name
		if skName == "" {
			skName = "wgc-" + wg.Name
			wg.Spec.PrivateKeySecretRef.Name = skName
			updateWgc = true
		}
		skNamespace := GetK8sNamespace()
		skKey := wg.Spec.PrivateKeySecretRef.Key
		if skKey == "" {
			skKey = "privateKey"
			wg.Spec.PrivateKeySecretRef.Key = skKey
			updateWgc = true
		}

		var privk wgtypes.Key
		sk := new(corev1.Secret)

		err := r.client.Get(ctx, client.ObjectKey{
			Namespace: skNamespace,
			Name:      skName,
		}, sk)
		if err == nil {

			privk, err = wgtypes.ParseKey(string(sk.Data[skKey]))
			if err != nil {
				return reconcile.Result{}, fmt.Errorf("error parsing key: %w", err)
			}

		} else {

			if wg.Spec.PrivateKeySecretRef.Value != "" {
				privk, err = wgtypes.ParseKey(wg.Spec.PrivateKeySecretRef.Value)
				if err != nil {
					return ctrl.Result{}, fmt.Errorf("error parsing key: %w", err)
				}
				wg.Spec.PrivateKeySecretRef.Value = ""
				updateWgc = true
			} else {
				privk, err = wgtypes.GenerateKey()
				if err != nil {
					return ctrl.Result{}, fmt.Errorf("error generating key: %w", err)
				}
			}

			sk.Name = skName
			sk.Namespace = skNamespace
			sk.Data = make(map[string][]byte)
			sk.Data[skKey] = []byte(privk.String())
			err = r.client.Create(ctx, sk)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("error creating secret: %w", err)
			}
		}

		if wg.Status == nil {
			wg.Status = &v1beta.WireguardClusterClientStatus{}
		}
		if wg.Status.PublicKey == "" {
			wg.Status.PublicKey = privk.PublicKey().String()
			updateWgc = true
		}

		if updateWgc {
			r.log.Info("Updating WireguardClusterClient", "name", wg.Name)
			err = r.client.Update(ctx, &wg)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("error updating wgc: %w", err)
			}
		}

		wg.Spec.PrivateKeySecretRef.Value = privk.String()
		wgcs.Items[i] = wg

	}

	err = wgcSync(wgcs.Items)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error syncing wgc: %w", err)
	}

	return ctrl.Result{}, nil
}

func GetK8sNamespace() string {
	if ns, ok := os.LookupEnv("POD_NAMESPACE"); ok {
		return ns
	}

	if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns
		}
	}

	return "default"
}

func wgcSync(wgc []v1beta.WireguardClusterClient) error {
	// list existing interfaces
	lnks, err := netlink.LinkList()
	if err != nil {
		return err
	}

	existing := make(map[string]netlink.Link)
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
				Mask: FullMask(ip),
			}
		} else {
			ip, net, err := net.ParseCIDR(wgc.Spec.Address)
			if err != nil {
				return fmt.Errorf("error parsing address: %w", err)
			}
			addr = net
			addr.IP = ip
		}

		slog.Info("syncing WireguardClusterClient", "name", wgc.Name, "address", addr)

		err = netlink.AddrReplace(link, &netlink.Addr{
			IPNet: addr,
		})
		if err != nil {
			return fmt.Errorf("cannot add address: %w", err)
		}

		// if addr not in wgc.Spec.Addresses, delete it
		addrs, _ := netlink.AddrList(link, netlink.FAMILY_ALL)
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

		// get existing routes
		hasRoutes, err := netlink.RouteList(link, netlink.FAMILY_ALL)
		if err != nil {
			return fmt.Errorf("cannot get routes: %w", err)
		}

		for _, hasRoute := range hasRoutes {
			delete := true
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

func FullMask(ip net.IP) net.IPMask {
	return net.IPMask(bytes.Repeat([]byte{0xff}, len(ip)))
}
