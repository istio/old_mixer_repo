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

	"github.com/davecgh/go-spew/spew"

	istio_mixer_v1 "istio.io/api/mixer/v1"
	pb "istio.io/api/mixer/v1/config/descriptor"
	tmpl "istio.io/mixer/template"
	metricTmpl "istio.io/mixer/template/metric"
	"istio.io/mixer/test/e2e"
	testEnv "istio.io/mixer/test/testenv"
)

const (
	adapter1 = "spyAdapter1"
	adapter2 = "spyAdapter2"
	adapter3 = "spyAdapter3"

	identityAttr       = "destination.service"
	identityAttrDomain = "svc.cluster.local"
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
			name:  "Report",
			attrs: map[string]interface{}{"target.name": "somesrvcname"},
			behaviors: []AdapterBehavior{
				{Name: adapter1, Tmpls: []string{metricTmpl.TemplateName}},
				{Name: adapter2, Tmpls: []string{metricTmpl.TemplateName}},
				{Name: adapter3, Tmpls: []string{metricTmpl.TemplateName}},
			},
			validate: func(t *testing.T, spyAdpts []*Adapter) {
				adptr := spyAdpts[0]
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
func createReportReq(attrs map[string]interface{}) istio_mixer_v1.ReportRequest {
	req := istio_mixer_v1.ReportRequest{
		Attributes: []istio_mixer_v1.Attributes{
			testEnv.GetAttrBag(attrs,
				identityAttr,
				identityAttrDomain)},
	}
	return req
}

func cmpSliceAndErr(msg string, t1 interface{}, wantInsts []*metricTmpl.Instance, t *testing.T) {
	if !e2e.CmpSliceAndErr(t1, wantInsts) {
		t.Errorf("%s:\n%s\n\nwant :\n%s", msg, spew.Sdump(t1), spew.Sdump(wantInsts))
	}
}
func cmpMapAndErr(msg string, t1 interface{}, wantTypes map[string]interface{}, t *testing.T) {
	if !e2e.CmpMapAndErr(t1, wantTypes) {
		t.Errorf("%s:\n%s\n\nwant :\n%s", msg, spew.Sdump(t1), spew.Sdump(wantTypes))
	}
}

func setup(behaviors []AdapterBehavior, configDir string) ([]*Adapter, testEnv.TestEnv, error) {
	adapterInfos, spyAdpts := constructAdapterInfos(behaviors)
	var args = testEnv.Args{
		// Start Mixer server on a free port on loop back interface
		MixerServerAddr:               `127.0.0.1:0`,
		ConfigStoreURL:                `fs://` + configDir,
		ConfigStore2URL:               `fs://` + configDir,
		ConfigDefaultNamespace:        "istio-config-default",
		ConfigIdentityAttribute:       identityAttr,
		ConfigIdentityAttributeDomain: identityAttrDomain,
		UseAstEvaluator:               false,
	}
	env, err := testEnv.NewEnv(&args, tmpl.SupportedTmplInfo, adapterInfos)
	return spyAdpts, env, err
}
