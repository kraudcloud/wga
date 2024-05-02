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

	handler := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cfg, err := Fetch(ctx, crdClient)
		if err != nil {
			slog.Error("Error fetching CRDs", "error", err)
			return
		}

		err = wgSync(ctx, cfg, crdClient)
		if err != nil {
			slog.Error("Error syncing CRDs", "error", err)
		}

		nftSync(ctx, cfg)
		sysctl()
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

func watchCR(ctx context.Context, clientset dynamic.Interface, gvr schema.GroupVersionResource, handler func()) {
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
		slog.Info("Event received", "event", event.Type, "cr", cr.GetName())

		if event.Type == "ERROR" {
			slog.Error("Error watching CR", "error", err)
			continue
		}

		if event.Type == "BOOKMARK" {
			slog.Info("Received bookmark event", "cr", cr.GetName())
			continue
			// TODO: Handle bookmark events idk what they do
		}

		// Handle the event based on its type
		switch event.Type {
		case "ADDED":
			slog.Info("CR added", "cr", cr.GetName())
		case "MODIFIED":
			slog.Info("CR modified", "cr", cr.GetName())
		case "DELETED":
			slog.Info("CR deleted", "cr", cr.GetName())
		}

		handler()
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
