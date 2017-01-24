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

package adapterManager

import (
	"fmt"

	"istio.io/mixer/pkg/adapter"
)

// Registry is a simple implementation of pkg/adapter/Registrar and pkg/aspect/uber.RegistryQuerier which requires
// that all registered adapters have a unique adapter name.
type Registry struct {
	buildersByName map[string]adapter.Builder
}

// NewRegistry returns a registry whose implementation requires that all builders have a globally unique name
// (not just unique per aspect). Registering two adapters with the same name results in a runtime panic.
func NewRegistry(builders []adapter.MustRegisterFn) *Registry {
	r := &Registry{buildersByName: make(map[string]adapter.Builder)}
	for _, builder := range builders {
		builder(r)
	}
	return r
}

// FindBuilder returns the builder with the given name.
func (r *Registry) FindBuilder(impl string) (adapter.Builder, bool) {
	b, ok := r.buildersByName[impl]
	return b, ok
}

// FindValidator is used to find a config validator given a name. see: config.ValidatorFinder
func (r *Registry) FindValidator(name string) (adapter.ConfigValidator, bool) {
	return r.FindBuilder(name)
}

// RegisterListChecker registers a new ListChecker builder.
func (r *Registry) RegisterListChecker(list adapter.ListCheckerBuilder) {
	r.insert(list)
}

// RegisterDenyChecker registers a new DenyChecker builder.
func (r *Registry) RegisterDenyChecker(deny adapter.DenyCheckerBuilder) {
	r.insert(deny)
}

// RegisterLogger registers a new Logger builder.
func (r *Registry) RegisterLogger(logger adapter.LoggerBuilder) {
	r.insert(logger)
}

// RegisterQuota registers a new Quota builder.
func (r *Registry) RegisterQuota(quota adapter.QuotaBuilder) {
	r.insert(quota)
}

func (r *Registry) insert(b adapter.Builder) {
	if _, exists := r.buildersByName[b.Name()]; exists {
		panic(fmt.Errorf("attempting to register a builder with a name already in the registry: %s", b.Name()))
	}
	r.buildersByName[b.Name()] = b
}
