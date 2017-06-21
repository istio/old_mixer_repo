// Copyright 2017 Istio Authors.
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

	"github.com/golang/glog"

	registrar2 "istio.io/mixer/pkg/adapter"
	adpCnfg "istio.io/mixer/pkg/adapter/config"
)

// HandlerInfo provides information about an individual handler.
type HandlerInfo struct {
	// Builder is the builder of interest.
	HandlerBuilder adpCnfg.HandlerBuilder

	// Templates specifies the the name of the templates this builder is capable of handling.
	Templates []string
}

// registry2 implements pkg/adapter/Registrar2.
// registry2 is initialized in the constructor and is immutable thereafter.
// All registered handlers must have unique names per aspect kind.
type registry2 struct {
	handlerBuildersByName map[string]*HandlerInfo
}

// newRegistry returns a new Builder registry.
func newRegistry2(builders []registrar2.RegisterFn2) *registry2 {
	r := &registry2{make(map[string]*HandlerInfo)}
	for idx, builder := range builders {
		glog.V(3).Infof("Registering [%d] %#v", idx, builder)
		builder(r)
	}

	// ensure interfaces are satisfied.
	// should be compiled out.
	var _ registrar2.Registrar2 = r
	return r
}

// HandlerMap returns the known handlers, indexed by their names.
func HandlerMap(handlerRegFns []registrar2.RegisterFn2) map[string]*HandlerInfo {
	return newRegistry2(handlerRegFns).handlerBuildersByName
}

// FindHandler returns the handler object with the given name.
func (r *registry2) FindHandler(name string) (b adpCnfg.HandlerBuilder, found bool) {
	bi, found := r.handlerBuildersByName[name]
	if !found {
		return nil, false
	}
	return bi.HandlerBuilder, true

}

func (r *registry2) insertHandler(tmplName string, b adpCnfg.HandlerBuilder) {
	bi := r.handlerBuildersByName[b.Name()]
	if bi == nil {
		bi = &HandlerInfo{HandlerBuilder: b, Templates: make([]string, 0)}
		r.handlerBuildersByName[b.Name()] = bi

	} else if bi.HandlerBuilder != b {
		// panic only if 2 different handler objects are trying to identify by the
		// same Name.  2nd registration is ok so long as old and the new are same
		msg := fmt.Errorf("duplicate registration for '%s' : old = %v new = %v", b.Name(), bi, b)
		glog.Error(msg)
		panic(msg)
	}
	if !contains(bi.Templates, tmplName) {
		bi.Templates = append(bi.Templates, tmplName)
	}
	if glog.V(1) {
		glog.Infof("Registered %s", b.Name())
	}
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
