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

package adapterManager

import (
	"fmt"
	"sync"

	"bytes"
	"crypto/sha1"
	"encoding/gob"

	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/aspect"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/config"
	"istio.io/mixer/pkg/expr"
)

// BuilderFinder finds a builder given the impl name.
type BuilderFinder interface {
	// ByImpl queries the registry by adapter name.
	FindBuilder(impl string) (adapter.Builder, bool)
}

// Manager manages all aspects - provides uniform interface to
// all aspect managers
type Manager struct {
	managerFinder aspect.ManagerFinder
	builderFinder BuilderFinder
	mapper        expr.Evaluator

	// protects cache
	lock        sync.RWMutex
	aspectCache map[cacheKey]aspect.Wrapper
}

// cacheKey is used to cache fully constructed aspects
// These parameters are used in constructing an aspect
type cacheKey struct {
	Kind             string
	Impl             string
	BuilderParamsSha [sha1.Size]byte
	AspectParamsSha  [sha1.Size]byte
}

func newCacheKey(cfg *config.Combined) (*cacheKey, error) {
	ret := cacheKey{
		Kind: cfg.Aspect.GetKind(),
		Impl: cfg.Builder.GetImpl(),
	}

	//TODO pre-compute shas and store with params
	var b bytes.Buffer
	// use gob encoding so that we don't rely on proto marshal
	enc := gob.NewEncoder(&b)

	if cfg.Builder.GetParams() != nil {
		if err := enc.Encode(cfg.Builder.GetParams()); err != nil {
			return nil, err
		}
		ret.BuilderParamsSha = sha1.Sum(b.Bytes())
	}
	b.Reset()
	if cfg.Aspect.GetParams() != nil {
		if err := enc.Encode(cfg.Aspect.GetParams()); err != nil {
			return nil, err
		}

		ret.AspectParamsSha = sha1.Sum(b.Bytes())
	}

	return &ret, nil
}

// NewManager Creates a new Uber Aspect manager
func NewManager(b BuilderFinder, m aspect.ManagerFinder, exp expr.Evaluator) *Manager {
	return &Manager{
		managerFinder: m,
		builderFinder: b,
		mapper:        exp,
		aspectCache:   make(map[cacheKey]aspect.Wrapper),
	}
}

// Execute performs the aspect function based on CombinedConfig and attributes and an expression evaluator
// returns aspect output or error if the operation could not be performed
func (m *Manager) Execute(cfg *config.Combined, attrs attribute.Bag) (*aspect.Output, error) {
	var mgr aspect.Manager
	var found bool

	if mgr, found = m.managerFinder.FindManager(cfg.Aspect.Kind); !found {
		return nil, fmt.Errorf("could not find aspect manager %#v", cfg.Aspect.Kind)
	}

	var adp adapter.Builder
	if adp, found = m.builderFinder.FindBuilder(cfg.Builder.Impl); !found {
		return nil, fmt.Errorf("could not find registered adapter %#v", cfg.Builder.Impl)
	}

	var asp aspect.Wrapper
	var err error
	if asp, err = m.cacheGet(cfg, mgr, adp); err != nil {
		return nil, err
	}

	// TODO act on adapter.Output
	return asp.Execute(attrs, m.mapper)
}

// CacheGet -- get from the cache, use adapter.Manager to construct an object in case of a cache miss
func (m *Manager) cacheGet(cfg *config.Combined, mgr aspect.Manager, builder adapter.Builder) (asp aspect.Wrapper, err error) {
	var key *cacheKey
	if key, err = newCacheKey(cfg); err != nil {
		return nil, err
	}
	// try fast path with read lock
	m.lock.RLock()
	asp, found := m.aspectCache[*key]
	m.lock.RUnlock()
	if found {
		return asp, nil
	}
	// obtain write lock
	m.lock.Lock()
	defer m.lock.Unlock()
	asp, found = m.aspectCache[*key]
	if !found {
		env := newEnv(builder.Name())
		asp, err = mgr.NewAspect(cfg, builder, env)
		if err != nil {
			return nil, err
		}
		m.aspectCache[*key] = asp
	}
	return asp, nil
}
