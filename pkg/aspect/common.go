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

package aspect

import (
	"fmt"

	multierror "github.com/hashicorp/go-multierror"

	dpb "istio.io/api/mixer/v1/config/descriptor"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/adapter/config"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/expr"
)

func FromHandler(handler config.Handler) CreateAspectFunc {
	return func(adapter.Env, adapter.Config, ...interface{}) (adapter.Aspect, error) {
		return handler, nil
	}
}

func FromBuilder(builder adapter.Builder) CreateAspectFunc {
	switch b := builder.(type) {
	case adapter.DenialsBuilder:
		// yet `return b.NewDenialsAspect` is a compilation error.
		return func(env adapter.Env, c adapter.Config, _ ...interface{}) (adapter.Aspect, error) {
			return b.NewDenialsAspect(env, c)
		}
	case adapter.AccessLogsBuilder:
		// yet `return b.NewDenialsAspect` is a compilation error.
		return func(env adapter.Env, c adapter.Config, _ ...interface{}) (adapter.Aspect, error) {
			return b.NewAccessLogsAspect(env, c)
		}
	case adapter.ApplicationLogsBuilder:
		// yet `return b.NewDenialsAspect` is a compilation error.
		return func(env adapter.Env, c adapter.Config, _ ...interface{}) (adapter.Aspect, error) {
			return b.NewApplicationLogsAspect(env, c)
		}
	case adapter.AttributesGeneratorBuilder:
		// yet `return b.NewDenialsAspect` is a compilation error.
		return func(env adapter.Env, c adapter.Config, _ ...interface{}) (adapter.Aspect, error) {
			return b.BuildAttributesGenerator(env, c)
		}
	case adapter.ListsBuilder:
		// yet `return b.NewDenialsAspect` is a compilation error.
		return func(env adapter.Env, c adapter.Config, _ ...interface{}) (adapter.Aspect, error) {
			return b.NewListsAspect(env, c)
		}
	case adapter.MetricsBuilder:
		// yet `return b.NewDenialsAspect` is a compilation error.
		return func(env adapter.Env, c adapter.Config, cfg ...interface{}) (adapter.Aspect, error) {
			if len(cfg) < 1 {
				return nil, fmt.Errorf("metric builders must have configuration args.")
			}
			metrics, ok := cfg[0].(map[string]*adapter.MetricDefinition)
			if !ok {
				return nil, fmt.Errorf("arg to metrics builder must be a map[string]*adapter.MetricDefinition, got: %#v", cfg[0])
			}
			return b.NewMetricsAspect(env, c, metrics)
		}
	case adapter.QuotasBuilder:
		// yet `return b.NewDenialsAspect` is a compilation error.
		return func(env adapter.Env, c adapter.Config, cfg ...interface{}) (adapter.Aspect, error) {
			if len(cfg) < 1 {
				return nil, fmt.Errorf("quota builders must have configuration args.")
			}
			quotas, ok := cfg[0].(map[string]*adapter.QuotaDefinition)
			if !ok {
				return nil, fmt.Errorf("arg to quota builder must be a map[string]*adapter.QuotaDefinition, got: %#v", cfg[0])
			}
			return b.NewQuotasAspect(env, c, quotas)
		}
	default:
		// TODO: should we do something stronger here, or maybe change this func to return `(CreateAspectFunc, error)`?
		return func(adapter.Env, adapter.Config, ...interface{}) (adapter.Aspect, error) {
			return nil, fmt.Errorf("Invalid builder type: %#v", builder)
		}
	}
}

func evalAll(expressions map[string]string, attrs attribute.Bag, eval expr.Evaluator) (map[string]interface{}, error) {
	result := &multierror.Error{}
	labels := make(map[string]interface{}, len(expressions))
	for label, texpr := range expressions {
		val, err := eval.Eval(texpr, attrs)
		if err != nil {
			result = multierror.Append(result, fmt.Errorf("failed to construct value for label '%s': %v", label, err))
			continue
		}
		labels[label] = val
	}
	return labels, result.ErrorOrNil()
}

func validateLabels(ceField string, labels map[string]string, labelDescs map[string]dpb.ValueType, v expr.TypeChecker, df expr.AttributeDescriptorFinder) (
	ce *adapter.ConfigErrors) {

	if len(labels) != len(labelDescs) {
		ce = ce.Appendf(ceField, "wrong dimensions: descriptor expects %d labels, found %d labels", len(labelDescs), len(labels))
	}
	for name, exp := range labels {
		if labelType, found := labelDescs[name]; !found {
			ce = ce.Appendf(ceField, "wrong dimensions: extra label named %s", name)
		} else if err := v.AssertType(exp, df, labelType); err != nil {
			ce = ce.Appendf(ceField, "error type checking label '%s': %v", name, err)
		}
	}
	return
}

func validateTemplateExpressions(ceField string, expressions map[string]string, tc expr.TypeChecker, df expr.AttributeDescriptorFinder) (
	ce *adapter.ConfigErrors) {

	// We can't do type assertions since we don't know what each template param needs to resolve to, but we can
	// make sure they're syntactically correct and we have the attributes they need available in the system.
	for name, exp := range expressions {
		if _, err := tc.EvalType(exp, df); err != nil {
			ce = ce.Appendf(ceField, "failed to parse expression '%s': %v", name, err)
		}
	}
	return
}
