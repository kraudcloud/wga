package operator

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"math/rand/v2"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/go-logr/logr"
	"github.com/kraudcloud/wga/pkgs/apis/v1beta"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
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

const (
	DEVICENAME = "wga"
)

// WGConfig is readonly after `wgInit` is called.
var (
	WGConfig   = wgtypes.Config{}
	WGInitOnce = sync.Once{}
)

func init() {
	v1beta.SchemeBuilder.AddToScheme(scheme.Scheme)
}

func RunWGA(ctx context.Context, config *rest.Config, serviceNets []net.IPNet, peerNets []net.IPNet, dnsServers []string, serverAddr string) {
	mgr, err := manager.New(config, manager.Options{})
	if err != nil {
		slog.Error("unable to create new manager", "err", err)
		os.Exit(1)
	}

	log.SetLogger(logr.FromSlogHandler(slog.With("component", "wga-controller").Handler()))

	registerLoadBalancerReconciler(mgr, serviceNets, slog.Default())
	registerPeerReconciler(mgr, serviceNets, peerNets, dnsServers, serverAddr, slog.Default())

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

var peerPredicate = &predicate.TypedFuncs[client.Object]{
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

func registerPeerReconciler(
	mgr manager.Manager,
	servicesNets []net.IPNet,
	clientsNets []net.IPNet,
	dnsServers []string,
	serverAddr string,
	log *slog.Logger,
) {
	epInit(&clientsNets[0])

	err := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta.WireguardAccessPeer{}).
		WithEventFilter(peerPredicate).
		Owns(&v1beta.WireguardAccessPeer{}, builder.WithPredicates(peerPredicate)).
		Watches(&v1beta.WireguardAccessPeer{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
			return []reconcile.Request{{NamespacedName: client.ObjectKeyFromObject(o)}}
		}), builder.WithPredicates(peerPredicate)).
		Complete(reconcile.AsReconciler(mgr.GetClient(), &PeerReconciler{
			serverAddr:   serverAddr,
			clientsNets:  clientsNets,
			servicesNets: servicesNets,
			dnsServers:   dnsServers,
			client:       mgr.GetClient(),
			log:          log.With("component", "peer-reconciler"),
		}))
	if err != nil {
		log.Error("Error creating peer reconciler", "error", err)
		os.Exit(1)
	}

	err = ctrl.NewControllerManagedBy(mgr).
		For(&v1beta.WireguardAccessRule{}).
		WithEventFilter(peerPredicate).
		Owns(&v1beta.WireguardAccessRule{}, builder.WithPredicates(peerPredicate)).
		Watches(&v1beta.WireguardAccessRule{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
			return []reconcile.Request{{NamespacedName: client.ObjectKeyFromObject(o)}}
		}), builder.WithPredicates(peerPredicate)).
		Complete(reconcile.AsReconciler(mgr.GetClient(), &RulesReconciler{
			client: mgr.GetClient(),
			log:    log.With("component", "rules-reconciler"),
		}))
	if err != nil {
		log.Error("Error creating peer reconciler", "error", err)
		os.Exit(1)
	}
}

type RulesReconciler struct {
	client client.Client
	log    *slog.Logger
}

func (r *RulesReconciler) Reconcile(ctx context.Context, rule *v1beta.WireguardAccessRule) (ctrl.Result, error) {
	r.log.Info("reconciling rule", "rule", rule.Name)
	return ctrl.Result{}, WGASync(r.client, r.log)
}

type PeerReconciler struct {
	serverAddr   string
	clientsNets  []net.IPNet
	servicesNets []net.IPNet
	dnsServers   []string
	client       client.Client
	log          *slog.Logger
}

func (r *PeerReconciler) Reconcile(ctx context.Context, peer *v1beta.WireguardAccessPeer) (ctrl.Result, error) {
	if peer.Status != nil && peer.Status.Address != "" {
		return ctrl.Result{}, nil
	}

	r.log.Info("setting peer status", "peer", peer.Name)

	cnet := randPick(r.clientsNets)
	sip, err := cidr.HostBig(&cnet, generateIndex(time.Now(), maskBits(cnet)))
	if err != nil {
		r.log.Error(err.Error(), "peer", peer.Name)
		return ctrl.Result{}, err
	}

	peer.Status = &v1beta.WireguardAccessPeerStatus{
		LastUpdated: metav1.Now(),
		Address:     sip.String(),
		DNS:         r.dnsServers,
		Peers: []v1beta.WireguardAccessPeerStatusPeer{
			{
				PublicKey:  WGConfig.PrivateKey.PublicKey().String(),
				Endpoint:   net.JoinHostPort(r.serverAddr, strconv.FormatInt(int64(*WGConfig.ListenPort), 10)),
				AllowedIPs: netsAsStrings(r.servicesNets),
			},
		},
	}

	err = r.client.Update(ctx, peer)
	if err != nil {
		slog.Error(err.Error(), "peer", peer.Name)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, WGASync(r.client, r.log)
}

func netsAsStrings(nets []net.IPNet) []string {
	var netsStr []string
	for _, n := range nets {
		netsStr = append(netsStr, n.String())
	}
	return netsStr
}

type Config struct {
	Rules []v1beta.WireguardAccessRule
	Peers []v1beta.WireguardAccessPeer
}

func Fetch(ctx context.Context, client client.Client) (*Config, error) {
	wgap := new(v1beta.WireguardAccessPeerList)

	err := client.List(ctx, wgap)
	if err != nil {
		return nil, fmt.Errorf("error listing wga: %w", err)
	}

	wgar := new(v1beta.WireguardAccessRuleList)
	err = client.List(ctx, wgar)
	if err != nil {
		return nil, fmt.Errorf("error listing wga: %w", err)
	}

	return &Config{
		Rules: wgar.Items,
		Peers: wgap.Items,
	}, nil
}

func WGASync(client client.Client, log *slog.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cfg, err := Fetch(ctx, client)
	if err != nil {
		log.Error("Error fetching CRDs", "error", err)
		return nil
	}

	log.Debug("syncing wg")
	err = wgaSync(log, cfg)
	if err != nil {
		log.Error("Error syncing CRDs", "error", err)
	}
	log.Debug("syncing wg done")

	log.Debug("syncing nft")
	nftSync(ctx, log, cfg, "wga")
	log.Debug("syncing nft done")

	log.Debug("syncing sysctl")
	sysctl(ctx, log)
	log.Debug("syncing sysctl done")

	return nil
}

func epInit(clientCIDR *net.IPNet) {
	WGInitOnce.Do(func() {
		if err := wgaInit(clientCIDR); err != nil {
			panic(err)
		}
	})

	NFTInitOnce.Do(nftInit)
}

func wgaInit(clientCIDR *net.IPNet) error {
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

	err = netlink.RouteAdd(&netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       clientCIDR,
	})
	if err != nil {
		return fmt.Errorf("cannot add route: %w", err)
	}

	return nil
}

func wgaSync(log *slog.Logger, config *Config) error {
	shouldPeers := make(map[string]wgtypes.PeerConfig, 0)
	log.Debug("syncing peers")
	for _, peer := range config.Peers {
		if peer.Status == nil {
			continue
		}

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

func maskBits(cidr net.IPNet) int {
	ones, bits := cidr.Mask.Size()
	return bits - ones
}

func bigMax(bits int) *big.Int {
	return new(big.Int).Exp(big.NewInt(2), big.NewInt(int64(bits)), nil)
}

// generate a new index for the given cidr range
// this is used to generate a unique ip.
//
// Fill the top bits with time, and the bottom with
// at least 16 random bits.
func generateIndex(t time.Time, mask int) *big.Int {
	if mask >= 128 {
		panic("mask too large")
	}

	z := big.NewInt(0)
	z.SetInt64(t.UnixNano())

	// Calculate the required size for random bits
	// Ensures at least 16 bits and utilizes remaining bits in mask after time bits
	randSize := max(16, mask-64)
	r := big.NewInt(rand.Int64N(1<<uint(randSize) - 1))

	// Move the random bits to the bottom of the int
	z.Lsh(z, uint(randSize))
	z.Or(z, r)

	z.Add(z, r)
	z.Rem(z, bigMax(mask))
	return z
}

func readKey() (wgtypes.Key, error) {
	pkstr, err := os.ReadFile("/etc/wga/endpoint/privateKey")
	if err != nil {
		return wgtypes.Key{}, fmt.Errorf("cannot read private key from /etc/wga/endpoint/privateKey: %w", err)
	}

	return wgtypes.ParseKey(strings.TrimSpace(string(pkstr)))
}

func randPick[T any](s []T) T {
	return s[rand.IntN(len(s))]
}
