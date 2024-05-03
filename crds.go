package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/kraudcloud/wga/apis/generated/clientset/versioned"
	"github.com/kraudcloud/wga/apis/generated/controllers/wga.kraudcloud.com"
	"github.com/kraudcloud/wga/apis/wga.kraudcloud.com/v1beta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Config struct {
	Rules []v1beta.WireguardAccessRule
	Peers []v1beta.WireguardAccessPeer
}

func epMain(
	ctx context.Context,
	clientCIDR *net.IPNet,
	serverAddr string,
	allowedIPs []string,
) {
	epInit()

	config, err := clientConfig()
	if err != nil {
		slog.Error("Error building Kubernetes config", "error", err)
		os.Exit(1)
	}

	client, err := versioned.NewForConfig(config)
	if err != nil {
		slog.Error("Error building CRD client", "error", err)
		os.Exit(1)
	}

	controller := wga.NewFactoryFromConfigOrDie(config)
	controller.Wga().V1beta().WireguardAccessPeer().OnChange(ctx, "on-change", OnPeerChange(serverAddr, allowedIPs, clientCIDR, client))
	controller.Wga().V1beta().WireguardAccessPeer().AddGenericHandler(ctx, "ep-main", OnEvent(client, "WireguardAccessPeer"))
	controller.Wga().V1beta().WireguardAccessRule().AddGenericHandler(ctx, "ep-main", OnEvent(client, "WireguardAccessRule"))

	err = controller.Sync(ctx)
	if err != nil {
		slog.Error("Error starting controller", "error", err)
		os.Exit(1)
	}

	err = controller.Start(ctx, 2)
	if err != nil {
		slog.Error("Error starting controller", "error", err)
		os.Exit(1)
	}

	<-ctx.Done()
}

func Fetch(ctx context.Context, client *versioned.Clientset) (*Config, error) {
	wgap, err := client.WgaV1beta().WireguardAccessPeers().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing peers: %w", err)
	}

	wgar, err := client.WgaV1beta().WireguardAccessRules().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing rules: %w", err)
	}

	return &Config{
		Rules: wgar.Items,
		Peers: wgap.Items,
	}, nil
}

// TODO: handle psk changes
func OnPeerChange(serverAddr string, allowedIPs []string, clientCIDR *net.IPNet, client *versioned.Clientset) func(key string, peer *v1beta.WireguardAccessPeer) (*v1beta.WireguardAccessPeer, error) {
	return func(key string, peer *v1beta.WireguardAccessPeer) (*v1beta.WireguardAccessPeer, error) {
		// can happen if the peer is deleted. We don't really care though
		if peer == nil {
			return nil, nil
		}

		if peer.Status == nil || (peer.Labels != nil && peer.Labels[ForceRefreshSpec] == "true") {
			sip, err := cidr.Host(clientCIDR, generateIndex(clientCIDR))
			if err != nil {
				slog.Error(err.Error(), "peer", peer.Name)
			}

			// remove the force refresh label
			if peer.Labels != nil {
				delete(peer.Labels, ForceRefreshSpec)
			}

			slog.Info("setting peer status", "peer", peer.Name)
			rsp, err := client.WgaV1beta().WireguardAccessPeers().Update(context.TODO(), &v1beta.WireguardAccessPeer{
				TypeMeta:   peer.TypeMeta,
				Spec:       peer.Spec,
				ObjectMeta: peer.ObjectMeta,
				Status: &v1beta.WireguardAccessPeerStatus{
					LastUpdated: time.Now().Format(time.RFC3339),
					Address:     sip.String(),
					Peers: []v1beta.WireguardAccessPeerStatusPeer{
						{
							PublicKey:  WGConfig.PrivateKey.PublicKey().String(),
							Endpoint:   net.JoinHostPort(serverAddr, strconv.FormatInt(int64(*WGConfig.ListenPort), 10)),
							AllowedIPs: allowedIPs,
						},
					},
				},
			}, metav1.UpdateOptions{})
			if err != nil {
				slog.Error(err.Error(), "peer", peer.Name)
			}

			return rsp, nil
		}

		return peer, nil
	}
}

func OnEvent(client *versioned.Clientset, kind string) func(key string, obj runtime.Object) (runtime.Object, error) {
	return func(key string, obj runtime.Object) (runtime.Object, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		log := slog.With("eventKind", kind)
		cfg, err := Fetch(ctx, client)
		if err != nil {
			log.Error("Error fetching CRDs", "error", err)
			return obj, nil
		}

		log.Debug("syncing wg")
		err = wgSync(log, cfg, client)
		if err != nil {
			log.Error("Error syncing CRDs", "error", err)
		}
		log.Debug("syncing wg done")

		log.Debug("syncing nft")
		nftSync(ctx, log, cfg)
		log.Debug("syncing nft done")

		log.Debug("syncing sysctl")
		sysctl(ctx, log)
		log.Debug("syncing sysctl done")

		return obj, nil
	}
}

// clientConfig loads the config either from kubeconfig or falls back to the cluster
// the k8s client has a similar function but it logs stuff when trying to fallback.
func clientConfig() (*rest.Config, error) {
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}, &clientcmd.ConfigOverrides{},
		).ClientConfig()
	}

	return rest.InClusterConfig()
}

func epInit() {
	WGInitOnce.Do(func() {
		if err := wgInit(); err != nil {
			panic(err)
		}
	})

	NFTInitOnce.Do(nftInit)
}
