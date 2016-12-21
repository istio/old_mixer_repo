// Copyright 2016 Google Inc.
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

package registry

import (
	"errors"
	"sync"

	"istio.io/mixer/aspect"
	"istio.io/mixer/aspect/listChecker"
	"istio.io/mixer/pkg/attribute"
)

// Registry -- Interface used by adapters to register themselves
type Registry interface {
	// RegisterCheckList
	RegisterCheckList(b listChecker.Adapter) error

	// ByImpl gets an adapter by impl name
	ByImpl(impl string) (adapter aspect.Adapter, found bool)
}

// Manager manages all aspects - provides uniform interface to
// all aspect managers
type Manager struct {
	mreg map[string]aspect.Manager
	areg Registry

	// protects cache
	lock    sync.RWMutex
	aspects map[CacheKey]aspect.Aspect
}

// CacheKey is used to cache fully constructed aspects
// These parameters are used in constructing an aspect
type CacheKey struct {
	Kind   string
	Impl   string
	Params interface{}
	Args   interface{}
}

func cacheKey(cfg *aspect.Config) CacheKey {
	return CacheKey{
		Kind:   cfg.Aspect.GetKind(),
		Impl:   cfg.Adapter.GetImpl(),
		Params: cfg.Aspect.GetParams(),
		Args:   cfg.Adapter.GetArgs(),
	}
}

// NewManager Creates a new Uber Aspect manager
// provides an Execute function to act on a aspect/adapter
// in a given context
func NewManager(areg Registry) *Manager {
	// Add all manager here as new aspects are added
	mgrs := []aspect.Manager{
		listChecker.Manager(),
	}
	mreg := make(map[string]aspect.Manager, len(mgrs))
	for _, mgr := range mgrs {
		mreg[mgr.Kind()] = mgr
	}
	return &Manager{
		mreg:    mreg,
		aspects: make(map[CacheKey]aspect.Aspect),
		areg:    areg,
	}
}

// Execute performs the aspect function based on given Cfg and AdapterCfg and attributes
// returns aspect output or error if the operation could not be performed
func (m *Manager) Execute(cfg *aspect.Config, ctx attribute.Context) (*aspect.Output, error) {
	mgr, found := m.mreg[cfg.Aspect.Kind]
	if !found {
		return nil, errors.New("Could not find Mgr " + cfg.Aspect.Kind)
	}

	adapter, found := m.areg.ByImpl(cfg.Adapter.Impl)
	if !found {
		return nil, errors.New("Could not find registered adapter " + cfg.Adapter.Impl)
	}

	asp, err := m.CacheGet(cfg, mgr, adapter)
	if err != nil {
		return nil, err
	}
	// TODO act on aspect.Output and mutate attribute.Context
	return mgr.Execute(cfg, ctx, asp)
}

// CacheGet -- get from the cache, use BuilderClosure to construct an object in case of a cache miss
func (m *Manager) CacheGet(cfg *aspect.Config, mgr aspect.Manager, adapter aspect.Adapter) (asp aspect.Aspect, err error) {
	key := cacheKey(cfg)
	// try fast path with read lock
	m.lock.RLock()
	asp, found := m.aspects[key]
	m.lock.RUnlock()
	if found {
		return asp, nil
	}
	// obtain write lock
	m.lock.Lock()
	defer m.lock.Unlock()
	asp, found = m.aspects[key]
	if !found {
		asp, err = mgr.NewAspect(cfg, adapter)
		if err != nil {
			return nil, err
		}
		m.aspects[key] = asp
	}
	return asp, nil
}
