package v1beta

import (
	corev1 "k8s.io/api/core/v1"
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
	metav1.TypeMeta `json:",inline"`
	//+optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              WireguardAccessPeerSpec `json:"spec" yaml:"spec"`
	//+optional
	Status *WireguardAccessPeerStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

type WireguardAccessPeerSpec struct {
	//+optional
	PreSharedKey string   `yaml:"preSharedKey,omitempty" json:"preSharedKey,omitempty"`
	PublicKey    string   `yaml:"publicKey" json:"publicKey"`
	AccessRules  []string `yaml:"accessRules" json:"accessRules"`
}

type WireguardAccessPeerStatus struct {
	//+optional
	LastUpdated metav1.Time                     `yaml:"lastUpdated,omitempty" json:"lastUpdated,omitempty"`
	Address     string                          `yaml:"address" json:"address"`
	Peers       []WireguardAccessPeerStatusPeer `yaml:"peers" json:"peers"`
}

type WireguardAccessPeerStatusPeer struct {
	PublicKey string `yaml:"publicKey" json:"publicKey"`
	Endpoint  string `yaml:"endpoint" json:"endpoint"`
	//+optional
	PreSharedKey string   `yaml:"preSharedKey,omitempty" json:"preSharedKey,omitempty"`
	AllowedIPs   []string `yaml:"allowedIPs" json:"allowedIPs"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WireguardClusterClient struct {
	metav1.TypeMeta `json:",inline"`
	//+optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec WireguardClusterClientSpec `json:"spec" yaml:"spec"`

	//+optional
	Status *WireguardClusterClientStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

type WireguardClusterClientSpec struct {
	Address string                           `yaml:"address" json:"address"`
	Nodes   []WireguardClusterClientNode     `yaml:"nodes" json:"nodes"`
	Server  WireguardClusterClientSpecServer `yaml:"server" json:"server"`
	Routes  []string                         `yaml:"routes" json:"routes"`
	//+optional
	PersistentKeepalive int `yaml:"persistentKeepalive,omitempty" json:"persistentKeepalive,omitempty"`
}

type WireguardClusterClientNode struct {
	NodeName string `yaml:"nodeName" json:"nodeName"`

	//+optional
	PreSharedKey string `yaml:"preSharedKey,omitempty" json:"preSharedKey,omitempty"`

	PrivateKey WireguardClusterClientNodePrivateKey `yaml:"privateKey" json:"privateKey"`
}

type WireguardClusterClientNodePrivateKey struct {
	// oneOf
	//+optional
	Value string `yaml:"value,omitempty" json:"value,omitempty"`
	//+optional
	SecretRef corev1.SecretReference `yaml:"secretRef,omitempty" json:"secretRef,omitempty"`
}

type WireguardClusterClientStatus struct {
	PublicKey string `yaml:"publicKey,omitempty" json:"publicKey,omitempty"`
}

type WireguardClusterClientSpecServer struct {
	Endpoint  string `yaml:"endpoint" json:"endpoint"`
	PublicKey string `yaml:"publicKey" json:"publicKey"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type WireguardAccessPeerList struct {
	metav1.TypeMeta `json:",inline"`
	//+optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WireguardAccessPeer `json:"items" yaml:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type WireguardAccessRuleList struct {
	metav1.TypeMeta `json:",inline"`
	//+optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WireguardAccessRule `json:"items" yaml:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type WireguardClusterClientList struct {
	metav1.TypeMeta `json:",inline"`
	//+optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WireguardClusterClient `json:"items" yaml:"items"`
}
