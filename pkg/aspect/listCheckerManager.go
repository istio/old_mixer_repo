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

package aspect

import (
	"fmt"

	"google.golang.org/genproto/googleapis/rpc/code"

	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/aspect/config"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/expr"
)

type (
	listCheckerManager struct{}

	listCheckerWrapper struct {
		aspect         adapter.ListCheckerAspect
		blacklist      bool
		inputs         map[string]string
		checkAttribute string
	}
)

// NewListCheckerManager returns an instance of the ListChecker aspect manager.
func NewListCheckerManager() Manager {
	return listCheckerManager{}
}

// NewAspect creates a listChecker aspect.
func (m listCheckerManager) NewAspect(c *CombinedConfig, b adapter.Builder, env adapter.Env) (Wrapper, error) {
	aspectCfg := m.DefaultConfig().(*config.ListCheckerParams)
	if c.Aspect.Params != nil {
		if err := structToProto(c.Aspect.Params, aspectCfg); err != nil {
			return nil, fmt.Errorf("could not parse aspect config: %v", err)
		}
	}

	bldr := b.(adapter.ListCheckerBuilder)
	adapterCfg := bldr.DefaultConfig()
	if c.Builder.Params != nil {
		if err := structToProto(c.Builder.Params, adapterCfg); err != nil {
			return nil, fmt.Errorf("could not parse adapter config: %v", err)
		}
	}

	var inputs map[string]string
	if c.Aspect != nil && c.Aspect.Inputs != nil {
		inputs = c.Aspect.Inputs
	}

	a, err := bldr.NewListChecker(env, adapterCfg)
	if err != nil {
		return nil, err
	}

	return &listCheckerWrapper{
		aspect:         a,
		blacklist:      aspectCfg.Blacklist,
		inputs:         inputs,
		checkAttribute: aspectCfg.CheckAttribute,
	}, nil
}

func (listCheckerManager) Kind() string {
	return "istio/listChecker"
}

func (listCheckerManager) DefaultConfig() adapter.AspectConfig {
	return &config.ListCheckerParams{
		CheckAttribute: "src.ip",
	}
}

func (listCheckerManager) ValidateConfig(c adapter.AspectConfig) (ce *adapter.ConfigErrors) {
	lc := c.(*config.ListCheckerParams)
	if lc.CheckAttribute == "" {
		ce = ce.Appendf("CheckAttribute", "Missing")
	}
	return
}

func (a *listCheckerWrapper) Execute(attrs attribute.Bag, mapper expr.Evaluator) (*Output, error) {
	var found bool
	var err error
	var symbol string
	var symbolExpr string

	// CheckAttribute should be processed and sent to input
	if symbolExpr, found = a.inputs[a.checkAttribute]; !found {
		return nil, fmt.Errorf("mapping for %s not found", a.checkAttribute)
	}

	if symbol, err = mapper.EvalString(symbolExpr, attrs); err != nil {
		return nil, err
	}

	if found, err = a.aspect.CheckList(symbol); err != nil {
		return nil, err
	}
	rCode := code.Code_PERMISSION_DENIED

	if found != a.blacklist {
		rCode = code.Code_OK
	}

	return &Output{Code: rCode}, nil
}
