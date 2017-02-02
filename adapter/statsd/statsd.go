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
	"text/template"
	"time"

	"github.com/cactus/go-statsd-client/statsd"
	"github.com/golang/protobuf/ptypes/duration"

	"istio.io/mixer/adapter/statsd/config"
	"istio.io/mixer/pkg/adapter"
)

type (
	// builder holds a connection to the statsd server which is shared by all aspects, as well as the parsed
	// templates for constructing statsd metric names.
	builder struct {
		adapter.DefaultBuilder

		client    statsd.Statter
		templates map[string]*template.Template // metric name -> template
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
		FlushInterval:             &duration.Duration{Seconds: 0, Nanos: int32(300 * time.Millisecond)},
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
	client, err := statsd.NewBufferedClient(conf.Address, conf.Prefix, toTime(conf.FlushInterval), int(conf.FlushBytes))
	if err != nil {
		// TODO: something other than panic?
		panic("failed to construct statsd client with err: " + err.Error())
	}

	templates := make(map[string]*template.Template)
	for metricName, s := range conf.MetricNameTemplateStrings {
		if t, err := template.New(metricName).Parse(s); err != nil {
			// This should never happen given ValidateConfig, but we call this before validateconfig? unclear how this needs to work.
			// We could migrate this parsing into a sync.Once and called during NewMetrics, but then it happen during a request.
			panic(fmt.Errorf("failed to parse template '%s' for metric '%s' with err: %s", s, metricName, err))
		} else {
			templates[metricName] = t
		}
	}

	return &builder{adapter.NewDefaultBuilder(name, desc, conf), client, templates}
}

func (b *builder) ValidateConfig(c adapter.AspectConfig) (ce *adapter.ConfigErrors) {
	params, ok := c.(*config.Params)
	if !ok {
		// TODO: should this just report an error?
		panic(fmt.Errorf("invalid config type, want adapter/statsd/config/config.proto, got: %v", params))
	}

	// TODO: we need to validate that the provided template strings are satisfiable by the mixer's attributes/metric's labels
	for metricName, s := range params.MetricNameTemplateStrings {
		if _, err := template.New(metricName).Parse(s); err != nil {
			ce = ce.Append("template "+metricName, fmt.Errorf("failed to parse template '%s' for metric '%s' with err: %s", s, metricName, err))
		}
	}
	return
}

func (b *builder) Close() error {
	return b.client.Close()
}

func (b *builder) NewMetricsAspect(env adapter.Env, cfg adapter.AspectConfig, metrics []adapter.MetricDefinition) (adapter.MetricsAspect, error) {
	// TODO: should we verify that templates are satisfiable here? we have the labels from the definitions, but it's not clear other than
	// trying to fill in each template that we can satisfy them.

	params, ok := cfg.(*config.Params)
	if !ok {
		return nil, fmt.Errorf("invalid config, expected statsd.config.Params, got %+v", cfg)
	}
	return &aspect{params.SamplingRate, b.client, b.templates}, nil
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

func toTime(d *duration.Duration) time.Duration {
	return (time.Duration(d.Seconds) * time.Second) + (time.Duration(d.Nanos) * time.Nanosecond)
}
