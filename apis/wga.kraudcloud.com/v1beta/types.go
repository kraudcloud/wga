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
