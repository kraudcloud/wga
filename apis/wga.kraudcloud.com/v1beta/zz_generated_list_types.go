// Code generated by codegen. DO NOT EDIT.

// +k8s:deepcopy-gen=package
// +groupName=wga.kraudcloud.com
package v1beta

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WireguardAccessPeerList is a list of WireguardAccessPeer resources
type WireguardAccessPeerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []WireguardAccessPeer `json:"items"`
}

func NewWireguardAccessPeer(namespace, name string, obj WireguardAccessPeer) *WireguardAccessPeer {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("WireguardAccessPeer").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WireguardAccessRuleList is a list of WireguardAccessRule resources
type WireguardAccessRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []WireguardAccessRule `json:"items"`
}

func NewWireguardAccessRule(namespace, name string, obj WireguardAccessRule) *WireguardAccessRule {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("WireguardAccessRule").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}