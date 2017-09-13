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
	"io"
	"log"
	"path/filepath"
	"testing"

	istio_mixer_v1 "istio.io/api/mixer/v1"
	"istio.io/mixer/pkg/adapter"
	tmpl "istio.io/mixer/template"
	metricTmpl "istio.io/mixer/template/metric"
	spyAdapter "istio.io/mixer/test/e2e/builtintemplate/spyAdapter"
	testEnv "istio.io/mixer/test/testenv"
)

func closeHelper(c io.Closer) {
	err := c.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func TestReport(t *testing.T) {
	cfgDir, err := filepath.Abs("../../../testdata/config/")
	if err != nil {
		t.Fatal(err)
	}

	var args = testEnv.Args{
		// Start Mixer server on a free port on loop back interface
		MixerServerAddr:               `127.0.0.1:0`,
		ConfigStoreURL:                `fs://` + cfgDir,
		ConfigStore2URL:               `fs://` + cfgDir,
		ConfigDefaultNamespace:        "istio-config-default",
		ConfigIdentityAttribute:       "destination.service",
		ConfigIdentityAttributeDomain: "svc.cluster.local",
		UseAstEvaluator:               true,
	}

	behaviors := []spyAdapter.AdapterBehavior{{Name: "fakeHandler", Tmpls: []string{metricTmpl.TemplateName}}}
	adapterInfos, _ := ConstructAdapterInfos(behaviors)
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

	attrs := map[string]interface{}{}
	req := istio_mixer_v1.ReportRequest{
		Attributes: []istio_mixer_v1.Attributes{
			testEnv.GetAttrBag(attrs,
				args.ConfigIdentityAttribute,
				args.ConfigIdentityAttributeDomain)},
	}
	_, err = client.Report(context.Background(), &req)

	// TODO ADD VALIDATION.
}

// ConstructAdapterInfos constructs spyAdapters for each of the adptBehavior. It returns
// the constructed spyAdapters along with the adapters Info functions.
func ConstructAdapterInfos(adptBehaviors []spyAdapter.AdapterBehavior) ([]adapter.InfoFn, []*spyAdapter.Adapter) {
	var adapterInfos []adapter.InfoFn = make([]adapter.InfoFn, 0)
	spyAdapters := make([]*spyAdapter.Adapter, 0)
	for _, b := range adptBehaviors {
		sa := spyAdapter.NewSpyAdapter(b)
		spyAdapters = append(spyAdapters, sa)
		adapterInfos = append(adapterInfos, sa.GetAdptInfoFn())
	}
	return adapterInfos, spyAdapters
}
