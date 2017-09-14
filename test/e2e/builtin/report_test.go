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

	istio_mixer_v1 "istio.io/api/mixer/v1"
	pb "istio.io/api/mixer/v1/config/descriptor"
	tmpl "istio.io/mixer/template"
	metricTmpl "istio.io/mixer/template/metric"
	"istio.io/mixer/test/e2e"
	spyAdapter "istio.io/mixer/test/e2e/builtin/spyAdapter"
	testEnv "istio.io/mixer/test/testenv"
)

const (
	adapter1 = "spyAdapter1"
	adapter2 = "spyAdapter2"
	adapter3 = "spyAdapter3"
)

func TestReport(t *testing.T) {
	configDir, err := filepath.Abs("../../../testdata/config/")
	if err != nil {
		t.Fatal(err)
	}
	configDir, err = buildConfigStore(configDir)
	defer func() {
		if removeErr := os.RemoveAll(configDir); removeErr != nil {
			t.Fatal(removeErr)
		}
	}()

	var args = testEnv.Args{
		// Start Mixer server on a free port on loop back interface
		MixerServerAddr:               `127.0.0.1:0`,
		ConfigStoreURL:                `fs://` + configDir,
		ConfigStore2URL:               `fs://` + configDir,
		ConfigDefaultNamespace:        "istio-config-default",
		ConfigIdentityAttribute:       "destination.service",
		ConfigIdentityAttributeDomain: "svc.cluster.local",
		UseAstEvaluator:               true,
	}

	behaviors := []spyAdapter.AdapterBehavior{
		{Name: adapter1, Tmpls: []string{metricTmpl.TemplateName}},
		{Name: adapter2, Tmpls: []string{metricTmpl.TemplateName}},
		{Name: adapter3, Tmpls: []string{metricTmpl.TemplateName}},
	}

	adapterInfos, spyAdpts := constructAdapterInfos(behaviors)
	env, err := testEnv.NewEnv(&args, tmpl.SupportedTmplInfo, adapterInfos)
	if err != nil {
		t.Fatalf("fail to create testenv: %v", err)
	}

	defer closeHelper(env)

	client, conn, err := env.CreateMixerClient()
	if err != nil {
		t.Fatalf("fail to create client connection: %v", err)
	}
	defer closeHelper(conn)

	attrs := map[string]interface{}{"target.name": "somesrvcname"}
	req := istio_mixer_v1.ReportRequest{
		Attributes: []istio_mixer_v1.Attributes{
			testEnv.GetAttrBag(attrs,
				args.ConfigIdentityAttribute,
				args.ConfigIdentityAttributeDomain)},
	}
	_, err = client.Report(context.Background(), &req)

	adptr := spyAdpts[0]

	e2e.CmpMapAndErr(t, "SetMetricTypesTypes input", adptr.BuilderData.SetMetricTypesTypes,
		map[string]interface{}{
			"requestcount.metric.istio-config-default": &metricTmpl.Type{
				Value: pb.INT64,
				Dimensions: map[string]pb.ValueType{
					"method":        pb.STRING,
					"response_code": pb.INT64,
					"service":       pb.STRING,
					"source":        pb.STRING,
					"target":        pb.STRING,
					"version":       pb.STRING,
				},
				MonitoredResourceDimensions: map[string]pb.ValueType{},
			},
		},
	)

	e2e.CmpSliceAndErr(t, "HandleMetricInstances input", adptr.HandlerData.HandleMetricInstances,
		[]*metricTmpl.Instance{
			{
				Name:  "requestcount.metric.istio-config-default",
				Value: int64(1),
				Dimensions: map[string]interface{}{
					"response_code": int64(200),
					"service":       "unknown",
					"source":        "unknown",
					"target":        "unknown",
					"version":       "unknown",
					"method":        "unknown",
				},
				MonitoredResourceType:       "UNSPECIFIED",
				MonitoredResourceDimensions: map[string]interface{}{},
			},
		},
	)
}
