/*
Copyright 2021 Kong, Inc.

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
	"context"

	v1alpha1 "github.com/kong/kubernetes-ingress-controller/v2/pkg/apis/configuration/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeIngressClassParameterses implements IngressClassParametersInterface
type FakeIngressClassParameterses struct {
	Fake *FakeConfigurationV1alpha1
	ns   string
}

var ingressclassparametersesResource = v1alpha1.SchemeGroupVersion.WithResource("ingressclassparameterses")

var ingressclassparametersesKind = v1alpha1.SchemeGroupVersion.WithKind("IngressClassParameters")

// Get takes name of the ingressClassParameters, and returns the corresponding ingressClassParameters object, and an error if there is any.
func (c *FakeIngressClassParameterses) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.IngressClassParameters, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(ingressclassparametersesResource, c.ns, name), &v1alpha1.IngressClassParameters{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.IngressClassParameters), err
}

// List takes label and field selectors, and returns the list of IngressClassParameterses that match those selectors.
func (c *FakeIngressClassParameterses) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.IngressClassParametersList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(ingressclassparametersesResource, ingressclassparametersesKind, c.ns, opts), &v1alpha1.IngressClassParametersList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.IngressClassParametersList{ListMeta: obj.(*v1alpha1.IngressClassParametersList).ListMeta}
	for _, item := range obj.(*v1alpha1.IngressClassParametersList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested ingressClassParameterses.
func (c *FakeIngressClassParameterses) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(ingressclassparametersesResource, c.ns, opts))

}

// Create takes the representation of a ingressClassParameters and creates it.  Returns the server's representation of the ingressClassParameters, and an error, if there is any.
func (c *FakeIngressClassParameterses) Create(ctx context.Context, ingressClassParameters *v1alpha1.IngressClassParameters, opts v1.CreateOptions) (result *v1alpha1.IngressClassParameters, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(ingressclassparametersesResource, c.ns, ingressClassParameters), &v1alpha1.IngressClassParameters{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.IngressClassParameters), err
}

// Update takes the representation of a ingressClassParameters and updates it. Returns the server's representation of the ingressClassParameters, and an error, if there is any.
func (c *FakeIngressClassParameterses) Update(ctx context.Context, ingressClassParameters *v1alpha1.IngressClassParameters, opts v1.UpdateOptions) (result *v1alpha1.IngressClassParameters, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(ingressclassparametersesResource, c.ns, ingressClassParameters), &v1alpha1.IngressClassParameters{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.IngressClassParameters), err
}

// Delete takes name of the ingressClassParameters and deletes it. Returns an error if one occurs.
func (c *FakeIngressClassParameterses) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(ingressclassparametersesResource, c.ns, name, opts), &v1alpha1.IngressClassParameters{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeIngressClassParameterses) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(ingressclassparametersesResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.IngressClassParametersList{})
	return err
}

// Patch applies the patch and returns the patched ingressClassParameters.
func (c *FakeIngressClassParameterses) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.IngressClassParameters, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(ingressclassparametersesResource, c.ns, name, pt, data, subresources...), &v1alpha1.IngressClassParameters{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.IngressClassParameters), err
}
