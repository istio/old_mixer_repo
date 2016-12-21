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
	"errors"
	"reflect"

	"google.golang.org/genproto/googleapis/rpc/code"
	listcheckerpb "istio.io/api/istio/config/v1/aspect/listChecker"
	"istio.io/mixer/pkg/aspect"
	"istio.io/mixer/pkg/attribute"
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

// NewAspect creates a listChecker aspect
func (m *manager) NewAspect(cfg *aspect.Config, _aa aspect.Adapter) (aspect.Aspect, error) {
	aa, ok := _aa.(Adapter)
	if !ok {
		return nil, errors.New("Adapter of incorrect type Expected listChecker.Adapter got " + reflect.TypeOf(_aa).String())
	}
	acfg, ok := cfg.Aspect.TypedParams.(*listcheckerpb.Config)
	if !ok {
		return nil, errors.New("Params of Incorrect type Expected listcheckerpb.Config got " + reflect.TypeOf(cfg.Aspect.TypedParams).String())
	}
	implcfg := cfg.Adapter.TypedArgs
	if err := aa.ValidateConfig(implcfg); err != nil {
		return nil, err
	}
	return aa.NewAspect(&AdapterConfig{
		Aspect: acfg,
		Impl:   implcfg})
}

// Execute performs the aspect function based on given Cfg and AdapterCfg and attributes
func (m *manager) Execute(cfg *aspect.Config, _asp aspect.Aspect, ctx attribute.Context, mapper attribute.ExprEvaluator) (*aspect.Output, error) {
	var ok bool
	var err error

	asp, ok := _asp.(Aspect)
	if !ok {
		return nil, errors.New("Invalid type assertion")
	}

	var symbol string
	var symbolExpr string

	// check if mapping is available

	if symbolExpr, ok = cfg.Aspect.Inputs["Symbol"]; !ok {
		return nil, errors.New("Mapping for Symbol not found")
	}

	var iSymbol interface{}
	if iSymbol, err = mapper.Eval(symbolExpr, ctx); err != nil {
		return nil, err
	}

	if symbol, ok = iSymbol.(string); !ok {
		return nil, errors.New("Mapping for Symbol not a string " + reflect.TypeOf(iSymbol).String())
	}

	ok, err = asp.CheckList(&Arg{
		CfgInput: &listcheckerpb.Input{
			Symbol: symbol,
		},
	})
	if err != nil {
		return nil, err
	}
	rCode := code.Code_PERMISSION_DENIED
	if ok {
		rCode = code.Code_OK
	}
	return &aspect.Output{
		Code: rCode,
	}, nil
}

func (*manager) Kind() string {
	return kind
}
