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

package fake

import (
	navigatorv1 "github.com/alxegox/navigator/pkg/apis/navigator/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeCanaryReleases implements CanaryReleaseInterface
type FakeCanaryReleases struct {
	Fake *FakeNavigatorV1
	ns   string
}

var canaryreleasesResource = schema.GroupVersionResource{Group: "navigator.alxegox.dev", Version: "v1", Resource: "canaryreleases"}

var canaryreleasesKind = schema.GroupVersionKind{Group: "navigator.alxegox.dev", Version: "v1", Kind: "CanaryRelease"}

// Get takes name of the canaryRelease, and returns the corresponding canaryRelease object, and an error if there is any.
func (c *FakeCanaryReleases) Get(name string, options v1.GetOptions) (result *navigatorv1.CanaryRelease, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(canaryreleasesResource, c.ns, name), &navigatorv1.CanaryRelease{})

	if obj == nil {
		return nil, err
	}
	return obj.(*navigatorv1.CanaryRelease), err
}

// List takes label and field selectors, and returns the list of CanaryReleases that match those selectors.
func (c *FakeCanaryReleases) List(opts v1.ListOptions) (result *navigatorv1.CanaryReleaseList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(canaryreleasesResource, canaryreleasesKind, c.ns, opts), &navigatorv1.CanaryReleaseList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &navigatorv1.CanaryReleaseList{ListMeta: obj.(*navigatorv1.CanaryReleaseList).ListMeta}
	for _, item := range obj.(*navigatorv1.CanaryReleaseList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested canaryReleases.
func (c *FakeCanaryReleases) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(canaryreleasesResource, c.ns, opts))

}

// Create takes the representation of a canaryRelease and creates it.  Returns the server's representation of the canaryRelease, and an error, if there is any.
func (c *FakeCanaryReleases) Create(canaryRelease *navigatorv1.CanaryRelease) (result *navigatorv1.CanaryRelease, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(canaryreleasesResource, c.ns, canaryRelease), &navigatorv1.CanaryRelease{})

	if obj == nil {
		return nil, err
	}
	return obj.(*navigatorv1.CanaryRelease), err
}

// Update takes the representation of a canaryRelease and updates it. Returns the server's representation of the canaryRelease, and an error, if there is any.
func (c *FakeCanaryReleases) Update(canaryRelease *navigatorv1.CanaryRelease) (result *navigatorv1.CanaryRelease, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(canaryreleasesResource, c.ns, canaryRelease), &navigatorv1.CanaryRelease{})

	if obj == nil {
		return nil, err
	}
	return obj.(*navigatorv1.CanaryRelease), err
}

// Delete takes name of the canaryRelease and deletes it. Returns an error if one occurs.
func (c *FakeCanaryReleases) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(canaryreleasesResource, c.ns, name), &navigatorv1.CanaryRelease{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeCanaryReleases) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(canaryreleasesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &navigatorv1.CanaryReleaseList{})
	return err
}

// Patch applies the patch and returns the patched canaryRelease.
func (c *FakeCanaryReleases) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *navigatorv1.CanaryRelease, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(canaryreleasesResource, c.ns, name, pt, data, subresources...), &navigatorv1.CanaryRelease{})

	if obj == nil {
		return nil, err
	}
	return obj.(*navigatorv1.CanaryRelease), err
}