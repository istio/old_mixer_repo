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

package stackdriver

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3"
	"github.com/golang/protobuf/ptypes"
	"github.com/googleapis/gax-go"
	xcontext "golang.org/x/net/context"
	gapiopts "google.golang.org/api/option"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"

	"istio.io/mixer/adapter/stackdriver/config"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/adapter/test"
)

var clientFunc = func(err error) createClientFunc {
	return func(cfg *config.Params) (*monitoring.MetricClient, error) {
		if err != nil {
			return nil, err
		}
		return nil, nil
	}
}

func TestFactory_NewMetricsAspect(t *testing.T) {
	tests := []struct {
		name           string
		cfg            *config.Params
		metricNames    []string
		missingMetrics []string // We check that the method logged these metric names because they're not mapped in cfg
		err            string   // If != "" we expect an error containing this string
	}{
		{"empty", &config.Params{}, []string{}, []string{}, ""},
		{"missing metric", &config.Params{}, []string{"request_count"}, []string{"request_count"}, ""},
		{
			"happy path",
			&config.Params{MetricInfo: map[string]*config.Params_MetricInfo{"request_count": {}}},
			[]string{"request_count"},
			[]string{},
			"",
		},
	}

	for idx, tt := range tests {
		t.Run(fmt.Sprintf("[%d] %s", idx, tt.name), func(t *testing.T) {
			metrics := make(map[string]*adapter.MetricDefinition)
			for _, name := range tt.metricNames {
				metrics[name] = &adapter.MetricDefinition{Name: name}
			}
			env := test.NewEnv(t)

			_, err := newFactory(clientFunc(nil)).NewMetricsAspect(env, tt.cfg, metrics)
			if err != nil || tt.err != "" {
				if tt.err == "" {
					t.Fatalf("factory{}.NewMetricsAspect(test.NewEnv(t), nil, nil) = '%s', wanted no err", err.Error())
				} else if !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("Expected errors containing the string '%s', actual: '%s'", tt.err, err.Error())
				}
			}
			// If we expect missing metrics make sure they're present in the logs; otherwise make sure none were missing.
			if len(tt.missingMetrics) > 0 {
				for _, missing := range tt.missingMetrics {
					found := false
					for _, log := range env.GetLogs() {
						found = found || strings.Contains(log, missing)
					}
					if !found {
						t.Errorf("Wanted missing log %s, got logs: %v", missing, env.GetLogs())
					}
				}
			} else {
				for _, log := range env.GetLogs() {
					if strings.Contains(log, "No stackdriver info found for metric") {
						t.Errorf("Expected no missing metrics, found log entry: %s", log)
					}
				}
			}
		})
	}
}

func TestFactory_NewMetricsAspect_Errs(t *testing.T) {
	err := fmt.Errorf("expected")
	f := newFactory(clientFunc(err))
	res, e := f.NewMetricsAspect(test.NewEnv(t), &config.Params{}, map[string]*adapter.MetricDefinition{})
	if e != nil && !strings.Contains(e.Error(), err.Error()) {
		t.Fatalf("Expected error from factory.createClient to be propagated, got %v, %v", res, e)
	} else if e == nil {
		t.Fatalf("Got no error")
	}
}

func TestToOpts(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Params
		out  []gapiopts.ClientOption // we only assert that the types match, so contents of the option don't matter
	}{
		{"empty", &config.Params{}, []gapiopts.ClientOption{}},
		{"api key", &config.Params{Creds: &config.Params_ApiKey{}}, []gapiopts.ClientOption{gapiopts.WithAPIKey("")}},
		{"app creds", &config.Params{Creds: &config.Params_AppCredentials{}}, []gapiopts.ClientOption{}},
		{"service account",
			&config.Params{Creds: &config.Params_ServiceAccountPath{}},
			[]gapiopts.ClientOption{gapiopts.WithServiceAccountFile("")}},
		{"endpoint",
			&config.Params{Endpoint: "foo.bar"},
			[]gapiopts.ClientOption{gapiopts.WithEndpoint("")}},
		{"endpoint + svc account",
			&config.Params{Endpoint: "foo.bar", Creds: &config.Params_ServiceAccountPath{}},
			[]gapiopts.ClientOption{gapiopts.WithEndpoint(""), gapiopts.WithServiceAccountFile("")}},
	}
	for idx, tt := range tests {
		t.Run(fmt.Sprintf("[%d] %s", idx, tt.name), func(t *testing.T) {
			opts := toOpts(tt.cfg)
			if len(opts) != len(tt.out) {
				t.Errorf("len(toOpts(%v)) = %d, expected %d", tt.cfg, len(opts), len(tt.out))
			}

			optSet := make(map[gapiopts.ClientOption]struct{})
			for _, opt := range opts {
				optSet[opt] = struct{}{}
			}

			for _, expected := range tt.out {
				found := false
				for _, actual := range opts {
					// We care that the types are what we expect, no necessarily that they're identical
					found = found || (reflect.TypeOf(expected) == reflect.TypeOf(actual))
				}
				if !found {
					t.Errorf("toOpts() = %v, wanted opt '%v' (type %v)", opts, expected, reflect.TypeOf(expected))
				}
			}
		})
	}
}

func TestRecord(t *testing.T) {
	projectID := "pid"
	resource := &monitoredres.MonitoredResource{
		Type: "global",
		Labels: map[string]string{
			"project_id": projectID,
		},
	}
	metric := &metricpb.Metric{
		Type:   "type",
		Labels: map[string]string{"str": "str", "int": "34"},
	}
	info := map[string]sdinfo{
		"gauge":      {ttype: "type", kind: metricpb.MetricDescriptor_GAUGE, value: metricpb.MetricDescriptor_INT64},
		"cumulative": {ttype: "type", kind: metricpb.MetricDescriptor_CUMULATIVE, value: metricpb.MetricDescriptor_STRING},
		"delta":      {ttype: "type", kind: metricpb.MetricDescriptor_DELTA, value: metricpb.MetricDescriptor_BOOL},
	}
	now := time.Now()
	pbnow, _ := ptypes.TimestampProto(now)

	tests := []struct {
		name     string
		vals     []adapter.Value
		expected []*monitoringpb.TimeSeries
	}{
		{"empty", []adapter.Value{}, []*monitoringpb.TimeSeries{}},
		{"missing", []adapter.Value{
			{
				Definition: &adapter.MetricDefinition{Name: "not in the info map"},
			},
		}, []*monitoringpb.TimeSeries{}},
		{"gauge", []adapter.Value{
			{
				Definition:  &adapter.MetricDefinition{Name: "gauge", Value: adapter.Int64},
				MetricValue: int64(7),
				EndTime:     now,
				Labels:      map[string]interface{}{"str": "str", "int": int64(34)},
			},
		}, []*monitoringpb.TimeSeries{
			{
				Metric:     metric,
				Resource:   resource,
				MetricKind: metricpb.MetricDescriptor_GAUGE,
				ValueType:  metricpb.MetricDescriptor_INT64,
				Points: []*monitoringpb.Point{{
					Interval: &monitoringpb.TimeInterval{EndTime: pbnow},
					Value:    &monitoringpb.TypedValue{&monitoringpb.TypedValue_Int64Value{Int64Value: int64(7)}},
				}},
			},
		}},
		{"cumulative", []adapter.Value{
			{
				Definition:  &adapter.MetricDefinition{Name: "cumulative", Value: adapter.String},
				MetricValue: "s",
				EndTime:     now,
				Labels:      map[string]interface{}{"str": "str", "int": int64(34)},
			},
		}, []*monitoringpb.TimeSeries{
			{
				Metric:     metric,
				Resource:   resource,
				MetricKind: metricpb.MetricDescriptor_CUMULATIVE,
				ValueType:  metricpb.MetricDescriptor_STRING,
				Points: []*monitoringpb.Point{{
					Interval: &monitoringpb.TimeInterval{EndTime: pbnow},
					Value:    &monitoringpb.TypedValue{&monitoringpb.TypedValue_StringValue{StringValue: "s"}},
				}},
			},
		}},
		{"delta", []adapter.Value{
			{
				Definition:  &adapter.MetricDefinition{Name: "delta", Value: adapter.Bool},
				MetricValue: true,
				EndTime:     now,
				Labels:      map[string]interface{}{"str": "str", "int": int64(34)},
			},
		}, []*monitoringpb.TimeSeries{
			{
				Metric:     metric,
				Resource:   resource,
				MetricKind: metricpb.MetricDescriptor_DELTA,
				ValueType:  metricpb.MetricDescriptor_BOOL,
				Points: []*monitoringpb.Point{{
					Interval: &monitoringpb.TimeInterval{EndTime: pbnow},
					Value:    &monitoringpb.TypedValue{&monitoringpb.TypedValue_BoolValue{BoolValue: true}},
				}},
			},
		}},
	}

	for idx, tt := range tests {
		t.Run(fmt.Sprintf("[%d] %s", idx, tt.name), func(t *testing.T) {
			s := &sd{metricInfo: info, m: sync.Mutex{}, projectID: projectID}
			_ = s.Record(tt.vals)

			if len(s.toSend) != len(tt.expected) {
				t.Errorf("Want %d values to send, got %d", len(tt.expected), len(s.toSend))
			}
			for _, expected := range tt.expected {
				found := false
				for _, actual := range s.toSend {
					found = found || reflect.DeepEqual(expected, actual)
				}
				if !found {
					t.Errorf("Want timeseries %v, but not present: %v", expected, s.toSend)
				}
			}
		})
	}
}

func TestPushData(t *testing.T) {
	mkfunc := func(f func() error) pushFunc {
		return func(ctx xcontext.Context, req *monitoringpb.CreateTimeSeriesRequest, opts ...gax.CallOption) error {
			return f()
		}
	}

	tests := []struct {
		name         string
		toSend       []*monitoringpb.TimeSeries
		push         pushFunc
		expectedLogs []string // we don't have a good handle into error cases other than logs
	}{
		{"no data", []*monitoringpb.TimeSeries{}, mkfunc(func() error { panic("") }), []string{"No data to send"}},
		{"push err", []*monitoringpb.TimeSeries{{}}, mkfunc(func() error { return fmt.Errorf("expected") }), []string{"expected"}},
		{"happy path", []*monitoringpb.TimeSeries{{}}, mkfunc(func() error { return nil }), []string{"1 timeseries"}},
	}

	for idx, tt := range tests {
		t.Run(fmt.Sprintf("[%d] %s", idx, tt.name), func(t *testing.T) {
			env := test.NewEnv(t)
			s := &sd{pushMetrics: tt.push, toSend: tt.toSend, env: env, projectID: "pid"}
			s.pushData()

			logs := env.GetLogs()
			for _, expected := range tt.expectedLogs {
				found := false
				for _, actual := range logs {
					found = found || strings.Contains(actual, expected)
				}
				if !found {
					t.Errorf("Want log '%s', got: %v", expected, logs)
				}
			}
		})
	}
}
