/*
 * Copyright (c) 2024, Intel Corporation.  All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1alpha1 "github.com/intel/intel-resource-drivers-for-kubernetes/pkg/intel.com/resource/gaudi/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeGaudiClassParameters implements GaudiClassParametersInterface
type FakeGaudiClassParameters struct {
	Fake *FakeGaudiV1alpha1
}

var gaudiclassparametersResource = v1alpha1.SchemeGroupVersion.WithResource("gaudiclassparameters")

var gaudiclassparametersKind = v1alpha1.SchemeGroupVersion.WithKind("GaudiClassParameters")

// Get takes name of the gaudiClassParameters, and returns the corresponding gaudiClassParameters object, and an error if there is any.
func (c *FakeGaudiClassParameters) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.GaudiClassParameters, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(gaudiclassparametersResource, name), &v1alpha1.GaudiClassParameters{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.GaudiClassParameters), err
}

// List takes label and field selectors, and returns the list of GaudiClassParameters that match those selectors.
func (c *FakeGaudiClassParameters) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.GaudiClassParametersList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(gaudiclassparametersResource, gaudiclassparametersKind, opts), &v1alpha1.GaudiClassParametersList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.GaudiClassParametersList{ListMeta: obj.(*v1alpha1.GaudiClassParametersList).ListMeta}
	for _, item := range obj.(*v1alpha1.GaudiClassParametersList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested gaudiClassParameters.
func (c *FakeGaudiClassParameters) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(gaudiclassparametersResource, opts))
}

// Create takes the representation of a gaudiClassParameters and creates it.  Returns the server's representation of the gaudiClassParameters, and an error, if there is any.
func (c *FakeGaudiClassParameters) Create(ctx context.Context, gaudiClassParameters *v1alpha1.GaudiClassParameters, opts v1.CreateOptions) (result *v1alpha1.GaudiClassParameters, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(gaudiclassparametersResource, gaudiClassParameters), &v1alpha1.GaudiClassParameters{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.GaudiClassParameters), err
}

// Update takes the representation of a gaudiClassParameters and updates it. Returns the server's representation of the gaudiClassParameters, and an error, if there is any.
func (c *FakeGaudiClassParameters) Update(ctx context.Context, gaudiClassParameters *v1alpha1.GaudiClassParameters, opts v1.UpdateOptions) (result *v1alpha1.GaudiClassParameters, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(gaudiclassparametersResource, gaudiClassParameters), &v1alpha1.GaudiClassParameters{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.GaudiClassParameters), err
}

// Delete takes name of the gaudiClassParameters and deletes it. Returns an error if one occurs.
func (c *FakeGaudiClassParameters) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(gaudiclassparametersResource, name, opts), &v1alpha1.GaudiClassParameters{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeGaudiClassParameters) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(gaudiclassparametersResource, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.GaudiClassParametersList{})
	return err
}

// Patch applies the patch and returns the patched gaudiClassParameters.
func (c *FakeGaudiClassParameters) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.GaudiClassParameters, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(gaudiclassparametersResource, name, pt, data, subresources...), &v1alpha1.GaudiClassParameters{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.GaudiClassParameters), err
}
