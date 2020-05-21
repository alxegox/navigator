/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by client-gen. DO NOT EDIT.

package v1

import (
	"time"

	scheme "github.com/alxegox/navigator/pkg/apis/generated/clientset/versioned/scheme"
	v1 "github.com/alxegox/navigator/pkg/apis/navigator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// GatewaysGetter has a method to return a GatewayInterface.
// A group's client should implement this interface.
type GatewaysGetter interface {
	Gateways(namespace string) GatewayInterface
}

// GatewayInterface has methods to work with Gateway resources.
type GatewayInterface interface {
	Create(*v1.Gateway) (*v1.Gateway, error)
	Update(*v1.Gateway) (*v1.Gateway, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error
	Get(name string, options metav1.GetOptions) (*v1.Gateway, error)
	List(opts metav1.ListOptions) (*v1.GatewayList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Gateway, err error)
	GatewayExpansion
}

// gateways implements GatewayInterface
type gateways struct {
	client rest.Interface
	ns     string
}

// newGateways returns a Gateways
func newGateways(c *NavigatorV1Client, namespace string) *gateways {
	return &gateways{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the gateway, and returns the corresponding gateway object, and an error if there is any.
func (c *gateways) Get(name string, options metav1.GetOptions) (result *v1.Gateway, err error) {
	result = &v1.Gateway{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("gateways").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Gateways that match those selectors.
func (c *gateways) List(opts metav1.ListOptions) (result *v1.GatewayList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1.GatewayList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("gateways").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested gateways.
func (c *gateways) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("gateways").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch()
}

// Create takes the representation of a gateway and creates it.  Returns the server's representation of the gateway, and an error, if there is any.
func (c *gateways) Create(gateway *v1.Gateway) (result *v1.Gateway, err error) {
	result = &v1.Gateway{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("gateways").
		Body(gateway).
		Do().
		Into(result)
	return
}

// Update takes the representation of a gateway and updates it. Returns the server's representation of the gateway, and an error, if there is any.
func (c *gateways) Update(gateway *v1.Gateway) (result *v1.Gateway, err error) {
	result = &v1.Gateway{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("gateways").
		Name(gateway.Name).
		Body(gateway).
		Do().
		Into(result)
	return
}

// Delete takes name of the gateway and deletes it. Returns an error if one occurs.
func (c *gateways) Delete(name string, options *metav1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("gateways").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *gateways) DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	var timeout time.Duration
	if listOptions.TimeoutSeconds != nil {
		timeout = time.Duration(*listOptions.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("gateways").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Timeout(timeout).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched gateway.
func (c *gateways) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Gateway, err error) {
	result = &v1.Gateway{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("gateways").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
