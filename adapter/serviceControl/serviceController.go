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
	"errors"
	"fmt"
	"time"

	"github.com/pborman/uuid"
	servicecontrol "google.golang.org/api/servicecontrol/v1"

	"istio.io/mixer/adapter/serviceControl/config"
	"istio.io/mixer/pkg/adapter"
)

type (
	builder struct {
		adapter.DefaultBuilder

		client
	}

	aspect struct {
		serviceName string
		service     *servicecontrol.Service
		logger      adapter.Logger
	}
)

var (
	name = "serviceControl"
)

// Register records the builders exposed by this adapter.
func Register(r adapter.Registrar) {
	r.RegisterMetricsBuilder(
		&builder{adapter.NewDefaultBuilder(
			name,
			"Push metrics to GCP service controller",
			new(config.Params),
		), new(clientImpl)})

	r.RegisterApplicationLogsBuilder(
		&builder{adapter.NewDefaultBuilder(
			name,
			"Writes log entries to GCP service controller",
			new(config.Params),
		), new(clientImpl)})
}

func (*builder) ValidateConfig(c adapter.Config) (ce *adapter.ConfigErrors) {
	return nil
}

func (b *builder) NewMetricsAspect(env adapter.Env, cfg adapter.Config, metrics map[string]*adapter.MetricDefinition) (adapter.MetricsAspect, error) {
	return b.newAspect(env, cfg)
}

func (b *builder) NewApplicationLogsAspect(env adapter.Env, cfg adapter.Config) (adapter.ApplicationLogsAspect, error) {
	return b.newAspect(env, cfg)
}

func (*builder) NewAccessLogsAspect(env adapter.Env, cfg adapter.Config) (adapter.AccessLogsAspect, error) {
	return nil, errors.New("access logs are not supported")
}

func (b *builder) newAspect(env adapter.Env, cfg adapter.Config) (*aspect, error) {
	params := cfg.(*config.Params)
	ss, err := b.client.create(env.Logger())
	if err != nil {
		return nil, err
	}
	return &aspect{params.ServiceName, ss, env.Logger()}, nil
}

func (a *aspect) Record(values []adapter.Value) error {
	var vs []*servicecontrol.MetricValueSet
	var method string
	for _, v := range values {
		// TODO Only support request count.
		if v.Definition.Name != "request_count" {
			a.logger.Warningf("service control metric got unsupported metric name: %s\n", v.Definition.Name)
			continue
		}
		var mv servicecontrol.MetricValue
		mv.Labels, method = translate(v.Labels)
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
		Labels: map[string]string{
			"cloud.googleapis.com/location":            "global",
			"serviceruntime.googleapis.com/api_method": method,
		},
	}
	rq := &servicecontrol.ReportRequest{
		Operations: []*servicecontrol.Operation{op},
	}

	rp, err := a.service.Services.Report(a.serviceName, rq).Do()

	if a.logger.VerbosityLevel(4) {
		a.logger.Infof("service control metric report operation id %s\nmetrics %v\nresponse %v",
			op.OperationId, values, rp)
	}
	return err
}

// translate mixer label key to GCP label, and find the api method. TODO Need general redesign.
// Current implementation is just adding a prefix / which does not work for some labels.
func translate(labels map[string]interface{}) (map[string]string, string) {
	ml := make(map[string]string)
	var method string
	for k, v := range labels {
		nk := fmt.Sprintf("/%s", k)
		ml[nk] = fmt.Sprintf("%v", v)
		if k == "request.method" {
			method = ml[nk]
		}
	}
	return ml, method
}

//TODO Add supports to struct payload.
func (a *aspect) log(entries []adapter.LogEntry, now time.Time, operationId string) *servicecontrol.ReportRequest {
	var ls []*servicecontrol.LogEntry
	for _, e := range entries {
		l := &servicecontrol.LogEntry{
			Name:        e.LogName,
			Severity:    e.Severity.String(),
			TextPayload: e.TextPayload,
			Timestamp:   e.Timestamp,
		}
		ls = append(ls, l)
	}

	op := &servicecontrol.Operation{
		OperationId:   operationId,
		OperationName: "reportLogs",
		StartTime:     now.Format(time.RFC3339),
		EndTime:       now.Format(time.RFC3339),
		LogEntries:    ls,
		Labels:        map[string]string{"cloud.googleapis.com/location": "global"},
	}

	return &servicecontrol.ReportRequest{
		Operations: []*servicecontrol.Operation{op},
	}
}

func (a *aspect) Log(entries []adapter.LogEntry) error {
	rq := a.log(entries, time.Now(), fmt.Sprintf("mixer-log-report-id-%d", uuid.New()))
	rp, err := a.service.Services.Report(a.serviceName, rq).Do()

	if a.logger.VerbosityLevel(4) {
		a.logger.Infof("service control log report for operation id %s\nlogs %v\nresponse %v",
			rq.Operations[0].OperationId, entries, rp)
	}
	return err
}

func (a *aspect) LogAccess(entries []adapter.LogEntry) error {
	//TODO access log will be merged into application log.
	return nil
}

func (a *aspect) Close() error { return nil }
