package wgav1beta

import (
	"context"
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

const GroupName = "wga.kraudcloud.com"
const GroupVersion = "v1beta"

const WireguardAccessPeerKind = "WireguardAccessPeer"
const WireguardAccessRuleKind = "WireguardAccessRule"

const WireguardAccessPeerResource = "wireguardaccesspeers"
const WireguardAccessRuleResource = "wireguardaccessrules"

var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: GroupVersion}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&WireguardAccessPeer{}, &WireguardAccessPeerList{},
		&WireguardAccessRule{}, &WireguardAccessRuleList{},
	)

	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}

type Client struct {
	restClient rest.Interface
}

func NewForConfig(c *rest.Config) (*Client, error) {
	config := *c
	config.ContentConfig.GroupVersion = &schema.GroupVersion{Group: GroupName, Version: GroupVersion}
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()

	AddToScheme(scheme.Scheme)

	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	return &Client{restClient: client}, nil
}

func (c *Client) ListWireguardAccessPeers(ctx context.Context, opts metav1.ListOptions) (*WireguardAccessPeerList, error) {
	result := &WireguardAccessPeerList{}

	opts.TypeMeta = metav1.TypeMeta{
		Kind:       WireguardAccessPeerKind,
		APIVersion: SchemeGroupVersion.String(),
	}

	err := c.restClient.Get().
		Resource(WireguardAccessPeerResource).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *Client) ListWireguardAccessRules(ctx context.Context, opts metav1.ListOptions) (*WireguardAccessRuleList, error) {
	result := &WireguardAccessRuleList{}

	opts.TypeMeta = metav1.TypeMeta{
		Kind:       WireguardAccessRuleKind,
		APIVersion: SchemeGroupVersion.String(),
	}

	err := c.restClient.Get().
		Resource(WireguardAccessRuleResource).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *Client) GetWireguardAccessPeer(ctx context.Context, name string, opts metav1.GetOptions) (*WireguardAccessPeer, error) {
	result := &WireguardAccessPeer{}
	err := c.restClient.Get().
		Resource(WireguardAccessPeerResource).
		Name(name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return result, err
}

func (c *Client) CreateWireguardAccessPeer(ctx context.Context, p WireguardAccessPeer) (result *WireguardAccessPeer, err error) {
	result = &WireguardAccessPeer{}
	buf, err := json.Marshal(p)
	if err != nil {
		return
	}

	err = c.restClient.Post().
		Resource(WireguardAccessPeerResource).
		Body(buf).
		Do(ctx).
		Into(result)
	return
}

func (c *Client) PutWireguardAccessPeer(ctx context.Context, name string, p WireguardAccessPeer) (result *WireguardAccessPeer, err error) {
	result = &WireguardAccessPeer{}

	buf, err := json.Marshal(p)
	if err != nil {
		return
	}

	err = c.restClient.Put().
		Resource(WireguardAccessPeerResource).
		Name(name).
		Body(buf).
		Do(ctx).
		Into(result)
	return
}

func (c *Client) WatchWireguardAccessPeers(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	opts.TypeMeta = metav1.TypeMeta{
		Kind:       WireguardAccessPeerKind,
		APIVersion: SchemeGroupVersion.String(),
	}

	return c.restClient.Get().
		Resource(WireguardAccessPeerResource).
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch(ctx)
}
