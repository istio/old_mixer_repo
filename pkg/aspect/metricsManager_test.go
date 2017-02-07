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
	"strconv"
	"strings"
	"testing"

	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/adapter/test"
	aconfig "istio.io/mixer/pkg/aspect/config"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/config"
	pb "istio.io/mixer/pkg/config/proto"
	"istio.io/mixer/pkg/expr"
)

type fakeBag struct {
	attribute.Bag
}

type fakeEval struct {
	expr.PredicateEvaluator
	expr.Validator

	body func(string, attribute.Bag) (interface{}, error)
}

func (f *fakeEval) Eval(expression string, attrs attribute.Bag) (interface{}, error) {
	return f.body(expression, attrs)
}

func (f *fakeEval) EvalString(expression string, attrs attribute.Bag) (string, error) {
	r, err := f.body(expression, attrs)
	return r.(string), err
}

type fakeaspect struct {
	adapter.Aspect
	closed bool
	body   func([]adapter.Value) error
}

func (a *fakeaspect) Close() error {
	a.closed = true
	return nil
}

func (a *fakeaspect) Record(v []adapter.Value) error {
	return a.body(v)
}

type fakeBuilder struct {
	adapter.Builder

	body func() (adapter.MetricsAspect, error)
}

func (b *fakeBuilder) NewMetricsAspect(env adapter.Env, config adapter.AspectConfig, metrics []adapter.MetricDefinition) (adapter.MetricsAspect, error) {
	return b.body()
}

func TestNewMetricsManager(t *testing.T) {
	m := NewMetricsManager()
	if m.Kind() != MetricKind {
		t.Errorf("m.Kind() = %s wanted %s", m.Kind(), MetricKind)
	}
	if err := m.ValidateConfig(m.DefaultConfig()); err != nil {
		t.Errorf("m.ValidateConfig(m.DefaultConfig()) = %v; wanted no err", err)
	}
}

func TestMetricsManager_NewAspect(t *testing.T) {
	conf := &config.Combined{
		Aspect: &pb.Aspect{
			Params: &aconfig.MetricsParams{
				Metrics: []*aconfig.MetricsParams_Metric{
					{
						Descriptor_: "api_responses",
						Value:       "",
						Labels:      map[string]string{"api_method": "", "response_code": ""},
					},
				},
			},
		},
		// the params we use here don't matter because we're faking the aspect
		Builder: &pb.Adapter{Params: &aconfig.MetricsParams{}},
	}
	builder := &fakeBuilder{body: func() (adapter.MetricsAspect, error) {
		return &fakeaspect{body: func([]adapter.Value) error { return nil }}, nil
	}}
	if _, err := NewMetricsManager().NewAspect(conf, builder, test.NewEnv(t)); err != nil {
		t.Errorf("NewAspect(conf, builder, test.NewEnv(t)) = _, %v; wanted no err", err)
	}
}

func TestMetricsManager_NewAspect_PropagatesError(t *testing.T) {
	conf := &config.Combined{
		Aspect: &pb.Aspect{Params: &aconfig.MetricsParams{}},
		// the params we use here don't matter because we're faking the aspect
		Builder: &pb.Adapter{Params: &aconfig.MetricsParams{}},
	}
	errString := "expected"
	builder := &fakeBuilder{
		body: func() (adapter.MetricsAspect, error) {
			return nil, fmt.Errorf(errString)
		}}
	_, err := NewMetricsManager().NewAspect(conf, builder, test.NewEnv(t))
	if err == nil {
		t.Error("NewMetricsManager().NewAspect(conf, builder, test.NewEnv(t)) = _, nil; wanted err")
	}
	if !strings.Contains(err.Error(), errString) {
		t.Errorf("NewAspect(conf, builder, test.NewEnv(t)) = _, %v; wanted err %s", err, errString)
	}
}

func TestMetricsWrapper_Execute(t *testing.T) {
	goodEval := &fakeEval{body: func(exp string, _ attribute.Bag) (interface{}, error) {
		switch exp {
		case "value":
			return 1, nil
		case "api_method":
			return "method", nil
		case "response_code":
			return 200, nil
		default:
			return nil, fmt.Errorf("default case for exp = %s", exp)
		}
	}}
	errEval := &fakeEval{body: func(_ string, _ attribute.Bag) (interface{}, error) {
		return nil, fmt.Errorf("expected")
	}}
	labelErrEval := &fakeEval{body: func(exp string, _ attribute.Bag) (interface{}, error) {
		switch exp {
		case "value":
			return 1, nil
		default:
			return nil, fmt.Errorf("expected")
		}
	}}

	goodMd := map[string]*metricInfo{
		"api_responses": {
			metricKind: adapter.Counter,
			value:      "value",
			labels: map[string]string{
				"api_method":    "api_method",
				"response_code": "response_code",
			},
		},
	}
	badGoodMd := map[string]*metricInfo{
		"bad": {
			metricKind: adapter.Counter,
			value:      "bad",
			labels: map[string]string{
				"bad": "bad",
			},
		},
		"api_responses": {
			metricKind: adapter.Counter,
			value:      "value",
			labels: map[string]string{
				"api_method":    "api_method",
				"response_code": "response_code",
			},
		},
	}

	type o struct {
		value  interface{}
		labels []string
	}
	cases := []struct {
		mdin      map[string]*metricInfo
		recordErr error
		eval      *fakeEval
		out       map[string]o
		errString string
	}{
		{make(map[string]*metricInfo), nil, &fakeEval{}, make(map[string]o), ""},
		{goodMd, nil, errEval, make(map[string]o), "expected"},
		{goodMd, nil, labelErrEval, make(map[string]o), "expected"},
		{goodMd, nil, goodEval, map[string]o{"api_responses": {1, []string{"api_method", "response_code"}}}, ""},
		{goodMd, fmt.Errorf("record"), goodEval, map[string]o{"api_responses": {1, []string{"api_method", "response_code"}}}, "record"},
		{badGoodMd, nil, goodEval, map[string]o{"api_responses": {1, []string{"api_method", "response_code"}}}, "default case"},
	}
	for idx, c := range cases {
		t.Run(strconv.Itoa(idx), func(t *testing.T) {
			var receivedValues []adapter.Value
			wrapper := &metricsWrapper{
				aspect: &fakeaspect{body: func(v []adapter.Value) error {
					receivedValues = v
					return c.recordErr
				}},
				metadata: c.mdin,
			}
			_, err := wrapper.Execute(&fakeBag{}, c.eval)

			errString := ""
			if err != nil {
				errString = err.Error()
			}
			if !strings.Contains(errString, c.errString) {
				t.Errorf("wrapper.Execute(&fakeBag{}, eval) = _, %v; wanted err containing %s", err, c.errString)
			}

			if len(receivedValues) != len(c.out) {
				t.Errorf("wrapper.Execute(&fakeBag{}, eval) got vals %v, wanted at least %d", receivedValues, len(c.out))
			}
			for _, v := range receivedValues {
				o, found := c.out[v.Name]
				if !found {
					t.Errorf("Got unexpected value %v, wanted only %v", v, c.out)
				}
				if v.MetricValue != o.value {
					t.Errorf("v.MetricValue = %v; wanted %v", v.MetricValue, o.value)
				}
				for _, l := range o.labels {
					if _, found := v.Labels[l]; !found {
						t.Errorf("value.Labels = %v; wanted label named %s", v.Labels, l)
					}
				}
			}
		})
	}
}

func TestMetricsWrapper_Close(t *testing.T) {
	inner := &fakeaspect{closed: false}
	wrapper := &metricsWrapper{aspect: inner}
	if err := wrapper.Close(); err != nil {
		t.Errorf("wrapper.Close() = %v; wanted no err", err)
	}
	if !inner.closed {
		t.Error("metricsWrapper.Close() didn't close the aspect inside")
	}
}

func TestMetrics_Find(t *testing.T) {
	cases := []struct {
		in   []*aconfig.MetricsParams_Metric
		find string
		out  bool
	}{
		{[]*aconfig.MetricsParams_Metric{}, "", false},
		{[]*aconfig.MetricsParams_Metric{{Descriptor_: "foo"}}, "foo", true},
		{[]*aconfig.MetricsParams_Metric{{Descriptor_: "bar"}}, "foo", false},
	}
	for _, c := range cases {
		t.Run(c.find, func(t *testing.T) {
			if _, found := find(c.in, c.find); found != c.out {
				t.Errorf("find(%v, %s) = _, %t; wanted %t", c.in, c.find, found, c.out)
			}
		})
	}
}
