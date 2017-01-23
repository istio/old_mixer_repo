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
	denyCheckerManager struct{}

	denyCheckerWrapper struct {
		aspect adapter.DenyCheckerAspect
	}
)

// NewDenyCheckerManager returns an instance of the DenyChecker aspect manager.
func NewDenyCheckerManager() Manager {
	return denyCheckerManager{}
}

// NewAspect creates a denyChecker aspect.
func (m denyCheckerManager) NewAspect(c *CombinedConfig, b adapter.Builder, env adapter.Env) (Wrapper, error) {
	aspectCfg := m.DefaultConfig().(*config.DenyCheckerParams)
	if c.Aspect.Params != nil {
		if err := structToProto(c.Aspect.Params, aspectCfg); err != nil {
			return nil, fmt.Errorf("could not parse aspect config: %v", err)
		}
	}

	bldr := b.(adapter.DenyCheckerBuilder)
	adapterCfg := bldr.DefaultConfig()
	if c.Builder.Params != nil {
		if err := structToProto(c.Builder.Params, adapterCfg); err != nil {
			return nil, fmt.Errorf("could not parse adapter config: %v", err)
		}
	}

	a, err := bldr.NewDenyChecker(env, adapterCfg)
	if err != nil {
		return nil, err
	}

	return &denyCheckerWrapper{a}, nil
}

func (denyCheckerManager) Kind() string                                                     { return "istio/denyChecker" }
func (denyCheckerManager) DefaultConfig() adapter.AspectConfig                              { return &config.DenyCheckerParams{} }
func (denyCheckerManager) ValidateConfig(c adapter.AspectConfig) (ce *adapter.ConfigErrors) { return }

func (a *denyCheckerWrapper) Execute(attrs attribute.Bag, mapper expr.Evaluator) (*Output, error) {
	status := a.aspect.Deny()
	return &Output{Code: code.Code(status.Code)}, nil
}
