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

	"github.com/golang/glog"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/aspect"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/config"
	"istio.io/mixer/pkg/expr"
)

// Manager manages all aspects - provides uniform interface to
// all aspect managers
type Manager struct {
	managerFinder aspect.ManagerFinder
	mapper        expr.Evaluator
	builderFinder builderFinder

	// protects cache
	lock        sync.RWMutex
	aspectCache map[cacheKey]aspect.Wrapper
}

// builderFinder finds a builder by name.
type builderFinder interface {
	// FindBuilder finds a builder by name.
	FindBuilder(name string) (adapter.Builder, bool)
}

// cacheKey is used to cache fully constructed aspects
// These parameters are used in constructing an aspect
type cacheKey struct {
	kind             string
	impl             string
	builderParamsSHA [sha1.Size]byte
	aspectParamsSHA  [sha1.Size]byte
}

func newCacheKey(cfg *config.Combined) (*cacheKey, error) {
	ret := cacheKey{
		kind: cfg.Aspect.GetKind(),
		impl: cfg.Builder.GetImpl(),
	}

	//TODO pre-compute shas and store with params
	var b bytes.Buffer
	// use gob encoding so that we don't rely on proto marshal
	enc := gob.NewEncoder(&b)

	if cfg.Builder.GetParams() != nil {
		if err := enc.Encode(cfg.Builder.GetParams()); err != nil {
			return nil, err
		}
		ret.builderParamsSHA = sha1.Sum(b.Bytes())
	}
	b.Reset()
	if cfg.Aspect.GetParams() != nil {
		if err := enc.Encode(cfg.Aspect.GetParams()); err != nil {
			return nil, err
		}

		ret.aspectParamsSHA = sha1.Sum(b.Bytes())
	}

	return &ret, nil
}

// NewManager creates a new adapterManager.
func NewManager(builders []adapter.RegisterFn, m aspect.ManagerFinder, exp expr.Evaluator) *Manager {
	return newManager(newRegistry(builders), m, exp)
}

// newManager
func newManager(r builderFinder, m aspect.ManagerFinder, exp expr.Evaluator) *Manager {
	return &Manager{
		managerFinder: m,
		builderFinder: r,
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

	// create an aspect
	env := newEnv(builder.Name())
	asp, err = mgr.NewAspect(cfg, builder, env)
	if err != nil {
		return nil, err
	}

	// obtain write lock
	m.lock.Lock()
	// see if someone else beat you to it
	if other, found := m.aspectCache[*key]; found {
		if err1 := asp.Close(); err1 != nil {
			glog.Warningf("Error closing aspect: %v", asp)
		}
		asp = other
	} else {
		// your are the first one, save your aspect
		m.aspectCache[*key] = asp
	}

	m.lock.Unlock()

	return asp, nil
}

// FindValidator is used to find a config validator given a name. see: config.ValidatorFinder
func (m *Manager) FindValidator(name string) (adapter.ConfigValidator, bool) {
	return m.builderFinder.FindBuilder(name)
}
