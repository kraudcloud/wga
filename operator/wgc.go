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
	"github.com/kraudcloud/wga/pkgs/apis/v1beta"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

const (
	SecretKeyName      = "privateKey"
	NodeLabelWGCStatus = "wga.kraudcloud.com/wgc-%s"

	WGCReady  = "Ready"
	WGCFailed = "Failed"
)

func FormatWGCNodeLabel(wgcName string) string {
	return fmt.Sprintf(NodeLabelWGCStatus, wgcName)
}

func (r *ClusterClientReconciler) Reconcile(ctx context.Context, c *v1beta.WireguardClusterClient) (res ctrl.Result, err error) {
	defer func() {
		if err == nil {
			return
		}

		// set node label to failed
		r.client.Patch(ctx, &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   getK8sNode(),
				Labels: map[string]string{FormatWGCNodeLabel(c.Name): WGCFailed},
			},
		}, client.Merge)
	}()

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	wgcs := new(v1beta.WireguardClusterClientList)
	err = r.client.List(ctx, wgcs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error listing peers: %w", err)
	}

	// find out what node this client is on
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		return ctrl.Result{}, fmt.Errorf("NODE_NAME environment variable not set")
	}

	peers := []wgPeer{}
	for _, wg := range wgcs.Items {
		r.log.Info("WireguardClusterClient", "name", wg.Name)

		updateWgc := false
		node := v1beta.WireguardClusterClientNode{}
		for _, n := range wg.Spec.Nodes {
			if n.NodeName == nodeName {
				node = n
				break
			}
		}
		if node.NodeName == "" {
			r.log.Warn("Node not found", "name", wg.Name, "node", nodeName)
			continue
		}

		peerPrivateKey := node.PrivateKey.Value
		if peerPrivateKey == nil {
			ref := node.PrivateKey.SecretRef
			if ref == nil {
				return ctrl.Result{}, fmt.Errorf("privateKey.value or privateKey.secretRef must be set")
			}

			skNamespace := ref.Namespace
			if skNamespace == "" {
				skNamespace = getK8sNamespace()
			}

			skName := ref.Name
			if skName == "" {
				skName = formatSecretName(nodeName, wg.Name)
				updateWgc = true
			}

			sk := new(corev1.Secret)
			err := r.client.Get(ctx, client.ObjectKey{
				Namespace: skNamespace,
				Name:      skName,
			}, sk)
			if err != nil {
				privk, err := wgtypes.GenerateKey()
				if err != nil {
					return ctrl.Result{}, fmt.Errorf("error generating key: %w", err)
				}

				sk = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: skNamespace,
						Name:      skName,
					},
					Data: map[string][]byte{
						SecretKeyName: []byte(privk.String()),
					},
				}
				err = r.client.Create(ctx, sk)
				if err != nil {
					return ctrl.Result{}, fmt.Errorf("error creating secret: %w", err)
				}
			}

			skdata := string(sk.Data[SecretKeyName])
			peerPrivateKey = &skdata
			updateWgc = true
		}

		privk, err := wgtypes.ParseKey(*peerPrivateKey)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error parsing key: %w", err)
		}

		if wg.Status == nil {
			wg.Status = &v1beta.WireguardClusterClientStatus{}
		}

		if wg.Status.Nodes == nil {
			wg.Status.Nodes = []v1beta.WireguardClusterClientStatusNode{}
		}

		peerFound := false
		for i, p := range wg.Status.Nodes {
			if p.NodeName == nodeName {
				peerFound = true
				if p.PublicKey != privk.PublicKey().String() {
					wg.Status.Nodes[i].PublicKey = privk.PublicKey().String()
					updateWgc = true
				}
				break
			}
		}

		if !peerFound {
			wg.Status.Nodes = append(wg.Status.Nodes, v1beta.WireguardClusterClientStatusNode{
				NodeName:  nodeName,
				PublicKey: privk.PublicKey().String(),
			})
			updateWgc = true
		}

		if updateWgc {
			r.log.Info("Updating WireguardClusterClient", "name", wg.Name)
			err = r.client.Update(ctx, &wg)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("error updating wgc: %w", err)
			}
		}

		for i := range wg.Spec.Nodes {
			if wg.Spec.Nodes[i].NodeName == nodeName {
				wg.Spec.Nodes[i] = node
				break
			}
		}

		serverPublicKey, err := wgtypes.ParseKey(wg.Spec.Server.PublicKey)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error parsing server public key: %w", err)
		}

		routes := []net.IPNet{}
		for _, r := range wg.Spec.Routes {
			_, snet, err := net.ParseCIDR(r)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("error parsing route: %w", err)
			}
			routes = append(routes, *snet)
		}

		peer := wgPeer{
			PeerName:            wg.Name,
			PeerAddress:         node.Address,
			PeerPrivateKey:      privk,
			ServerPublicKey:     serverPublicKey,
			Routes:              routes,
			PreSharedKey:        node.PreSharedKey,
			ServerEndpoint:      wg.Spec.Server.Endpoint,
			PersistentKeepalive: wg.Spec.PersistentKeepalive,
		}

		peers = append(peers, peer)
	}

	err = wgcSync(r.log, peers)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error syncing wgc: %w", err)
	}

	// if sync passed, update node labels to reflect we can use wgc
	r.client.Patch(ctx, &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   getK8sNode(),
			Labels: map[string]string{FormatWGCNodeLabel(c.Name): WGCFailed},
		},
	}, client.Merge)

	return ctrl.Result{}, nil
}

func getK8sNode() string {
	if ns, ok := os.LookupEnv("NODE_NAME"); ok {
		return ns
	}

	slog.Error("unable to get node name from env", "key", "NODE_NAME")
	os.Exit(1)
	return ""
}

func formatSecretName(nodeName string, wgcName string) string {
	return fmt.Sprintf("wgc-%s-%s", wgcName, nodeName)
}

func getK8sNamespace() string {
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

type wgPeer struct {
	PeerName            string
	PeerPrivateKey      wgtypes.Key
	PeerAddress         string
	PersistentKeepalive int
	ServerPublicKey     wgtypes.Key
	Routes              []net.IPNet
	PreSharedKey        string
	ServerEndpoint      string
}

func wgcSync(log *slog.Logger, wgc []wgPeer) error {
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
		ifname := "wgc-" + wgc.PeerName
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

		link, err := netlink.LinkByName(ifname)
		if err != nil {
			return fmt.Errorf("cannot get wg interface: %w", err)
		}

		wg, err := wgctrl.New()
		if err != nil {
			return fmt.Errorf("wgctrl.New: %w", err)
		}
		defer wg.Close()

		WGConfig.PrivateKey = &wgc.PeerPrivateKey

		epa, err := netip.ParseAddrPort(wgc.ServerEndpoint)
		if err != nil {
			return fmt.Errorf("error parsing endpoint: %w", err)
		}

		pc := wgtypes.PeerConfig{
			ReplaceAllowedIPs: true,
			PublicKey:         wgc.ServerPublicKey,
			AllowedIPs:        wgc.Routes,
			Endpoint:          net.UDPAddrFromAddrPort(epa),
		}

		if wgc.PreSharedKey != "" {
			sk, err := wgtypes.ParseKey(wgc.PreSharedKey)
			if err != nil {
				return fmt.Errorf("error parsing key: %w", err)
			}
			pc.PresharedKey = &sk

		}

		if wgc.PersistentKeepalive != 0 {
			ka := time.Second * time.Duration(wgc.PersistentKeepalive)
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
		ip := net.ParseIP(wgc.PeerAddress)
		if ip != nil {
			addr = &net.IPNet{
				IP:   ip,
				Mask: FullMask(ip),
			}
		} else {
			ip, net, err := net.ParseCIDR(wgc.PeerAddress)
			if err != nil {
				return fmt.Errorf("error parsing address: %w", err)
			}
			addr = net
			addr.IP = ip
		}

		log.Info("syncing WireguardClusterClient", "name", wgc.PeerName, "address", addr)

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
					log.Error("Error deleting old address", "addr", addr, "error", err)
					return err
				}
			}
		}

		for _, dst := range wgc.Routes {
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
			for _, route := range wgc.Routes {
				if hasRoute.Dst.String() == route.String() {
					delete = false
				}
			}

			if delete {
				if err := netlink.RouteDel(&hasRoute); err != nil {
					log.Error("Error deleting old route", "route", hasRoute, "error", err)
					return err
				}
			}

		}

	}

	// delete leftovers
	for n, lnk := range existing {
		if err := netlink.LinkDel(lnk); err != nil {
			log.Error("Error deleting old wg interface", "if", n, "error", err)
			return err
		}
	}

	return nil
}

func FullMask(ip net.IP) net.IPMask {
	return net.IPMask(bytes.Repeat([]byte{0xff}, len(ip)))
}
