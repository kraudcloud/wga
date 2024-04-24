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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Config struct {
	EP    *WireguardAccessEndpoint
	Rules []WireguardAccessRule
	Peers []WireguardAccessPeer
}

type WireguardAccessEndpointSpec struct {
	ClientCIDR string `yaml:"clientCIDR"`
}

type WireguardAccessEndpoint struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ObjectMeta           `json:"metadata,omitempty"`
	Spec            WireguardAccessEndpointSpec `json:"spec"`
}

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

func epMain(name string) {

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

	clientSetK, err := kubernetes.NewForConfig(config)
	if err != nil {
		slog.Error("Error creating Kubernetes client", "error", err)
		os.Exit(1)
	}

	gvr1 := schema.GroupVersionResource{
		Group:    "wga.kraudcloud.com",
		Version:  "v1beta",
		Resource: "wireguardaccessendpoints",
	}

	gvr2 := schema.GroupVersionResource{
		Group:    "wga.kraudcloud.com",
		Version:  "v1beta",
		Resource: "wireguardaccesspeers",
	}

	gvr3 := schema.GroupVersionResource{
		Group:    "wga.kraudcloud.com",
		Version:  "v1beta",
		Resource: "wireguardaccesspeers",
	}

	handler := func() {
		cfg, err := Fetch(clientset, clientSetK, name, gvr1, gvr2, gvr3)
		if err != nil {
			slog.Error("Error fetching CRDs", "error", err)
		}

		if cfg.EP == nil {
			slog.Error("WireguardAccessEndpoint not found", "name", name)
			return
		}

		wgSync(cfg)
		nftSync(cfg)
		sysctl(cfg)
	}

	go watchCR(clientset, gvr1, handler)
	go watchCR(clientset, gvr2, handler)
	go watchCR(clientset, gvr3, handler)

	// Wait for termination signal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	slog.Info("Received termination signal. Exiting.")
}

func Fetch(clientset dynamic.Interface, clientSetK kubernetes.Interface, name string, gvr1, gvr2, gvr3 schema.GroupVersionResource) (*Config, error) {

	wga, err := clientset.Resource(gvr1).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var wgaep *WireguardAccessEndpoint
	for _, item := range wga.Items {

		tt := &WireguardAccessEndpoint{}
		buf, _ := item.MarshalJSON()

		err = json.Unmarshal(buf, tt)
		if err != nil {
			slog.Error("Error decoding CR", "error", err)
			continue
		}

		if tt.Metadata.Name != name {
			continue
		}

		wgaep = tt
	}

	wgap, err := clientset.Resource(gvr2).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	wgapList := []WireguardAccessPeer{}
	for _, item := range wgap.Items {

		wgap := WireguardAccessPeer{}
		buf, _ := item.MarshalJSON()

		err = json.Unmarshal(buf, &wgap)
		if err != nil {
			slog.Error("Error decoding CR", "error", err)
			continue
		}

		wgapList = append(wgapList, wgap)

	}

	wgar, err := clientset.Resource(gvr3).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	wgarList := []WireguardAccessRule{}
	for _, item := range wgar.Items {

		wgar := WireguardAccessRule{}
		buf, _ := item.MarshalJSON()

		err = json.Unmarshal(buf, &wgar)
		if err != nil {
			slog.Error("Error decoding CR", "error", err)
			continue
		}

		wgarList = append(wgarList, wgar)
	}

	return &Config{
		EP:    wgaep,
		Rules: wgarList,
		Peers: wgapList,
	}, nil

}

func watchCR(clientset dynamic.Interface, gvr schema.GroupVersionResource, handler func()) {
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
