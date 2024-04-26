package wgav1beta

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type WireguardAccessRuleSpec struct {
	Destinations []string `yaml:"destinations" json:"destinations"`
}

type WireguardAccessRule struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	Metadata        metav1.ObjectMeta       `json:"metadata,omitempty"`
	Spec            WireguardAccessRuleSpec `json:"spec" yaml:"spec"`
}

type WireguardAccessRuleList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []WireguardAccessRule `json:"items" yaml:"items"`
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

type WireguardAccessPeer struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	Metadata        metav1.ObjectMeta          `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec            WireguardAccessPeerSpec    `json:"spec" yaml:"spec"`
	Status          *WireguardAccessPeerStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

type WireguardAccessPeerList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items           []WireguardAccessPeer `json:"items" yaml:"items"`
}

func (in *WireguardAccessRuleList) DeepCopyObject() runtime.Object {
	out := WireguardAccessRuleList{}
	in.DeepCopyInto(&out)
	return &out
}

func (in *WireguardAccessRuleList) DeepCopyInto(out *WireguardAccessRuleList) {
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items == nil {
		out.Items = nil
		return
	}

	out.Items = make([]WireguardAccessRule, len(in.Items))
	for i := range in.Items {
		in.Items[i].DeepCopyInto(&out.Items[i])
	}
}

func (in *WireguardAccessPeerList) DeepCopyObject() runtime.Object {
	out := WireguardAccessPeerList{}
	in.DeepCopyInto(&out)
	return &out
}

func (in *WireguardAccessPeerList) DeepCopyInto(out *WireguardAccessPeerList) {
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items == nil {
		out.Items = nil
		return
	}

	out.Items = make([]WireguardAccessPeer, len(in.Items))
	for i := range in.Items {
		in.Items[i].DeepCopyInto(&out.Items[i])
	}
}

// DeepCopyInto copies all properties of this object into another object of the
// same type that is provided as a pointer.
func (in *WireguardAccessPeer) DeepCopyInto(out *WireguardAccessPeer) {
	out.TypeMeta = in.TypeMeta
	out.Metadata = in.Metadata
	out.Spec = in.Spec
	out.Status = in.Status
}

// DeepCopyObject returns a generically typed copy of an object
func (in *WireguardAccessPeer) DeepCopyObject() runtime.Object {
	out := WireguardAccessPeer{}
	in.DeepCopyInto(&out)

	return &out
}

// DeepCopyInto copies all properties of this object into another object of the
// same type that is provided as a pointer.
func (in *WireguardAccessRule) DeepCopyInto(out *WireguardAccessRule) {
	out.TypeMeta = in.TypeMeta
	out.Metadata = in.Metadata
	out.Spec = in.Spec
}

func (in *WireguardAccessRule) DeepCopyObject() runtime.Object {
	out := WireguardAccessRule{}
	in.DeepCopyInto(&out)
	return &out
}
