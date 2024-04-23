package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

type WireguardAccessRuleSpec struct {
	Destinations []string `yaml:"destinations" json:"destinations"`
}

type WireguardAccessRule struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ObjectMeta       `json:"metadata,omitempty"`
	Spec            WireguardAccessRuleSpec `json:"spec"`
}

type WireguardAccessPeerSpec struct {
	Index     int      `yaml:"index" json:"index"`
	PublicKey string   `yaml:"pub" json:"pub"`
	PSK       string   `yaml:"psk" json:"psk"`
	Rules     []string `yaml:"rules" json:"rules"`
}

type WireguardAccessPeer struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ObjectMeta       `json:"metadata,omitempty"`
	Spec            WireguardAccessPeerSpec `json:"spec"`
}

func crdMain() {
	// Create a Kubernetes client
	config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		slog.Error("Error building Kubernetes config", "error", err)
		os.Exit(1)
	}

	clientset, err := dynamic.NewForConfig(config)
	if err != nil {
		slog.Error("Error creating Kubernetes client", "error", err)
		os.Exit(1)
	}

	// Start watching the CR
	go watchCR(clientset, schema.GroupVersionResource{
		Group:    "wga.kraudcloud.com",
		Version:  "v1beta",
		Resource: "wireguardaccesspeers",
	}, watchHandler[WireguardAccessPeer]{
		onAdded: func(p WireguardAccessPeer) {
			slog.Info("Peer added", "peer", p.Metadata.Name, "index", p.Spec.Index, "pub", p.Spec.PublicKey, "psk", p.Spec.PSK, "rules", p.Spec.Rules)
		},
		onModified: func(p WireguardAccessPeer) {
			slog.Info("Peer modified", "peer", p.Metadata.Name, "index", p.Spec.Index, "pub", p.Spec.PublicKey, "psk", p.Spec.PSK, "rules", p.Spec.Rules)
		},
		onDeleted: func(p WireguardAccessPeer) {
			slog.Info("Peer deleted", "peer", p.Metadata.Name, "index", p.Spec.Index, "pub", p.Spec.PublicKey, "psk", p.Spec.PSK, "rules", p.Spec.Rules)
		},
	})

	go watchCR(clientset, schema.GroupVersionResource{
		Group:    "wga.kraudcloud.com",
		Version:  "v1beta",
		Resource: "wireguardaccessrules",
	}, watchHandler[WireguardAccessRule]{
		onAdded: func(r WireguardAccessRule) {
			slog.Info("Rule added", "rule", r.Metadata.Name, "destinations", r.Spec.Destinations)
		},
		onModified: func(r WireguardAccessRule) {
			slog.Info("Rule modified", "rule", r.Metadata.Name, "destinations", r.Spec.Destinations)
		},
		onDeleted: func(r WireguardAccessRule) {
			slog.Info("Rule deleted", "rule", r.Metadata.Name, "destinations", r.Spec.Destinations)
		},
	})

	// Wait for termination signal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	slog.Info("Received termination signal. Exiting.")
}

type watchHandler[T any] struct {
	onAdded    func(T)
	onModified func(T)
	onDeleted  func(T)
}

func watchCR[T any](clientset dynamic.Interface, gvr schema.GroupVersionResource, handler watchHandler[T]) {
	watch, err := clientset.Resource(gvr).Watch(context.TODO(), metav1.ListOptions{
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

		// encode then decode json, the k8s go API sucks.
		// It doesn't have a way to decode into a proper struct so we have to revert to using reflection
		// or pull every field out individually. ew.
		v := new(T)
		buf, _ := cr.MarshalJSON()

		err := json.Unmarshal(buf, v)
		if err != nil {
			slog.Error("Error decoding CR", "error", err)
			continue
		}

		// Handle the event based on its type
		switch event.Type {
		case "ADDED":
			handler.onAdded(*v)
		case "MODIFIED":
			handler.onModified(*v)
		case "DELETED":
			handler.onDeleted(*v)
		}
	}
}
