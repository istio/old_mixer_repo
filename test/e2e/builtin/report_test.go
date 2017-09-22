// Copyright 2017 Istio Authors
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

package builtin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	pb "istio.io/api/mixer/v1/config/descriptor"
	listTmpl "istio.io/mixer/template/listentry"
	metricTmpl "istio.io/mixer/template/metric"
	quotaTmpl "istio.io/mixer/template/quota"
)

const (
	adapter1 = "spyAdapter1"
	adapter2 = "spyAdapter2"
	adapter3 = "spyAdapter3"
)

func TestReport(t *testing.T) {
	configDir, _ := filepath.Abs("../../../testdata/config/")
	configDir, _ = buildConfigStore(configDir)
	defer func() {
		_ = os.RemoveAll(configDir)
	}()

	tests := []struct {
		name      string
		behaviors []AdapterBehavior
		attrs     map[string]interface{}
		validate  func(t *testing.T, sypAdpts []*Adapter)
	}{
		{
			name: "Report",
			attrs: map[string]interface{}{
				"target.name":  "somesrvcname",
				"handler.name": "handler1",
			},
			behaviors: []AdapterBehavior{
				{Name: adapter1, Tmpls: []string{metricTmpl.TemplateName, listTmpl.TemplateName}},
				{Name: adapter2, Tmpls: []string{listTmpl.TemplateName, quotaTmpl.TemplateName}},
				{Name: adapter3, Tmpls: []string{metricTmpl.TemplateName, listTmpl.TemplateName, quotaTmpl.TemplateName}},
			},
			validate: func(t *testing.T, spyAdpts []*Adapter) {
				adptr := spyAdpts[0]
				if len(adptr.BuilderData.SetMetricTypesTypes) != 2 {
					t.Errorf("SetMetricTypesTypes called with types =%d; want %d", len(adptr.BuilderData.SetMetricTypesTypes), 2)
				}
				cmpMapAndErr("SetMetricTypesTypes was called with", adptr.BuilderData.SetMetricTypesTypes, map[string]interface{}{
					"requestcount.metric.istio-config-default": &metricTmpl.Type{
						Value: pb.INT64,
						Dimensions: map[string]pb.ValueType{
							"response_code":       pb.INT64,
							"source_service":      pb.STRING,
							"source_version":      pb.STRING,
							"destination_service": pb.STRING,
							"destination_version": pb.STRING,
						},
						MonitoredResourceDimensions: map[string]pb.ValueType{},
					},
					"responsesize.metric.istio-config-default": &metricTmpl.Type{
						Value: pb.INT64,
						Dimensions: map[string]pb.ValueType{
							"response_code":       pb.INT64,
							"source_service":      pb.STRING,
							"source_version":      pb.STRING,
							"destination_service": pb.STRING,
							"destination_version": pb.STRING,
						},
						MonitoredResourceDimensions: map[string]pb.ValueType{},
					},
				}, t)

				cmpSliceAndErr("HandleMetricInstances was called with", adptr.HandlerData.HandleMetricInstances, []*metricTmpl.Instance{
					{
						Name:  "requestcount.metric.istio-config-default",
						Value: int64(1),
						Dimensions: map[string]interface{}{
							"response_code":       int64(200),
							"source_service":      "unknown",
							"source_version":      "unknown",
							"destination_service": "svc.cluster.local",
							"destination_version": "unknown",
						},
						MonitoredResourceType:       "UNSPECIFIED",
						MonitoredResourceDimensions: map[string]interface{}{},
					},
					{
						Name:  "responsesize.metric.istio-config-default",
						Value: int64(0),
						Dimensions: map[string]interface{}{
							"response_code":       int64(200),
							"source_service":      "unknown",
							"source_version":      "unknown",
							"destination_service": "svc.cluster.local",
							"destination_version": "unknown",
						},
						MonitoredResourceType:       "UNSPECIFIED",
						MonitoredResourceDimensions: map[string]interface{}{},
					},
				}, t)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spyAdpts, env, _ := setup(tt.behaviors, configDir)
			defer closeHelper(env)

			client, conn, _ := env.CreateMixerClient()
			defer closeHelper(conn)

			req := createReportReq(tt.attrs)

			_, _ = client.Report(context.Background(), &req)
			tt.validate(t, spyAdpts)
		})
	}
}
