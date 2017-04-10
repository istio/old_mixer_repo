// Copyright 2017 the Istio Authors.
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
	"github.com/golang/glog"
	rpc "github.com/googleapis/googleapis/google/rpc"

	"istio.io/mixer/pkg/adapter"
	apb "istio.io/mixer/pkg/aspect/config"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/config"
	"istio.io/mixer/pkg/config/descriptor"
	cpb "istio.io/mixer/pkg/config/proto"
	"istio.io/mixer/pkg/expr"
	"istio.io/mixer/pkg/status"
)

type (
	attrGenMgr  struct{}
	attrGenExec struct {
		aspect adapter.AttributesGenerator
		params *apb.AttributeGeneratorsParams
	}
)

func newAttrGenMgr() PreprocessManager {
	return &attrGenMgr{}
}

func (attrGenMgr) Kind() config.Kind {
	return config.AttributesKind
}

func (attrGenMgr) DefaultConfig() (c config.AspectParams) {
	// NOTE: The default config leads to the generation of no new attributes.
	return &apb.AttributeGeneratorsParams{}
}

func (attrGenMgr) ValidateConfig(c config.AspectParams, v expr.Validator, df descriptor.Finder) (cerrs *adapter.ConfigErrors) {
	params := c.(*apb.AttributeGeneratorsParams)
	for n, expr := range params.InputExpressions {
		if _, err := v.TypeCheck(expr, df); err != nil {
			cerrs = cerrs.Appendf("input_expressions", "failed to parse expression '%s' with err: %v", n, err)
		}
	}
	attrs := make([]string, 0, len(params.ValueAttributeMap))
	for _, attrName := range params.ValueAttributeMap {
		attrs = append(attrs, attrName)
	}
	for _, name := range attrs {
		if a := df.GetAttribute(name); a == nil {
			cerrs = cerrs.Appendf(
				"value_attribute_map",
				"Attribute '%s' is not configured for use within the mixer. It cannot be used as a target for generated values.",
				name)
		}
	}
	return
}

func (attrGenMgr) NewPreprocessExecutor(cfg *cpb.Combined, b adapter.Builder, env adapter.Env, df descriptor.Finder) (PreprocessExecutor, error) {
	agb := b.(adapter.AttributesGeneratorBuilder)
	ag, err := agb.BuildAttributesGenerator(env, cfg.Builder.Params.(config.AspectParams))
	if err != nil {
		return nil, err
	}
	return &attrGenExec{aspect: ag, params: cfg.Aspect.Params.(*apb.AttributeGeneratorsParams)}, nil
}

func (e *attrGenExec) Execute(attrs attribute.Bag, mapper expr.Evaluator) (*PreprocessResult, rpc.Status) {
	attrGen := e.aspect
	in, err := evalAll(e.params.InputExpressions, attrs, mapper)
	if err != nil {
		errMsg := "Could not evaluate input expressions for attribute generation."
		glog.Error(errMsg, err)
		return nil, status.WithInternal(errMsg)
	}
	out, err := attrGen.Generate(in)
	if err != nil {
		errMsg := "Attribute value generation failed."
		glog.Error(errMsg, err)
		return nil, status.WithInternal(errMsg)
	}
	bag := attribute.GetMutableBag(nil)
	for key, val := range out {
		if attrName, found := e.params.ValueAttributeMap[key]; found {
			// TODO: type validation?
			bag.Set(attrName, val)
			continue
		}
		glog.Warningf("Generated value '%s' was not mapped to an attribute.", key)
	}
	// TODO: need to check that all attributes in map have been assigned a
	// value?
	return &PreprocessResult{Attrs: bag}, status.OK
}

func (e *attrGenExec) Close() error { return e.aspect.Close() }
