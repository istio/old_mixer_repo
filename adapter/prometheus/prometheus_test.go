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

package prometheus

import (
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"istio.io/mixer/adapter/prometheus/config"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/adapter/test"
)

type testServer struct {
	server

	errOnStart bool
}

func (t testServer) Start(adapter.Logger) error {
	if t.errOnStart {
		return errors.New("could not start server")
	}
	return nil
}

var (
	gaugeNoLabels = adapter.MetricDefinition{
		Name:        "/funky::gauge",
		Description: "funky all the time",
		Kind:        adapter.Gauge,
		Labels:      map[string]adapter.LabelType{},
	}

	counterNoLabels = adapter.MetricDefinition{
		Name:        "the.counter",
		Description: "count all the tests",
		Kind:        adapter.Counter,
		Labels:      map[string]adapter.LabelType{},
	}

	counter = adapter.MetricDefinition{
		Name:        "special_counter",
		Description: "count all the special tests",
		Kind:        adapter.Counter,
		Labels: map[string]adapter.LabelType{
			"bool":   adapter.Bool,
			"string": adapter.String,
			"email":  adapter.EmailAddress,
		},
	}

	unknown = adapter.MetricDefinition{
		Name:        "unknown",
		Description: "unknown",
		Kind:        adapter.Gauge - 2,
		Labels:      map[string]adapter.LabelType{},
	}

	counterVal = adapter.Value{
		Name: counter.Name,
		Labels: map[string]interface{}{
			"bool":   true,
			"string": "testing",
			"email":  "test@istio.io",
		},
		Kind:        adapter.Counter,
		MetricValue: float64(45),
	}

	gaugeVal = newGaugeVal(int64(993))
)

func TestInvariants(t *testing.T) {
	test.AdapterInvariants(Register, t)
}

func TestFactory_NewMetricsAspect(t *testing.T) {
	f := newFactory(&testServer{})

	tests := []struct {
		name    string
		metrics []adapter.MetricDefinition
	}{
		{"No Metrics", []adapter.MetricDefinition{}},
		{"One Gauge", []adapter.MetricDefinition{gaugeNoLabels}},
		{"One Counter", []adapter.MetricDefinition{counterNoLabels}},
		{"Multiple Metrics", []adapter.MetricDefinition{counterNoLabels, gaugeNoLabels}},
		{"With Labels", []adapter.MetricDefinition{counter}},
	}

	for _, v := range tests {
		t.Run(v.name, func(t *testing.T) {
			if _, err := f.NewMetricsAspect(test.NewEnv(t), &config.Params{}, v.metrics); err != nil {
				t.Errorf("NewMetricsAspect() => unexpected error: %v", err)
			}
		})
	}
}

func TestFactory_NewMetricsAspectServerFail(t *testing.T) {
	f := newFactory(&testServer{errOnStart: true})
	if _, err := f.NewMetricsAspect(test.NewEnv(t), &config.Params{}, []adapter.MetricDefinition{}); err == nil {
		t.Error("NewMetricsAspect() => expected error on server startup")
	}
}

func TestFactory_NewMetricsAspectMetricDefinitionErrors(t *testing.T) {
	f := newFactory(&testServer{})

	gaugeWithLabels := adapter.MetricDefinition{
		Name:        "/funky::gauge",
		Description: "funky all the time",
		Kind:        adapter.Gauge,
		Labels: map[string]adapter.LabelType{
			"test": adapter.String,
		},
	}

	altCounter := adapter.MetricDefinition{
		Name:        "special_counter",
		Description: "count all the special tests",
		Kind:        adapter.Counter,
		Labels: map[string]adapter.LabelType{
			"email": adapter.EmailAddress,
		},
	}

	tests := []struct {
		name    string
		metrics []adapter.MetricDefinition
	}{
		{"Gauge Definition Conflicts", []adapter.MetricDefinition{gaugeNoLabels, gaugeWithLabels}},
		{"Gauge Definition Conflicts", []adapter.MetricDefinition{counter, altCounter}},
		{"Unknown Metric MetricKind", []adapter.MetricDefinition{unknown}},
	}

	for _, v := range tests {
		t.Run(v.name, func(t *testing.T) {
			if _, err := f.NewMetricsAspect(test.NewEnv(t), &config.Params{}, v.metrics); err == nil {
				t.Error("NewMetricsAspect() => expected error during metrics registration")
			}
		})
	}
}

func TestProm_Close(t *testing.T) {
	f := newFactory(&testServer{})
	prom, _ := f.NewMetricsAspect(test.NewEnv(t), &config.Params{}, []adapter.MetricDefinition{})
	if err := prom.Close(); err != nil {
		t.Errorf("Close() should not have returned an error: %v", err)
	}
}

func TestProm_Record(t *testing.T) {
	f := newFactory(&testServer{})
	tests := []struct {
		name    string
		metrics []adapter.MetricDefinition
		values  []adapter.Value
	}{
		{"Increment Counter", []adapter.MetricDefinition{counter}, []adapter.Value{counterVal}},
		{"Change Gauge", []adapter.MetricDefinition{gaugeNoLabels}, []adapter.Value{gaugeVal}},
		{"Counter and Gauge", []adapter.MetricDefinition{counterNoLabels, gaugeNoLabels}, []adapter.Value{gaugeVal, newCounterVal(float64(16))}},
		{"Int64", []adapter.MetricDefinition{gaugeNoLabels}, []adapter.Value{newGaugeVal(int64(8))}},
		{"String", []adapter.MetricDefinition{gaugeNoLabels}, []adapter.Value{newGaugeVal("8.243543")}},
	}

	for _, v := range tests {
		t.Run(v.name, func(t *testing.T) {
			aspect, err := f.NewMetricsAspect(test.NewEnv(t), &config.Params{}, v.metrics)
			if err != nil {
				t.Errorf("NewMetricsAspect() => unexpected error: %v", err)
			}
			err = aspect.Record(v.values)
			if err != nil {
				t.Errorf("Record() => unexpected error: %v", err)
			}
			// Check tautological recording of entries.
			pr := aspect.(*prom)
			for _, adapterVal := range v.values {
				c, ok := pr.metrics[adapterVal.Name]
				if !ok {
					t.Errorf("Record() could not find metric with name %s:", adapterVal.Name)
					continue
				}

				m := new(dto.Metric)
				switch c.(type) {
				case *prometheus.CounterVec:
					if err := c.(*prometheus.CounterVec).With(promLabels(adapterVal.Labels)).Write(m); err != nil {
						t.Errorf("Error writing metric value to proto: %v", err)
						continue
					}
				case *prometheus.GaugeVec:
					if err := c.(*prometheus.GaugeVec).With(promLabels(adapterVal.Labels)).Write(m); err != nil {
						t.Errorf("Error writing metric value to proto: %v", err)
						continue
					}
				}

				got := metricValue(m)
				want, err := promValue(adapterVal)
				if err != nil {
					t.Errorf("Record(%s) could not get desired value: %v", adapterVal.Name, err)
				}
				if got != want {
					t.Errorf("Record(%s) => %f, want %f", adapterVal.Name, got, want)
				}
			}

		})
	}
}

func TestProm_RecordFailures(t *testing.T) {
	f := newFactory(&testServer{})
	unsupported := adapter.Value{
		Name:        counterNoLabels.Name,
		Labels:      map[string]interface{}{},
		Kind:        adapter.Gauge - 2,
		MetricValue: 99,
	}
	tests := []struct {
		name    string
		metrics []adapter.MetricDefinition
		values  []adapter.Value
	}{
		{"Not Found", []adapter.MetricDefinition{counterNoLabels}, []adapter.Value{newGaugeVal(true)}},
		{"Bool", []adapter.MetricDefinition{gaugeNoLabels}, []adapter.Value{newGaugeVal(true)}},
		{"Text String (Gauge)", []adapter.MetricDefinition{gaugeNoLabels}, []adapter.Value{newGaugeVal("not a value")}},
		{"Text String (Counter)", []adapter.MetricDefinition{counterNoLabels}, []adapter.Value{newCounterVal("not a value")}},
		{"Unsupported Metric MetricKind", []adapter.MetricDefinition{counterNoLabels}, []adapter.Value{unsupported}},
	}

	for _, v := range tests {
		t.Run(v.name, func(t *testing.T) {
			aspect, err := f.NewMetricsAspect(test.NewEnv(t), &config.Params{}, v.metrics)
			if err != nil {
				t.Errorf("NewMetricsAspect() => unexpected error: %v", err)
			}
			err = aspect.Record(v.values)
			if err == nil {
				t.Errorf("Record() - expected error, got none")
			}
		})
	}
}

func metricValue(m *dto.Metric) float64 {
	if c := m.GetCounter(); c != nil {
		return *c.Value
	}
	if c := m.GetGauge(); c != nil {
		return *c.Value
	}
	if c := m.GetUntyped(); c != nil {
		return *c.Value
	}
	return -1
}

func newGaugeVal(val interface{}) adapter.Value {
	return adapter.Value{
		Name:        gaugeNoLabels.Name,
		Labels:      map[string]interface{}{},
		Kind:        adapter.Gauge,
		MetricValue: val,
	}
}

func newCounterVal(val interface{}) adapter.Value {
	return adapter.Value{
		Name:        counterNoLabels.Name,
		Labels:      map[string]interface{}{},
		Kind:        adapter.Counter,
		MetricValue: val,
	}
}
