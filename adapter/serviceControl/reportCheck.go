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

package serviceControl

import (
	"fmt"
	"time"

	servicecontrol "google.golang.org/api/servicecontrol/v1"

	"istio.io/mixer/adapter/serviceControl/config"
	"istio.io/mixer/pkg/adapter"
	"github.com/pborman/uuid"
)

type (
	builder struct {
		adapter.DefaultBuilder
	}

	aspect struct {
		serviceName string
		service     *servicecontrol.Service
		logger adapter.Logger
		operationLabels map[string]string
	}
)

var (
	name        = "serviceControl"
	desc        = "Pushes metrics to service controller"
	defaultConf = &config.Params{
		ServiceName:      "xiaolan-library-example.sandbox.googleapis.com",
		ClientCredentialPath: "/Users/xiaolan/credentials/",
		OperationLabels: []{"cloud.googleapis.com/location": "global"},
	}
)

// Register records the builders exposed by this adapter.
func Register(r adapter.Registrar) {
	r.RegisterMetricsBuilder(newBuilder())
}

func newBuilder() *builder {
	return &builder{adapter.NewDefaultBuilder(name, desc, defaultConf)}
}

func (b *builder) ValidateConfig(c adapter.Config) (ce *adapter.ConfigErrors) {
	return nil
}

func (*builder) NewMetricsAspect(env adapter.Env, cfg adapter.Config, metrics map[string]*adapter.MetricDefinition) (adapter.MetricsAspect, error) {
	params := cfg.(*config.Params)

	ss, err := createAPIClient(env.Logger(), params.ClientCredentialPath)

	return &aspect{params.ServiceName, ss, env.Logger(), params.OperationLabels}, err
}

func (a *aspect) Record(values []adapter.Value) error {
	var vs []*servicecontrol.MetricValueSet
	for _, v := range values {
		var mv servicecontrol.MetricValue
		mv.Labels = translate(v.Labels)
		mv.StartTime = v.StartTime.Format(time.RFC3339)
		mv.EndTime = v.EndTime.Format(time.RFC3339)
		i, _ := v.Int64()
		mv.Int64Value = &i

		ms := &servicecontrol.MetricValueSet{
			MetricName:   v.Definition.Name,
			MetricValues: []*servicecontrol.MetricValue{&mv},
		}
		vs = append(vs, ms)
	}

	op := &servicecontrol.Operation{
		OperationId:     fmt.Sprintf("mixer-metric-report-id-%d", uuid.New()),
		OperationName:   "reportMetrics",
		StartTime:       time.Now().Format(time.RFC3339),
		EndTime:         time.Now().Format(time.RFC3339),
		MetricValueSets: vs,
		Labels:          a.operationLabel,
	}
	rq := &servicecontrol.ReportRequest{
		Operations: []*servicecontrol.Operation{op},
	}

	rp, err := a.service.Services.Report(a.serviceName, rq).Do()

	if a.logger.VerbosityLevel(4) {
		a.logger.Infof("service control metric report for operation id %s for metrics %v\nresponse %v",
			op.OperationId, values, rp)
	}
	return err
}

// translate mixer label key to GCP label. This is case by case and needs preconfiguration.
// Current implementation only works for some serviceruntime predefined labels by adding prefix /
func translate(labels map[string]interface{}) map[string]string {
	ml := make(map[string]string)
	for k, v := range labels {
		nk := fmt.Sprintf("/%s", k)
		ml[nk] = fmt.Sprintf("%v", v)
	}
	return ml
}

func (a *aspect) Close() error { return nil }
