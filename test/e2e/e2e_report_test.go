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

package e2e

import (
	"context"
	"os"
	"testing"

	istio_mixer_v1 "istio.io/api/mixer/v1"
	pb "istio.io/api/mixer/v1/config/descriptor"
	"istio.io/mixer/pkg/template"
	e2eTmpl "istio.io/mixer/test/e2e/template"
	reportTmpl "istio.io/mixer/test/e2e/template/report"
	spyAdapter "istio.io/mixer/test/spyAdapter"
	testEnv "istio.io/mixer/test/testenv"
	"io"
	"log"
)

const (
	globalCnfg = `
apiVersion: "config.istio.io/v1alpha2"
kind: attributemanifest
metadata:
  name: istio-proxy
  namespace: default
spec:
    attributes:
      source.name:
        value_type: STRING
      target.name:
        value_type: STRING
      response.count:
        value_type: INT64
      attr.bool:
        value_type: BOOL
      attr.string:
        value_type: STRING
      attr.double:
        value_type: DOUBLE
      attr.int64:
        value_type: INT64
---
`
	reportTestCnfg = `
apiVersion: "config.istio.io/v1alpha2"
kind: fakeHandler
metadata:
  name: fakeHandlerConfig
  namespace: istio-config-default

---

apiVersion: "config.istio.io/v1alpha2"
kind: samplereport
metadata:
  name: reportInstance
  namespace: istio-config-default
spec:
  value: "2"
  dimensions:
    source: source.name | "mysrc"
    target_ip: target.name | "mytarget"

---

apiVersion: "config.istio.io/v1alpha2"
kind: rule
metadata:
  name: rule1
  namespace: istio-config-default
spec:
  selector: target.name == "*"
  actions:
  - handler: fakeHandlerConfig.fakeHandler.istio-config-default
    instances: [ reportInstance.samplereport.istio-config-default ]

---
`
)

type testData struct {
	name          string
	oprtrCnfg     string
	adptBehaviors []spyAdapter.AdptBhvr
	templates     map[string]template.Info
	attribs       map[string]interface{}
	validate      func(t *testing.T, err error, sypAdpts []*spyAdapter.Adptr)
}


func closeHelper(c io.Closer) {
	err := c.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func TestReport(t *testing.T) {
	tests := []testData{
		{
			name:          "Report",
			oprtrCnfg:     reportTestCnfg,
			adptBehaviors: []spyAdapter.AdptBhvr{{Name: "fakeHandler"}},
			templates:     e2eTmpl.SupportedTmplInfo,
			attribs:       map[string]interface{}{"target.name": "somesrvcname"},
			validate: func(t *testing.T, err error, spyAdpts []*spyAdapter.Adptr) {

				adptr := spyAdpts[0]

				CmpMapAndErr("SetSampleReportTypes input", t, adptr.BldrCallData.SetSampleReportTypes_Types,
					map[string]interface{}{
						"reportInstance.samplereport.istio-config-default": &reportTmpl.Type{
							Value:      pb.INT64,
							Dimensions: map[string]pb.ValueType{"source": pb.STRING, "target_ip": pb.STRING},
						},
					},
				)

				CmpSliceAndErr("HandleSampleReport input", t, adptr.HndlrCallData.HandleSampleReport_Instances,
					[]*reportTmpl.Instance{
						{
							Name:       "reportInstance.samplereport.istio-config-default",
							Value:      int64(2),
							Dimensions: map[string]interface{}{"source": "mysrc", "target_ip": "somesrvcname"},
						},
					},
				)
			},
		},
	}
	for _, tt := range tests {
		configDir := GetCnfgs(tt.oprtrCnfg, globalCnfg)
		defer func() {
			if !t.Failed() {
				_ = os.RemoveAll(configDir)
			} else {
				t.Logf("The configs are located at %s", configDir)
			}
		}() // nolint: gas


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
		
		adapterInfos, spyAdapters := CnstrAdapterInfos(tt.adptBehaviors)
		env, err := testEnv.NewEnv(&args, e2eTmpl.SupportedTmplInfo, adapterInfos)
		if err != nil {
			t.Fatalf("fail to create testenv: %v", err)
		}

		defer closeHelper(env)

		client, conn, err := env.CreateMixerClient()
		if err != nil {
			t.Fatalf("fail to create client connection: %v", err)
		}
		defer closeHelper(conn)

		req := istio_mixer_v1.ReportRequest{Attributes: []istio_mixer_v1.Attributes{GetAttributes(tt.attribs, args.ConfigIdentityAttribute, args.ConfigIdentityAttributeDomain)}}
		_, err = client.Report(context.Background(), &req)

		tt.validate(t, err, spyAdapters)
	}
}
