package v1beta

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type WireguardAccessRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              WireguardAccessRuleSpec `json:"spec" yaml:"spec"`
}

type WireguardAccessRuleSpec struct {
	Destinations []string `yaml:"destinations" json:"destinations"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type WireguardAccessPeer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              WireguardAccessPeerSpec    `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status            *WireguardAccessPeerStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

type WireguardAccessPeerSpec struct {
	PreSharedKey string   `yaml:"preSharedKey,omitempty" json:"preSharedKey,omitempty"`
	PublicKey    string   `yaml:"publicKey" json:"publicKey"`
	AccessRules  []string `yaml:"accessRules" json:"accessRules"`
}

type WireguardAccessPeerStatus struct {
	LastUpdated string                          `yaml:"lastUpdated,omitempty" json:"lastUpdated,omitempty"`
	Address     string                          `yaml:"address,omitempty" json:"address,omitempty"`
	Peers       []WireguardAccessPeerStatusPeer `yaml:"peers,omitempty" json:"peers,omitempty"`
}

type WireguardAccessPeerStatusPeer struct {
	PublicKey    string   `yaml:"publicKey" json:"publicKey"`
	Endpoint     string   `yaml:"endpoint" json:"endpoint"`
	PreSharedKey string   `yaml:"preSharedKey,omitempty" json:"preSharedKey,omitempty"`
	AllowedIPs   []string `yaml:"allowedIPs,omitempty" json:"allowedIPs,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type WireguardClusterClient struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              WireguardClusterClientSpec    `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status            *WireguardClusterClientStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

type WireguardClusterClientSpec struct {
	Address             string                               `yaml:"address" json:"address"`
	PrivateKeySecretRef WireguardClusterClientSpecPrivateKey `yaml:"privateKeySecretRef" json:"privateKey"`
	Server              WireguardClusterClientSpecServer     `yaml:"server" json:"server"`
	Routes              []string                             `yaml:"routes" json:"routes"`
	PersistentKeepalive int                                  `yaml:"persistentKeepalive,omitempty" json:"persistentKeepalive,omitempty"`
}

type WireguardClusterClientStatus struct {
	PublicKey string `yaml:"publicKey,omitempty" json:"publicKey,omitempty"`
}

type WireguardClusterClientSpecPrivateKey struct {
	Name      string `yaml:"name,omitempty" json:"name,omitempty"`
	Key       string `yaml:"key,omitempty" json:"key,omitempty"`
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Value     string `yaml:"value,omitempty" json:"value,omitempty"`
}
type WireguardClusterClientSpecServer struct {
	Endpoint     string `yaml:"endpoint" json:"endpoint"`
	PublicKey    string `yaml:"publicKey" json:"publicKey"`
	PreSharedKey string `yaml:"preSharedKey,omitempty" json:"preSharedKey,omitempty"`
}
