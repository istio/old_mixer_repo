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

// Package crd provides the implementation of store interfaces to
// use kubernetes custom resource definitions (CRDs).
//
// Note that this is a transient approach. After all of the tasks are done,
// this package will be replaced by more directly connected to the Istio's
// config model.
package crd

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // for k8s config for GKE
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"istio.io/mixer/pkg/config/store"
)

type resource struct {
	version string
	spec    interface{}
}

// Mapping from the resource name to the resource spec.
type resourceMap map[string]*resource

type watcher struct {
	namespace string
	stoppers  []func()

	mu       sync.Mutex
	data     map[string]string
	listener store.Listener

	// kind to resourceMap
	resources map[string]resourceMap
}

var _ store.ChangeNotifier = &watcher{}

// newStore creates a new watcher instance with watching resources.
func newStore(u *url.URL) (store.KeyValueStore, error) {
	namespace := u.Host
	if namespace == "" {
		namespace = v1.NamespaceDefault
	}
	configFilePath := u.Path
	if configFilePath == "" {
		configFilePath = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}
	config, err := clientcmd.BuildConfigFromFlags("", configFilePath)
	if err != nil {
		return nil, err
	}

	w := &watcher{
		data:      map[string]string{},
		namespace: namespace,
		resources: map[string]resourceMap{},
	}
	if err = w.init(config); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *watcher) String() string {
	return fmt.Sprintf("k8s: %s", w.namespace)
}

// Get implements a KeyValueStore method.
func (w *watcher) Get(key string) (string, int, bool) {
	w.mu.Lock()
	value, found := w.data[key]
	w.mu.Unlock()
	return value, store.IndexNotSupported, found
}

// Set implements a KeyValueStore method.
func (w *watcher) Set(key, value string) (int, error) {
	return store.IndexNotSupported, errors.New("not implemented")
}

// List implements a KeyValueStore method.
func (w *watcher) List(key string, recurse bool) ([]string, int, error) {
	w.mu.Lock()
	if !strings.HasSuffix(key, "/") {
		key = key + "/"
	}
	keys := []string{}
	for k := range w.data {
		if strings.HasPrefix(k, key) {
			keys = append(keys, k)
		}
	}
	w.mu.Unlock()
	return keys, store.IndexNotSupported, nil
}

// Delete implements a KeyValueStore method.
func (w *watcher) Delete(key string) error {
	return errors.New("not implemented")
}

func unstructuredToResource(uns *unstructured.Unstructured) (string, *resource) {
	md := uns.Object["metadata"].(map[string]interface{})
	name := md["name"].(string)
	version := md["resourceVersion"].(string)
	return name, &resource{version, uns.Object["spec"]}
}

func (w *watcher) appendAndEncode(data resourceMap, field, key string) {
	if len(data) == 0 {
		return
	}
	res := make([]interface{}, 0, len(data))
	for _, d := range data {
		res = append(res, d.spec)
	}
	enc, err := yaml.Marshal(map[string]interface{}{field: res})
	if err != nil {
		glog.Errorf("Failed to marshal: %v", err)
		return
	}
	w.data[key] = string(enc)
}

func (w *watcher) encodeData(data resourceMap, field string) {
	for name, d := range data {
		key := "/scopes/global/subjects/" + name + "/" + field
		enc, err := yaml.Marshal(map[string]interface{}{field: d.spec})
		if err != nil {
			glog.Errorf("Failed to marshal: %v", err)
			return
		}
		w.data[key] = string(enc)
	}
}

func (w *watcher) buildDataAndNotify() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.appendAndEncode(w.resources["Adapter"], "adapters", "/scopes/global/adapters")
	w.appendAndEncode(w.resources["Handler"], "handlers", "/scopes/global/handlers")
	if desc, ok := w.resources["Descriptor"]["global"]; ok {
		enc, err := yaml.Marshal(desc.spec)
		if err != nil {
			return
		}
		w.data["/scopes/global/descriptors"] = string(enc)
	}
	w.encodeData(w.resources["Constructor"], "constructors")
	w.encodeData(w.resources["Rule"], "rules")
	w.encodeData(w.resources["Action"], "action_rules")
	if w.listener != nil {
		w.listener.NotifyStoreChanged(store.IndexNotSupported)
	}
}

func (w *watcher) runWatch(wi watch.Interface, target resourceMap) {
	defer wi.Stop()
	for ev := range wi.ResultChan() {
		if ev.Type == watch.Error {
			glog.Errorf("watch error: %+v", ev.Object)
			continue
		}
		name, d := unstructuredToResource(ev.Object.(*unstructured.Unstructured))
		if ev.Type == watch.Deleted {
			delete(target, name)
		} else if prev, found := target[name]; !found || prev.version != d.version {
			target[name] = d
		} else {
			continue
		}
		w.buildDataAndNotify()
	}
}

func (w *watcher) init(config *rest.Config) error {
	discovery, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return err
	}
	config.APIPath = "/apis"
	config.GroupVersion = &schema.GroupVersion{Group: "config.istio.io", Version: "v1alpha2"}
	dynamic, err := dynamic.NewClient(config)
	if err != nil {
		return err
	}

	list, err := discovery.ServerResourcesForGroupVersion("config.istio.io/v1alpha2")
	if err != nil {
		return err
	}
	for _, k := range []string{"Adapter", "Descriptor", "Constructor", "Handler", "Rule", "Action"} {
		w.resources[k] = resourceMap{}
	}
	for _, res := range list.APIResources {
		if rmap, ok := w.resources[res.Kind]; ok {
			rclient := dynamic.Resource(&res, w.namespace)
			rdata, err := rclient.List(metav1.ListOptions{})
			if err != nil {
				return err
			}
			for _, d := range rdata.(*unstructured.UnstructuredList).Items {
				name, rd := unstructuredToResource(&d)
				rmap[name] = rd
			}

			wi, err := dynamic.Resource(&res, w.namespace).Watch(metav1.ListOptions{})
			if err != nil {
				return err
			}
			w.stoppers = append(w.stoppers, wi.Stop)
			go w.runWatch(wi, rmap)
		}
	}
	w.buildDataAndNotify()
	return nil
}

// Close implements a KeyValueStore method.
func (w *watcher) Close() {
	w.mu.Lock()
	w.listener = nil
	w.mu.Unlock()
	for _, s := range w.stoppers {
		s()
	}
}

// RegisterListener implements a ChangeNotifier method.
func (w *watcher) RegisterListener(l store.Listener) {
	w.mu.Lock()
	w.listener = l
	w.mu.Unlock()
}

// Register registers this module as a config store.
// Do not use 'init()' for automatic registration; linker will drop
// the whole module because it looks unused.
func Register(m map[string]store.Builder) {
	m["k8s"] = newStore
}
