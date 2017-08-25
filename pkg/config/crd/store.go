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

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

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

	// initWaiterDelay is the delay to close donec in initWaiter after
	// the List() is called initially.
	initWaiterDelay = time.Millisecond * 100

	// crdRetryTimeout is the default timeout duration to retry initialization
	// of the caches when some CRDs are missing. The timeout can be customized
	// through "retry-timeout" query parameter in the config URL,
	// like k8s://?retry-timeout=1m
	crdRetryTimeout = time.Second * 30
)

type listerWatcherBuilderInterface interface {
	build(res metav1.APIResource) cache.ListerWatcher
}

type initWaiter struct {
	builder listerWatcherBuilderInterface
	donec   chan struct{}
	calledc chan string

	mu    sync.Mutex
	kinds map[string]bool
}

func wrapListerWatcherBuilder(lwb listerWatcherBuilderInterface) *initWaiter {
	iw := &initWaiter{
		builder: lwb,
		donec:   make(chan struct{}),
		calledc: make(chan string),
		kinds:   map[string]bool{},
	}
	go iw.run()
	return iw
}

func (iw *initWaiter) run() {
	for k := range iw.calledc {
		iw.mu.Lock()
		delete(iw.kinds, k)
		l := len(iw.kinds)
		iw.mu.Unlock()
		if l == 0 {
			break
		}
	}
	// Still, there's a slight delay between the initial List() call and
	// the data to be settled into the caches, here's an ad-hoc short delay
	// before iw.donec is closed.
	time.Sleep(initWaiterDelay)
	close(iw.donec)
}

func (iw *initWaiter) build(res metav1.APIResource) cache.ListerWatcher {
	iw.mu.Lock()
	iw.kinds[res.Kind] = true
	iw.mu.Unlock()
	lw := iw.builder.build(res)
	wrapped := &cache.ListWatch{WatchFunc: lw.Watch}
	wrapped.ListFunc = func(opts metav1.ListOptions) (runtime.Object, error) {
		obj, err := lw.List(opts)
		wrapped.ListFunc = lw.List
		iw.calledc <- res.Kind
		return obj, err
	}
	return wrapped
}

type contextCh struct {
	ctx context.Context
	ch  chan store.BackendEvent
}

// Store offers store.Store2Backend interface through kubernetes custom resource definitions.
type Store struct {
	conf         *rest.Config
	ns           map[string]bool
	caches       map[string]cache.Store
	retryTimeout time.Duration

	mu  sync.Mutex
	chs []*contextCh

	// They are used to inject testing interfaces.
	discoveryBuilder     func(conf *rest.Config) (discovery.DiscoveryInterface, error)
	listerWatcherBuilder func(conf *rest.Config) (listerWatcherBuilderInterface, error)
}

var _ store.Store2Backend = &Store{}

// Init implements store.Store2Backend interface.
func (s *Store) Init(ctx context.Context, kinds []string) error {
	scheme := runtime.NewScheme()
	kindsSet := map[string]bool{}
	for _, kind := range kinds {
		scheme.AddKnownTypeWithName(
			schema.GroupVersionKind{Group: apiGroup, Version: apiVersion, Kind: kind},
			&resource{},
		)
		scheme.AddKnownTypeWithName(
			schema.GroupVersionKind{Group: apiGroup, Version: apiVersion, Kind: kind + "List"},
			&resourceList{},
		)
		kindsSet[kind] = true
	}
	s.conf.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: serializer.NewCodecFactory(scheme)}
	d, err := s.discoveryBuilder(s.conf)
	if err != nil {
		return err
	}
	lwBuilder, err := s.listerWatcherBuilder(s.conf)
	if err != nil {
		return err
	}
	waiter := wrapListerWatcherBuilder(lwBuilder)
	s.caches = make(map[string]cache.Store, len(kinds))
	crdCtx, cancel := context.WithTimeout(ctx, s.retryTimeout)
	defer cancel()
	retry := false
	for len(s.caches) < len(kinds) {
		if crdCtx.Err() != nil {
			// TODO: runs goroutines for remaining kinds.
			break
		}
		if retry && bool(glog.V(3)) {
			glog.Infof("Retrying to fetch config...")
		}
		resources, err := d.ServerResourcesForGroupVersion(apiGroupVersion)
		if err != nil {
			glog.V(3).Infof("Failed to obtain resources for CRD: %v", err)
			continue
		}
		for _, res := range resources.APIResources {
			if _, ok := s.caches[res.Kind]; ok {
				continue
			}
			if _, ok := kindsSet[res.Kind]; ok {
				cl := waiter.build(res)
				informer := cache.NewSharedInformer(cl, nil, defaultResyncPeriod)
				s.caches[res.Kind] = informer.GetStore()
				informer.AddEventHandler(s)
				go informer.Run(ctx.Done())
			}
		}
	}
	if len(s.caches) > 0 {
		<-waiter.donec
	}
	return nil
}

func (s *Store) closeWatch(ctx context.Context, ch chan store.BackendEvent) {
	<-ctx.Done()
	s.mu.Lock()
	for i, c := range s.chs {
		if ch == c.ch {
			s.chs = append(s.chs[:i], s.chs[i+1:]...)
		}
	}
	s.mu.Unlock()
}

// Watch implements store.Store2Backend interface.
func (s *Store) Watch(ctx context.Context) (<-chan store.BackendEvent, error) {
	ch := make(chan store.BackendEvent)
	s.mu.Lock()
	s.chs = append(s.chs, &contextCh{ctx, ch})
	s.mu.Unlock()
	go s.closeWatch(ctx, ch)
	return ch, nil
}

// Get implements store.Store2Backend interface.
func (s *Store) Get(key store.Key) (map[string]interface{}, error) {
	c, ok := s.caches[key.Kind]
	if !ok {
		return nil, store.ErrNotFound
	}
	obj, exists, err := c.Get(&resource{ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name}})
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, store.ErrNotFound
	}
	r, ok := obj.(*resource)
	if !ok {
		return nil, fmt.Errorf("unrecognized response")
	}
	return r.Spec, nil
}

// List implements store.Store2Backend interface.
func (s *Store) List() map[store.Key]map[string]interface{} {
	result := map[store.Key]map[string]interface{}{}
	for kind, c := range s.caches {
		for _, obj := range c.List() {
			if res, ok := obj.(*resource); ok {
				if s.ns == nil || s.ns[res.Namespace] {
					key := store.Key{Kind: kind, Name: res.Name, Namespace: res.Namespace}
					result[key] = res.Spec
				}
			} else {
				glog.Errorf("Unrecognized object %+v", obj)
			}
		}
	}
	return result
}

func toEvent(t store.ChangeType, obj interface{}) (store.BackendEvent, error) {
	r, ok := obj.(*resource)
	if !ok {
		return store.BackendEvent{}, fmt.Errorf("unrecognized data %+v", obj)
	}
	return store.BackendEvent{
		Type:  t,
		Key:   store.Key{Kind: r.Kind, Namespace: r.Namespace, Name: r.Name},
		Value: r.Spec,
	}, nil
}

func (s *Store) dispatch(ev store.BackendEvent) {
	for _, ch := range s.chs {
		select {
		case <-ch.ctx.Done():
		case ch.ch <- ev:
		}
	}
}

// OnAdd implements cache.ResourceEventHandler interface.
func (s *Store) OnAdd(obj interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ev, err := toEvent(store.Update, obj); err != nil {
		glog.Errorf("Failed to process event: %v", err)
	} else {
		s.dispatch(ev)
	}
}

// OnUpdate implements cache.ResourceEventHandler interface.
func (s *Store) OnUpdate(oldObj, newObj interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ev, err := toEvent(store.Update, newObj); err != nil {
		glog.Errorf("Failed to process event: %v", err)
	} else {
		s.dispatch(ev)
	}
}

// OnDelete implements cache.ResourceEventHandler interface.
func (s *Store) OnDelete(obj interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ev, err := toEvent(store.Delete, obj); err != nil {
		glog.Errorf("Failed to process event: %v", err)
	} else {
		ev.Value = nil
		s.dispatch(ev)
	}
}
