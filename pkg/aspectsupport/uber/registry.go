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

// NewRegistry returns a registry whose implementation assumes that all adapters are uniquely named.
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
func (r *Registry) RegisterCheckList(a listChecker.Adapter) error {
	r.insert(a)
	return nil
}

// RegisterDeny registers adapters implementing the denyChecker aspect.
func (r *Registry) RegisterDeny(a denyChecker.Adapter) error {
	r.insert(a)
	return nil
}

// RegisterLogger informs the mixer that an implementation of the
// logging aspect is provided by the supplied adapter. This adapter
// will be used to build individual instances of the logger aspect
// according to mixer config.
func (r *Registry) RegisterLogger(a alogger.Adapter) error {
	r.insert(a)
	return nil
}

// RegisterQuota is used by adapters to register themselves as implementing the
// quota aspect.
func (r *Registry) RegisterQuota(a quota.Adapter) error {
	r.insert(a)
	return nil
}

func (r *Registry) insert(a aspect.Adapter) {
	r.lock.Lock()
	r.adaptersByName[a.Name()] = a
	r.lock.Unlock()
}
