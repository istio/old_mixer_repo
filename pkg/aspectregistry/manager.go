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

package aspectregistry

import (
	"errors"

	"istio.io/mixer/pkg/aspect"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/expr"
)

// NewManager Creates a new Uber Aspect manager
// provides an Execute function to act on a aspect/adapter
// in a given context
func NewManager(areg Registry) *Manager {
	// Add all manager here as new aspects are added
	mgrs := aspectManagers()
	mreg := make(map[string]aspect.Manager, len(mgrs))
	for _, mgr := range mgrs {
		mreg[mgr.Kind()] = mgr
	}
	return &Manager{
		mreg:        mreg,
		aspectCache: make(map[CacheKey]aspect.Aspect),
		areg:        areg,
	}
}

// Execute performs the aspect function based on given Cfg and AdapterCfg and attributes
// returns aspect output or error if the operation could not be performed
func (m *Manager) Execute(cfg *aspect.CombinedConfig, ctx attribute.Context, mapper expr.Evaluator) (*aspect.Output, error) {
	var mgr aspect.Manager
	var found bool

	if mgr, found = m.mreg[cfg.Aspect.Kind]; !found {
		return nil, errors.New("Could not find Mgr " + cfg.Aspect.Kind)
	}

	var adapter aspect.Adapter
	if adapter, found = m.areg.ByImpl(cfg.Adapter.Impl); !found {
		return nil, errors.New("Could not find registered adapter " + cfg.Adapter.Impl)
	}

	var asp aspect.Aspect
	var err error
	if asp, err = m.CacheGet(cfg, mgr, adapter); err != nil {
		return nil, err
	}
	// TODO act on aspect.Output
	return mgr.Execute(cfg, asp, ctx, mapper)
}

// CacheGet -- get from the cache, use aspect.Manager to construct an object in case of a cache miss
func (m *Manager) CacheGet(cfg *aspect.CombinedConfig, mgr aspect.Manager, adapter aspect.Adapter) (asp aspect.Aspect, err error) {
	key := cacheKey(cfg)
	// try fast path with read lock
	m.lock.RLock()
	asp, found := m.aspectCache[key]
	m.lock.RUnlock()
	if found {
		return asp, nil
	}
	// obtain write lock
	m.lock.Lock()
	defer m.lock.Unlock()
	asp, found = m.aspectCache[key]
	if !found {
		asp, err = mgr.NewAspect(cfg, adapter)
		if err != nil {
			return nil, err
		}
		m.aspectCache[key] = asp
	}
	return asp, nil
}
