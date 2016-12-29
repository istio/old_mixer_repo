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

package listChecker

import (
	"fmt"

	listcheckerpb "istio.io/api/istio/config/v1/aspect/listChecker"

	"google.golang.org/genproto/googleapis/rpc/code"
	"istio.io/mixer/pkg/aspect"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/expr"
)

const (
	kind = "istio/listChecker"
)

type (
	manager struct{}
)

// Manager returns "this" aspect Manager
func Manager() aspect.Manager {
	return &manager{}
}

// NewAspect implements aspect.Manager#NewAspect() Creates a listChecker aspect
func (m *manager) NewAspect(cfg *aspect.CombinedConfig, ga aspect.Adapter) (aspect.Aspect, error) {
	aa, ok := ga.(Adapter)
	if !ok {
		return nil, fmt.Errorf("Adapter of incorrect type. Expected listChecker.Adapter got %#v %T", ga, ga)
	}
	_, ok = cfg.Aspect.TypedParams.(*listcheckerpb.Config)
	if !ok {
		return nil, fmt.Errorf("Params of Incorrect type. Expected listcheckerpb.Config got %#v %T", cfg.Aspect.TypedParams, cfg.Aspect.TypedParams)
	}

	if err := aa.ValidateConfig(cfg.Adapter.TypedArgs); err != nil {
		return nil, err
	}
	return aa.NewAspect(
		&AdapterConfig{
			ImplConfig: cfg.Adapter.TypedArgs,
		})
}

// Execute implements aspect.Manager#Execute()
func (m *manager) Execute(cfg *aspect.CombinedConfig, ga aspect.Aspect, attrs attribute.Bag, mapper expr.Evaluator) (*aspect.Output, error) {
	var found bool
	var err error
	var asp Aspect

	asp, found = ga.(Aspect)
	if !found {
		return nil, fmt.Errorf("Unexpected aspect type %#v", ga)
	}

	var symbol string
	var symbolExpr string
	var acfg *listcheckerpb.Config

	// check if mapping is available
	if acfg, found = cfg.Aspect.TypedParams.(*listcheckerpb.Config); !found {
		return nil, fmt.Errorf("Unexpected type %#v", cfg.Aspect.TypedParams)
	}

	// CheckAttribute should be processed and sent to input
	if symbolExpr, found = cfg.Aspect.Inputs[acfg.CheckAttribute]; !found {
		return nil, fmt.Errorf("Mapping for %s not found", acfg.CheckAttribute)
	}

	if symbol, err = mapper.EvalString(symbolExpr, attrs); err != nil {
		return nil, err
	}

	if found, err = asp.CheckList(symbol); err != nil {
		return nil, err
	}
	rCode := code.Code_PERMISSION_DENIED

	if found != acfg.Blacklist {
		rCode = code.Code_OK
	}

	return &aspect.Output{Code: rCode}, nil
}

func (*manager) Kind() string {
	return kind
}
