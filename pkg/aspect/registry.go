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

package aspect

import (
	"fmt"

	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/config"
)

// Registry registers all aspect managers along with
// their APIMethod associations.
type Registry struct {
	r  map[string]Manager
	as map[config.APIMethod]config.AspectSet
}

// ManagerFinder find an aspect Manager given aspect kind.
type ManagerFinder interface {
	// ByImpl queries the registry by adapter name.
	FindManager(kind string) (Manager, bool)
}

// newRegistry returns a fully constructed aspect registry given APIBinding.
func newRegistry(bnds []APIBinding) *Registry {
	r := make(map[string]Manager)
	as := make(map[config.APIMethod]config.AspectSet)

	// setup aspect sets for all methods
	for _, am := range config.APIMethods() {
		as[am] = config.AspectSet{}
	}

	for _, bnd := range bnds {
		r[bnd.aspect.Kind()] = bnd.aspect
		as[bnd.method][bnd.aspect.Kind()] = true
	}
	ar := &Registry{r: r, as: as}
	// ensure that we implement the correct interfaces
	var _ config.ValidatorFinder = ar
	var _ ManagerFinder = ar
	return ar

}

// AspectSet returns aspect set associated with the given api Method.
func (r *Registry) AspectSet(method config.APIMethod) config.AspectSet {
	a, ok := r.as[method]
	if !ok {
		panic(fmt.Errorf("request for invalid api method: %v", method))
	}
	return a
}

// FindManager finds Manager given an aspect kind.
func (r *Registry) FindManager(kind string) (Manager, bool) {
	m, ok := r.r[kind]
	return m, ok
}

// FindValidator finds a config validator given a name. see: config.ValidatorFinder.
func (r *Registry) FindValidator(kind string) (adapter.ConfigValidator, bool) {
	return r.FindManager(kind)
}

// APIBinding associates an aspect with an API method
type APIBinding struct {
	aspect Manager
	method config.APIMethod
}

// DefaultRegistry returns a manager registry that contains
// all available aspect managers
func DefaultRegistry() *Registry {
	// Update the following list to add a new Aspect manager
	ab := []APIBinding{
		{NewDenyCheckerManager(), config.CheckMethod},
		{NewListCheckerManager(), config.CheckMethod},
		{NewLoggerManager(), config.ReportMethod},
	}

	return newRegistry(ab)
}
