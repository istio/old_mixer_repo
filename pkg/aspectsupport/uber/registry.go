// Copyright 2017 Google Inc.
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

package uber

import (
	"sync"

	"istio.io/mixer/pkg/aspect"
	"istio.io/mixer/pkg/aspect/denyChecker"
	"istio.io/mixer/pkg/aspect/listChecker"
	alogger "istio.io/mixer/pkg/aspect/logger"
	"istio.io/mixer/pkg/aspect/quota"
)

// Registry is a simple implementation of pkg/registry.Registrar and pkg/aspectsupport/uber.RegistryQuerier which requires
// that all registered adapters have a unique adapter name.
type Registry struct {
	// Guards adaptersByName
	lock           sync.Mutex
	adaptersByName map[string]aspect.Adapter
}

// NewRegistry returns a registry whose implementation requires that all adapters have a globally unique name
// (not just unique per aspect). Registering two adapters with the same name results in the first registered adapter
// being replaced by the second.
func NewRegistry() *Registry {
	return &Registry{adaptersByName: make(map[string]aspect.Adapter)}
}

// ByImpl returns the implementation with aspect.Adapter.Name() == adapterName.
func (r *Registry) ByImpl(adapterName string) (aspect.Adapter, bool) {
	r.lock.Lock()
	defer r.lock.Unlock()
	adapter, ok := r.adaptersByName[adapterName]
	return adapter, ok // yet `return r.adaptersByName[adapterName]` doesn't typecheck.
}

// RegisterCheckList registers adapters implementing the listChecker aspect.
func (r *Registry) RegisterCheckList(list listChecker.Adapter) error {
	r.insert(list)
	return nil
}

// RegisterDeny registers adapters implementing the denyChecker aspect.
func (r *Registry) RegisterDeny(deny denyChecker.Adapter) error {
	r.insert(deny)
	return nil
}

// RegisterLogger registers adapters implementing the logger aspect.
func (r *Registry) RegisterLogger(logger alogger.Adapter) error {
	r.insert(logger)
	return nil
}

// RegisterQuota registers adapters implementing the quota aspect.
func (r *Registry) RegisterQuota(quota quota.Adapter) error {
	r.insert(quota)
	return nil
}

func (r *Registry) insert(a aspect.Adapter) {
	r.lock.Lock()
	r.adaptersByName[a.Name()] = a
	r.lock.Unlock()
}
