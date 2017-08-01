// Copyright 2017 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package crd

import (
	"errors"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

const crdYaml = `
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: adapters.config.istio.io
spec:
  group: config.istio.io
  version: v1alpha2
  names:
    plural: adapters
    singular: adapter
    kind: Adapter
---
# maybe split it to multiple kinds (i.e. metrics / logs / quotas / monitored resources / principals?)
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: descriptors.config.istio.io
spec:
  group: config.istio.io
  version: v1alpha2
  names:
    plural: descriptors
    singular: descriptor
    kind: Descriptor
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: constructors.config.istio.io
spec:
  group: config.istio.io
  version: v1alpha2
  names:
    plural: constructors
    singular: constructor
    kind: Constructor
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: handlers.config.istio.io
spec:
  group: config.istio.io
  version: v1alpha2
  names:
    plural: handlers
    singular: handler
    kind: Handler
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: actions.config.istio.io
spec:
  group: config.istio.io
  version: v1alpha2
  names:
    plural: actions
    singular: action
    kind: Action
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: rules.config.istio.io
spec:
  group: config.istio.io
  version: v1alpha2
  names:
    plural: rules
    singular: rule
    kind: Rule
`

func crdResources() map[string]*unstructured.Unstructured {
	resources := map[string]*unstructured.Unstructured{}
	for _, y := range strings.Split(crdYaml, "\n---\n") {
		uns := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(y), &uns.Object); err != nil {
			glog.Errorf("Failed to unmarshal: %v", err)
			continue
		}
		name := uns.Object["metadata"].(map[string]interface{})["name"].(string)
		resources[name] = uns
	}
	return resources
}

func getCrdClient(discovery *discovery.DiscoveryClient, config *rest.Config) (*dynamic.ResourceClient, error) {
	gv := &schema.GroupVersion{Group: "apiextensions.k8s.io", Version: "v1beta1"}
	resources, err := discovery.ServerResourcesForGroupVersion(gv.String())
	if err != nil {
		return nil, err
	}
	for _, res := range resources.APIResources {
		if res.Kind == "CustomResourceDefinition" {
			config.APIPath = "/apis"
			config.GroupVersion = gv
			dynamic, err := dynamic.NewClient(config)
			if err != nil {
				return nil, err
			}
			return dynamic.Resource(&res, ""), nil
		}
	}
	return nil, errors.New("not found")
}

func declareCrds(client *dynamic.ResourceClient, crds map[string]*unstructured.Unstructured) error {
	defs, err := client.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, def := range defs.(*unstructured.UnstructuredList).Items {
		name := def.Object["metadata"].(map[string]interface{})["name"].(string)
		delete(crds, name)
	}
	if len(crds) == 0 {
		return nil
	}
	for _, crd := range crds {
		glog.V(7).Infof("Declaring %+v", crd)
		_, err := client.Create(crd)
		if err != nil {
			return err
		}
	}
	return nil
}
