// Code generated by codegen. DO NOT EDIT.

package v1beta

import (
	v1beta "github.com/kraudcloud/wga/apis/wga.kraudcloud.com/v1beta"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// WireguardAccessRuleLister helps list WireguardAccessRules.
// All objects returned here must be treated as read-only.
type WireguardAccessRuleLister interface {
	// List lists all WireguardAccessRules in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1beta.WireguardAccessRule, err error)
	// Get retrieves the WireguardAccessRule from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1beta.WireguardAccessRule, error)
	WireguardAccessRuleListerExpansion
}

// wireguardAccessRuleLister implements the WireguardAccessRuleLister interface.
type wireguardAccessRuleLister struct {
	indexer cache.Indexer
}

// NewWireguardAccessRuleLister returns a new WireguardAccessRuleLister.
func NewWireguardAccessRuleLister(indexer cache.Indexer) WireguardAccessRuleLister {
	return &wireguardAccessRuleLister{indexer: indexer}
}

// List lists all WireguardAccessRules in the indexer.
func (s *wireguardAccessRuleLister) List(selector labels.Selector) (ret []*v1beta.WireguardAccessRule, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta.WireguardAccessRule))
	})
	return ret, err
}

// Get retrieves the WireguardAccessRule from the index for a given name.
func (s *wireguardAccessRuleLister) Get(name string) (*v1beta.WireguardAccessRule, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1beta.Resource("wireguardaccessrule"), name)
	}
	return obj.(*v1beta.WireguardAccessRule), nil
}