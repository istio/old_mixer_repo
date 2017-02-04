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
	"text/template"
	"time"

	"github.com/cactus/go-statsd-client/statsd"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/duration"

	"istio.io/mixer/adapter/statsd/config"
	"istio.io/mixer/pkg/adapter"
)

type (
	// builder holds a connection to the statsd server which is shared by all aspects, as well as the parsed
	// templates for constructing statsd metric names.
	builder struct {
		adapter.DefaultBuilder
		client statsd.Statter
	}

	aspect struct {
		rate      float32
		client    statsd.Statter
		templates map[string]*template.Template // metric name -> template
	}
)

var (
	name = "statsd"
	desc = "Pushes statsd metrics"
	// conf encodes the default configuration for this adapter
	conf = &config.Params{
		Address:                   "localhost:8125",
		Prefix:                    "",
		FlushDuration:             &duration.Duration{Seconds: 0, Nanos: int32(300 * time.Millisecond)},
		FlushBytes:                512,
		SamplingRate:              1.0,
		MetricNameTemplateStrings: make(map[string]string),
	}
)

// Register records the builders exposed by this adapter.
func Register(r adapter.Registrar) {
	r.RegisterMetricsBuilder(newBuilder())
}

func newBuilder() *builder {
	flushDuration, err := ptypes.Duration(conf.FlushDuration)
	if err != nil {
		// we panic because we're reading in our static default config
		panic(fmt.Errorf("failed to parse default duration, %v, as a time.Duration", conf.FlushDuration))
	}

	client, err := statsd.NewBufferedClient(conf.Address, conf.Prefix, flushDuration, int(conf.FlushBytes))
	if err != nil {
		// TODO: something other than panic?
		panic("failed to construct statsd client with err: " + err.Error())
	}
	return &builder{adapter.NewDefaultBuilder(name, desc, conf), client}
}

func (b *builder) ValidateConfig(c adapter.AspectConfig) (ce *adapter.ConfigErrors) {
	params := c.(*config.Params)
	for metricName, s := range params.MetricNameTemplateStrings {
		if _, err := template.New(metricName).Parse(s); err != nil {
			ce = ce.Append("MetricNameTemplateStrings", fmt.Errorf("failed to parse template '%s' for metric '%s' with err: %s", s, metricName, err))
		}
	}
	return
}

func (b *builder) Close() error {
	return b.client.Close()
}

func (b *builder) NewMetricsAspect(env adapter.Env, cfg adapter.AspectConfig, metrics []adapter.MetricDefinition) (adapter.MetricsAspect, error) {
	params := cfg.(*config.Params)
	templates := make(map[string]*template.Template)
	for metricName, s := range params.MetricNameTemplateStrings {
		def, found := findMetric(metrics, metricName)
		if !found {
			continue // we don't have a metric that corresponds to this template, keep on going
		}

		t, err := template.New(metricName).Parse(s)
		if err != nil {
			// we panic here because ValidateConfig should catch any issues before this method is called.
			panic(fmt.Errorf("failed to parse template '%s' for metric '%s' with err: %s", s, metricName, err))
		}

		if err := t.Execute(ioutil.Discard, def.Labels); err != nil {
			// TODO: alternative is to log a warning and keep processing. the result is that we get the 'wrong' statsd metric name
			// for this metric when exporting.
			return nil, fmt.Errorf("unable to satisfy template with labels for metric %s; err: %s", metricName, err)
		}
		templates[metricName] = t
	}
	return &aspect{params.SamplingRate, b.client, templates}, nil
}

func (a *aspect) Record(values []adapter.Value) error {
	for _, v := range values {
		if err := a.record(v); err != nil {
			// TODO: should we keep track of all of the errors and return a composite, rather than aborting at the first problem?
			return err
		}
	}
	return nil
}

func (a *aspect) record(value adapter.Value) error {
	name := value.Name
	if t, found := a.templates[value.Name]; found {
		buf := new(bytes.Buffer)
		if err := t.Execute(buf, value.Labels); err != nil {
			// TODO: we should be able to verify that this will never happen at config time, should this panic?
			return fmt.Errorf("failed to create metric name with template '%s' and labels '%v'; got err: %s", t.Name(), value.Labels, err)
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

func (*aspect) Close() error { return nil }

func findMetric(metrics []adapter.MetricDefinition, name string) (adapter.MetricDefinition, bool) {
	for _, m := range metrics {
		if m.Name == name {
			return m, true
		}
	}
	return adapter.MetricDefinition{}, false
}
