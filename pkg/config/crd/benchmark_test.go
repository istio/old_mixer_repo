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
	"context"
	"encoding/base32"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/url"
	"strings"
	"testing"
	"time"

	"istio.io/mixer/pkg/config/store"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// The number of CRDs to be used in the test.
const dummyCRDs = 20

// The number of namespaces which this store should refer to.
const dummyNS = 2

// The number of total namespaces which exist in a benchmark.
const totalNS = dummyNS * 5

// The number of dummy resources in a namespace.
const resourceCount = 10

var configPath = flag.String("config-path", "",
	"The path to kubernetes config file. If omitted, it will skip the benchmark for connecting with actual k8s cluster.")

// createDummySpec creates a spec data with s elements. Both keys and
// values are random strings.
func createDummySpec(s int) (map[string]interface{}, error) {
	spec := make(map[string]interface{}, s)
	buf := make([]byte, 5)
	for i := 0; i < s; i++ {
		_, err := rand.Read(buf)
		if err != nil {
			return nil, err
		}
		k := base32.StdEncoding.EncodeToString(buf)
		_, err = rand.Read(buf)
		if err != nil {
			return nil, err
		}
		spec[k] = base32.StdEncoding.EncodeToString(buf)
	}
	return spec, nil
}

// createDummyStrings creates a slice of random names with 'dummy-' prefix. All of the names
// end with 'e' so that they can be pluralized by simply adding 's' at the end.
func createDummyStrings(count int) ([]string, error) {
	strs := make([]string, count)
	buf := make([]byte, 5)
	for i := 0; i < count; i++ {
		_, err := rand.Read(buf)
		if err != nil {
			return nil, err
		}
		strs[i] = fmt.Sprintf("dummy-%se", strings.ToLower(base32.StdEncoding.EncodeToString(buf)))
	}
	return strs, nil
}

// benchmarkEnv abstracts the environment where the benchmark runs.
type benchmarkEnv interface {
	prepareNamespaces(namespaces []string) error
	prepareCRDs(kinds []string) error
	create(key store.Key, spec map[string]interface{}) error
	update(key store.Key, spec map[string]interface{}) error
	newStore() *Store
	cleanup()
}

// dummyBenchmarkEnv is the environment with dummy instance.
type dummyBenchmarkEnv struct {
	discovery  *fake.FakeDiscovery
	lw         *dummyListerWatcherBuilder
	namespaces map[string]bool
	kinds      []string
}

func newDummyEnv() *dummyBenchmarkEnv {
	return &dummyBenchmarkEnv{
		lw: &dummyListerWatcherBuilder{
			data:     map[store.Key]*unstructured.Unstructured{},
			watchers: map[string]*watch.RaceFreeFakeWatcher{},
		},
		namespaces: map[string]bool{},
	}
}

func (e *dummyBenchmarkEnv) discoveryBuilder(*rest.Config) (discovery.DiscoveryInterface, error) {
	return e.discovery, nil
}

func (e *dummyBenchmarkEnv) listerWatcherBuilder(*rest.Config) (listerWatcherBuilderInterface, error) {
	return e.lw, nil
}

func (e *dummyBenchmarkEnv) prepareNamespaces(namespaces []string) error {
	for _, ns := range namespaces[:dummyNS] {
		e.namespaces[ns] = true
	}
	return nil
}

func (e *dummyBenchmarkEnv) prepareCRDs(kinds []string) error {
	e.kinds = kinds
	e.discovery = fakeDiscovery(kinds)
	return nil
}

func (e *dummyBenchmarkEnv) create(key store.Key, spec map[string]interface{}) error {
	return e.lw.put(key, spec)
}

func (e *dummyBenchmarkEnv) update(key store.Key, spec map[string]interface{}) error {
	return e.lw.put(key, spec)
}

func (e *dummyBenchmarkEnv) newStore() *Store {
	return &Store{
		conf:                 &rest.Config{},
		retryTimeout:         testingRetryTimeout,
		discoveryBuilder:     e.discoveryBuilder,
		listerWatcherBuilder: e.listerWatcherBuilder,
		ns:                   e.namespaces,
	}
}

func (e *dummyBenchmarkEnv) cleanup() {
}

// k8sBenchmarkEnv is the environment which connects with an actual k8s cluster.
type k8sBenchmarkEnv struct {
	u           *url.URL
	config      *rest.Config
	dclient     *dynamic.Client
	client      *kubernetes.Clientset
	crdClient   *dynamic.ResourceClient
	createdNS   []string
	createdCRDs []string
	cache       *unstructured.Unstructured
}

func newK8sEnv(u *url.URL) (*k8sBenchmarkEnv, error) {
	config, err := clientcmd.BuildConfigFromFlags("", u.Path)
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	dconfig := *config
	dconfig.APIPath = "/apis"
	dconfig.GroupVersion = &schema.GroupVersion{
		Group:   apiGroup,
		Version: apiVersion,
	}
	dclient, err := dynamic.NewClient(&dconfig)
	if err != nil {
		return nil, err
	}
	crdConfig := *config
	crdConfig.APIPath = "/apis"
	crdConfig.GroupVersion = &schema.GroupVersion{
		Group:   "apiextensions.k8s.io",
		Version: "v1beta1",
	}
	apiExtensionsClient, err := dynamic.NewClient(&crdConfig)
	if err != nil {
		return nil, err
	}
	return &k8sBenchmarkEnv{
		u:         u,
		config:    config,
		dclient:   dclient,
		client:    client,
		crdClient: apiExtensionsClient.Resource(&metav1.APIResource{Name: "customresourcedefinitions"}, ""),
		cache:     &unstructured.Unstructured{},
	}, nil
}

func (e *k8sBenchmarkEnv) prepareNamespaces(namespaces []string) error {
	e.createdNS = make([]string, 0, len(namespaces))
	for _, ns := range namespaces {
		_, err := e.client.CoreV1().Namespaces().Create(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
			},
		})
		if err != nil {
			return err
		}
		e.createdNS = append(e.createdNS, ns)
	}
	e.u.RawQuery = "ns=" + strings.Join(namespaces[:dummyNS], ",")
	return nil
}

func (e *k8sBenchmarkEnv) prepareCRDs(kinds []string) error {
	e.createdCRDs = make([]string, 0, len(kinds))
	for _, kind := range kinds {
		crd := &unstructured.Unstructured{}
		crd.SetAPIVersion("apiextensions.k8s.io/v1beta1")
		crd.SetKind("CustomResourceDefinition")
		crd.SetName(kind + "s." + apiGroup)
		crd.Object["spec"] = map[string]interface{}{
			"group":   apiGroup,
			"version": apiVersion,
			"scope":   "Namespaced",
			"names": map[string]string{
				"kind":     kind,
				"plural":   kind + "s",
				"singular": kind,
			},
		}
		if _, err := e.crdClient.Create(crd); err != nil {
			return err
		}
		e.createdCRDs = append(e.createdCRDs, crd.GetName())
	}
	return nil
}

func (e *k8sBenchmarkEnv) buildUnstructured(key store.Key, spec map[string]interface{}) *unstructured.Unstructured {
	e.cache.SetAPIVersion(apiGroupVersion)
	e.cache.SetKind(key.Kind)
	e.cache.SetName(key.Name)
	e.cache.SetNamespace(key.Namespace)
	e.cache.Object["spec"] = spec
	return e.cache
}

func (e *k8sBenchmarkEnv) create(key store.Key, spec map[string]interface{}) error {
	client := e.dclient.Resource(&metav1.APIResource{Name: key.Kind + "s", Namespaced: true}, key.Namespace)
	_, err := client.Create(e.buildUnstructured(key, spec))
	return err
}

func (e *k8sBenchmarkEnv) update(key store.Key, spec map[string]interface{}) error {
	client := e.dclient.Resource(&metav1.APIResource{Name: key.Kind + "s", Namespaced: true}, key.Namespace)
	obj := e.buildUnstructured(key, spec)
	bytes, err := json.Marshal(obj.Object)
	if err != nil {
		return err
	}
	_, err = client.Patch(key.Name, types.MergePatchType, bytes)
	return err
}

func (e *k8sBenchmarkEnv) newStore() *Store {
	s, err := NewStore(e.u)
	if err != nil {
		panic(err)
	}
	return s.(*Store)
}

func (e *k8sBenchmarkEnv) cleanup() {
	for _, ns := range e.createdNS {
		e.client.CoreV1().Namespaces().Delete(ns, &metav1.DeleteOptions{})
	}
	for _, crd := range e.createdCRDs {
		e.crdClient.Delete(crd, &metav1.DeleteOptions{})
	}
}

func runBenchmarks(b *testing.B, env benchmarkEnv) {
	defer env.cleanup()
	rand.Seed(time.Now().Unix())
	kinds, err := createDummyStrings(dummyCRDs)
	if err != nil {
		b.Fatal(err)
	}
	namespaces, err := createDummyStrings(totalNS)
	if err != nil {
		b.Fatal(err)
	}
	if err = env.prepareNamespaces(namespaces); err != nil {
		b.Fatal(err)
	}
	if err = env.prepareCRDs(kinds); err != nil {
		b.Fatal(err)
	}
	initBench := func(bb *testing.B) {
		for i := 0; i < bb.N; i++ {
			s := env.newStore()
			ctx, cancel := context.WithCancel(context.Background())
			if err := s.Init(ctx, kinds); err != nil {
				bb.Errorf("Failed to initialize: %v", err)
			}
			cancel()
		}
	}
	b.Run("Init", initBench)

	generatedKeys := make([]store.Key, 0, len(kinds)*len(namespaces)*resourceCount)
	for _, kind := range kinds {
		for _, ns := range namespaces {
			names, err := createDummyStrings(resourceCount)
			if err != nil {
				b.Fatal(err)
			}
			for _, name := range names {
				key := store.Key{Kind: kind, Name: name, Namespace: ns}
				dummySpec, err := createDummySpec(6)
				if err != nil {
					b.Fatal(err)
				}
				if err = env.create(key, dummySpec); err != nil {
					b.Fatal(err)
				}
				generatedKeys = append(generatedKeys, key)
			}
		}
	}

	if !b.Run("InitWithValues", initBench) {
		return
	}

	keys := make([]store.Key, 0, len(kinds)*dummyNS*resourceCount)
	targetNS := make(map[string]bool, dummyNS)
	for _, ns := range namespaces[:dummyNS] {
		targetNS[ns] = true
	}
	for _, k := range generatedKeys {
		if targetNS[k.Namespace] {
			keys = append(keys, k)
		}
	}

	s := env.newStore()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Init(ctx, kinds); err != nil {
		b.Fatal(err)
	}
	b.Run("List", func(bb *testing.B) {
		for i := 0; i < bb.N; i++ {
			objs := s.List()
			for _, key := range keys {
				if _, ok := objs[key]; !ok {
					bb.Errorf("Key %s is not found", key)
				}
			}
		}
	})

	b.Run("Watch", func(bb *testing.B) {
		wch, err := s.Watch(ctx)
		if err != nil {
			bb.Fatal(err)
		}
		for i := 0; i < bb.N; i++ {
			key := keys[rand.Int()%len(keys)]
			newSpec, err := createDummySpec(6)
			if err != nil {
				bb.Errorf("Failed to create a dummy spec: %v", err)
				continue
			}
			if err = env.update(key, newSpec); err != nil {
				bb.Errorf("Failed to put: %v", err)
				continue
			}
			if err = waitFor(wch, store.Update, key); err != nil {
				bb.Errorf("Expected event does not come: %v", err)
				continue
			}
			_, err = s.Get(key)
			if err != nil {
				bb.Errorf("Not found: %v", err)
				continue
			}
		}
	})
}

func BenchmarkDummy(b *testing.B) {
	runBenchmarks(b, newDummyEnv())
}

func BenchmarkActual(b *testing.B) {
	if *configPath == "" {
		b.Skip("--config-path is not specified. Skipping.")
	}
	env, err := newK8sEnv(&url.URL{
		Scheme: "k8s",
		Path:   *configPath,
	})
	if err != nil {
		b.Fatal(err)
	}
	runBenchmarks(b, env)
}
