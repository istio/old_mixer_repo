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

// Package crd provides the store interface to config resources stored as
// kubernetes custom resource definitions (CRDs).
package crd

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	// import GKE cluster authentication plugin
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// import OIDC cluster authentication plugin, e.g. for Tectonic
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"istio.io/mixer/pkg/config/store"
)

const (
	// API group / version for istio config.
	apiGroup        = "config.istio.io"
	apiVersion      = "v1alpha2"
	apiGroupVersion = apiGroup + "/" + apiVersion
)

const (
	// defaultResyncPeriod is the resync period for the k8s cache.
	// TODO: allow customization.
	defaultResyncPeriod = time.Minute

	// initializationPeriod is the duration to wait for getting
	// initial data. Mixer doesn't want to get added events for
	// initial data since it may arrive in any order, and it
	// would result in temporary invalid status.
	initializationPeriod = time.Second * 2
)

type listerWatcherBuilderInterface interface {
	build(res metav1.APIResource) cache.ListerWatcher
}

// Store offers store.Store2 interface through kubernetes custom resource definitions.
type Store struct {
	conf      *rest.Config
	ns        map[string]bool
	caches    map[string]cache.Store
	kinds     map[string]proto.Message
	resources map[string]metav1.APIResource

	mu  sync.Mutex
	eqs map[string][]*eventQueue

	// They are used to inject testing interfaces.
	discoveryBuilder     func(conf *rest.Config) (discovery.DiscoveryInterface, error)
	listerWatcherBuilder listerWatcherBuilderInterface
}

var _ store.Store2 = &Store{}

func defaultDiscoveryBuilder(conf *rest.Config) (discovery.DiscoveryInterface, error) {
	client, err := discovery.NewDiscoveryClientForConfig(conf)
	return client, err
}

type dynamicListerWatcherBuilder struct {
	client *dynamic.Client
}

func (b *dynamicListerWatcherBuilder) build(res metav1.APIResource) cache.ListerWatcher {
	return b.client.Resource(&res, "")
}

// SetValidator implements store.Store2 interface.
func (s *Store) SetValidator(store.Validator) {
	glog.Errorf("Validator is not implemented yet")
}

// Init implements store.Store2 interface.
func (s *Store) Init(ctx context.Context, kinds map[string]proto.Message) error {
	scheme := runtime.NewScheme()
	for kind, spec := range kinds {
		scheme.AddKnownTypeWithName(
			schema.GroupVersionKind{Group: apiGroup, Version: apiVersion, Kind: kind},
			&resource{},
		)
		scheme.AddKnownTypeWithName(
			schema.GroupVersionKind{Group: apiGroup, Version: apiVersion, Kind: kind + "List"},
			&resourceList{},
		)
		s.kinds[kind] = spec
	}
	s.conf.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: serializer.NewCodecFactory(scheme)}
	d, err := s.discoveryBuilder(s.conf)
	if err != nil {
		return err
	}
	resources, err := d.ServerResourcesForGroupVersion(apiGroupVersion)
	if err != nil {
		return err
	}
	lwBuilder := s.listerWatcherBuilder
	if lwBuilder == nil {
		dyn, err := dynamic.NewClient(s.conf)
		if err != nil {
			return err
		}
		lwBuilder = &dynamicListerWatcherBuilder{dyn}
	}
	s.caches = make(map[string]cache.Store, len(s.kinds))
	for _, res := range resources.APIResources {
		if _, ok := s.kinds[res.Kind]; ok {
			cl := lwBuilder.build(res)
			informer := cache.NewSharedInformer(cl, nil, defaultResyncPeriod)
			s.caches[res.Kind] = informer.GetStore()
			s.resources[res.Kind] = res
			informer.AddEventHandler(s)
			go informer.Run(ctx.Done())
		}
	}
	go s.closeAllWatches(ctx)
	time.Sleep(initializationPeriod) // TODO
	return nil
}

func (s *Store) closeAllWatches(ctx context.Context) {
	<-ctx.Done()
	s.mu.Lock()
	visited := map[*eventQueue]bool{}
	for _, eqs := range s.eqs {
		for _, eq := range eqs {
			if !visited[eq] {
				eq.cancel()
				visited[eq] = true
			}
		}
	}
	s.eqs = map[string][]*eventQueue{}
	s.mu.Unlock()
}

func (s *Store) closeWatch(ctx context.Context, kinds []string, q *eventQueue) {
	<-ctx.Done()
	s.mu.Lock()
	for _, k := range kinds {
		eqs := s.eqs[k]
		for i, eq := range eqs {
			if eq == q {
				s.eqs[k] = append(eqs[:i], eqs[i+1:]...)
				break
			}
		}
	}
	s.mu.Unlock()
}

// Watch implements store.Store2 interface.
func (s *Store) Watch(ctx context.Context, kinds []string) (<-chan store.Event, error) {
	ctx, cancel := context.WithCancel(ctx)
	q := newQueue(ctx, cancel)
	s.mu.Lock()
	for _, k := range kinds {
		s.eqs[k] = append(s.eqs[k], q)
	}
	s.mu.Unlock()
	go s.closeWatch(ctx, kinds, q)
	return q.chout, nil
}

// Get implements store.Store2 interface.
func (s *Store) Get(key store.Key, spec proto.Message) error {
	c, ok := s.caches[key.Kind]
	if !ok {
		return store.ErrNotFound
	}
	obj, exists, err := c.Get(&resource{ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name}})
	if err != nil {
		return err
	}
	if !exists {
		return store.ErrNotFound
	}
	// maybe check the obj's type and spec's type?
	return convert(obj.(*resource).Spec, spec)
}

// List implements store.Store2 interface.
func (s *Store) List() map[store.Key]proto.Message {
	result := map[store.Key]proto.Message{}
	for kind, c := range s.caches {
		tmpl, ok := s.kinds[kind]
		if !ok {
			glog.Warningf("Kind %s is not registered", kind)
			continue
		}
		for _, obj := range c.List() {
			if res, ok := obj.(*resource); ok {
				if s.ns[res.Namespace] {
					key := store.Key{Kind: kind, Name: res.Name, Namespace: res.Namespace}
					value := proto.Clone(tmpl)
					if err := convert(res.Spec, value); err != nil {
						glog.Errorf("Failed to convert: %v", err)
						continue
					}
					result[key] = value
				}
			} else {
				glog.Errorf("Unrecognized object %+v", obj)
			}
		}
	}
	return result
}

func (s *Store) toEvent(t store.ChangeType, obj interface{}) (store.Event, error) {
	r, ok := obj.(*resource)
	if !ok {
		return store.Event{}, fmt.Errorf("unrecognized data %+v", obj)
	}
	pbSpec, ok := s.kinds[r.Kind]
	if !ok {
		return store.Event{}, fmt.Errorf("unrecognized kind %s", r.Kind)
	}
	ev := store.Event{Type: t, Key: store.Key{Kind: r.Kind, Namespace: r.Namespace, Name: r.Name}}
	if t == store.Update {
		ev.Value = proto.Clone(pbSpec)
		if err := convert(r.Spec, ev.Value); err != nil {
			return store.Event{}, err
		}
	}
	return ev, nil
}

// OnAdd implements cache.ResourceEventHandler interface.
func (s *Store) OnAdd(obj interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.eqs) == 0 {
		return
	}
	if ev, err := s.toEvent(store.Update, obj); err != nil {
		glog.Errorf("Failed to process event: %v", err)
	} else {
		for _, eq := range s.eqs[ev.Kind] {
			eq.Send(ev)
		}
	}
}

// OnUpdate implements cache.ResourceEventHandler interface.
func (s *Store) OnUpdate(oldObj, newObj interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.eqs) == 0 {
		return
	}
	if ev, err := s.toEvent(store.Update, newObj); err != nil {
		glog.Errorf("Failed to process event: %v", err)
	} else {
		for _, eq := range s.eqs[ev.Kind] {
			eq.Send(ev)
		}
	}
}

// OnDelete implements cache.ResourceEventHandler interface.
func (s *Store) OnDelete(obj interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.eqs) == 0 {
		return
	}
	if ev, err := s.toEvent(store.Delete, obj); err != nil {
		glog.Errorf("Failed to process event: %v", err)
	} else {
		for _, eq := range s.eqs[ev.Kind] {
			eq.Send(ev)
		}
	}
}

// NewStore creates a new Store instance.
func NewStore(kubeconfig string, namespaces []string) (*Store, error) {
	conf, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	conf.APIPath = "/apis"
	conf.GroupVersion = &schema.GroupVersion{Group: apiGroup, Version: apiVersion}
	ns := map[string]bool{}
	for _, n := range namespaces {
		ns[n] = true
	}
	return &Store{
		conf:             conf,
		ns:               ns,
		kinds:            map[string]proto.Message{},
		resources:        map[string]metav1.APIResource{},
		eqs:              map[string][]*eventQueue{},
		discoveryBuilder: defaultDiscoveryBuilder,
	}, nil
}
