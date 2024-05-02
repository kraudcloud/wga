package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kraudcloud/wga/wgav1beta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Config struct {
	Rules []wgav1beta.WireguardAccessRule
	Peers []wgav1beta.WireguardAccessPeer
}

func epMain() {
	config, err := clientConfig()
	if err != nil {
		slog.Error("Error building Kubernetes config", "error", err)
		os.Exit(1)
	}

	clientset, err := dynamic.NewForConfig(config)
	if err != nil {
		slog.Error("Error creating Kubernetes client", "error", err)
		os.Exit(1)
	}

	crdClient, err := wgav1beta.NewForConfig(config)
	if err != nil {
		slog.Error("Error creating CRD client", "error", err)
		os.Exit(1)
	}

	wgaPeers := schema.GroupVersionResource{
		Group:    "wga.kraudcloud.com",
		Version:  "v1beta",
		Resource: "wireguardaccesspeers",
	}

	wgaRules := schema.GroupVersionResource{
		Group:    "wga.kraudcloud.com",
		Version:  "v1beta",
		Resource: "wireguardaccessrules",
	}

	handler := func(event *unstructured.Unstructured) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		log := slog.With("eventID", event.GetUID())
		log.Info("syncing CRDs", "name", event.GetName(), "resource", event.GetKind())

		cfg, err := Fetch(ctx, crdClient)
		if err != nil {
			log.Error("Error fetching CRDs", "error", err)
			return
		}

		log.Debug("syncing wg")
		err = wgSync(ctx, log.With("sub", "wg"), cfg, crdClient)
		if err != nil {
			log.Error("Error syncing CRDs", "error", err)
		}
		log.Debug("syncing wg done")

		log.Debug("syncing nft")
		nftSync(ctx, log.With("sub", "nft"), cfg)
		log.Debug("syncing nft done")

		log.Debug("syncing sysctl")
		sysctl(ctx, log.With("sub", "sysctl"))
		log.Debug("syncing sysctl done")
	}

	go watchCR(context.Background(), clientset, wgaPeers, handler)
	go watchCR(context.Background(), clientset, wgaRules, handler)

	// Wait for termination signal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	slog.Info("Received termination signal. Exiting.")
}

func Fetch(ctx context.Context, client *wgav1beta.Client) (*Config, error) {
	wgap, err := client.ListWireguardAccessPeers(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing peers: %w", err)
	}

	wgar, err := client.ListWireguardAccessRules(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing rules: %w", err)
	}

	return &Config{
		Rules: wgar.Items,
		Peers: wgap.Items,
	}, nil
}

func watchCR(ctx context.Context, clientset dynamic.Interface, gvr schema.GroupVersionResource, handler func(event *unstructured.Unstructured)) {
	watch, err := clientset.Resource(gvr).Watch(ctx, metav1.ListOptions{
		Watch: true,
	})
	if err != nil {
		slog.Error("Error watching CR", "error", err)
		return
	}
	defer watch.Stop()

	slog.Info("Watching CR", "resource", gvr.Resource)

	for event := range watch.ResultChan() {
		cr := event.Object.(*unstructured.Unstructured)
		if event.Type == "ERROR" {
			slog.Error("Error watching CR", "error", err)
			continue
		}

		if event.Type == "BOOKMARK" {
			slog.Info("Received bookmark event", "cr", cr.GetName())
			continue
		}

		go handler(cr)
	}

	slog.Error("Watching CR stopped", "resource", gvr.Resource)
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
