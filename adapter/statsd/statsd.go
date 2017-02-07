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

package statsd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"text/template"
	"time"

	"github.com/cactus/go-statsd-client/statsd"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/hashicorp/go-multierror"

	"istio.io/mixer/adapter/statsd/config"
	"istio.io/mixer/pkg/adapter"
)

const (
	defaultFlushBytes = 512
)

type (
	builder struct {
		adapter.DefaultBuilder
	}

	aspect struct {
		rate      float32
		client    statsd.Statter
		templates map[string]*template.Template // metric name -> template
	}
)

var (
	name        = "statsd"
	desc        = "Pushes statsd metrics"
	defaultConf = &config.Params{
		FlushDuration: &duration.Duration{Nanos: int32(300 * time.Millisecond)},
	}
)

// Register records the builders exposed by this adapter.
func Register(r adapter.Registrar) {
	r.RegisterMetricsBuilder(newBuilder())
}

func newBuilder() *builder {
	return &builder{adapter.NewDefaultBuilder(name, desc, defaultConf)}
}

func (b *builder) ValidateConfig(c adapter.AspectConfig) (ce *adapter.ConfigErrors) {
	params := c.(*config.Params)
	flushDuration, err := ptypes.Duration(params.FlushDuration)
	if err != nil {
		ce = ce.Append("FlushDuration", err)
	}
	if flushDuration < time.Duration(0) {
		ce = ce.Append("FlushDuration", fmt.Errorf("flush duration must be >= 0"))
	}
	if params.FlushBytes < 0 {
		ce = ce.Append("FlushBytes", fmt.Errorf("flush bytes must be >= 0"))
	}
	if params.SamplingRate < 0 {
		ce = ce.Append("SamplingRate", fmt.Errorf("sampling rate must be >= 0"))
	}
	if _, err := net.ResolveUDPAddr("udp", params.Address); err != nil {
		ce = ce.Append("Address", fmt.Errorf("could not resolve address '%s' with err: %s", params.Address, err))
	}
	for metricName, s := range params.MetricNameTemplateStrings {
		if _, err := template.New(metricName).Parse(s); err != nil {
			ce = ce.Append("MetricNameTemplateStrings", fmt.Errorf("failed to parse template '%s' for metric '%s' with err: %s", s, metricName, err))
		}
	}
	return
}

func (*builder) NewMetricsAspect(env adapter.Env, cfg adapter.AspectConfig, metrics []adapter.MetricDefinition) (adapter.MetricsAspect, error) {
	params := cfg.(*config.Params)

	flushBytes := int(params.FlushBytes)
	if flushBytes <= 0 {
		env.Logger().Infof("Got FlushBytes of '%d', defaulting to '%d'", flushBytes, defaultFlushBytes)
		// the statsd impl we use defaults to 1432 byte UDP packets when flushBytes <= 0; we want to default to 512 so we check ourselves.
		flushBytes = defaultFlushBytes
	}

	flushDuration, _ := ptypes.Duration(params.FlushDuration)
	client, _ := statsd.NewBufferedClient(params.Address, params.Prefix, flushDuration, flushBytes)

	templates := make(map[string]*template.Template)
	for metricName, s := range params.MetricNameTemplateStrings {
		def, found := findMetric(metrics, metricName)
		if !found {
			env.Logger().Infof("template registered for nonexistent metric '%s'", metricName)
			continue // we don't have a metric that corresponds to this template, skip processing it
		}

		t, _ := template.New(metricName).Parse(s)
		if err := t.Execute(ioutil.Discard, def.Labels); err != nil {
			env.Logger().Warningf(
				"skipping custom statsd metric name for metric '%s', could not satisfy template '%s' with labels '%v' with err: %s",
				metricName, s, def.Labels, err)
			continue
		}
		templates[metricName] = t
	}
	return &aspect{params.SamplingRate, client, templates}, nil
}

func (a *aspect) Record(values []adapter.Value) error {
	var result *multierror.Error
	for _, v := range values {
		if err := a.record(v); err != nil {
			result = multierror.Append(result, err)
		}
	}
	return result.ErrorOrNil()
}

func (a *aspect) record(value adapter.Value) error {
	name := value.Name
	if t, found := a.templates[value.Name]; found {
		buf := new(bytes.Buffer)
		if err := t.Execute(buf, value.Labels); err != nil {
			panic(fmt.Errorf("failed to create metric name with template '%s' and labels '%v'; got err: %s", t.Name(), value.Labels, err))
		}
		name = buf.String()
	}

	switch value.Kind {
	case adapter.Gauge:
		v, err := value.Int64()
		if err != nil {
			return fmt.Errorf("could not record gauge '%s' with err: %s", name, err)
		}
		return a.client.Gauge(name, v, a.rate)
	case adapter.Counter:
		v, err := value.Int64()
		if err != nil {
			return fmt.Errorf("could not record counter '%s' with err: %s", name, err)
		}
		return a.client.Inc(name, v, a.rate)
	default:
		return fmt.Errorf("unknown metric kind '%v'", value.Kind)
	}
}

func (a *aspect) Close() error { return a.client.Close() }

func findMetric(metrics []adapter.MetricDefinition, name string) (adapter.MetricDefinition, bool) {
	for _, m := range metrics {
		if m.Name == name {
			return m, true
		}
	}
	return adapter.MetricDefinition{}, false
}
