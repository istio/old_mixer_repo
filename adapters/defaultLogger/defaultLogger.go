// Copyright 2016 Google Inc.
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

package defaultLogger

import (
	"errors"
	"os"
	"text/template"

	"istio.io/mixer"
	"istio.io/mixer/adapters"
)

const (
	name = "istio.io/mixer/loggers/defaultLogger"
	desc = "Creates access log entries from Report() requests and flushes to os.Stdout"
	tmpl = "defaultLogger"
)

type (
	builder struct{}
	adapter struct {
		t *template.Template
	}

	// Config represents the adapter configuration for the default logger.
	Config struct {
		// Template is used to pass a golang text/template string to the
		// adapter for translation of fact values into a log entry.
		Template string
	}

	// ErrInvalidConfig is used to report issues with adapter config validation
	ErrInvalidConfig = errors.New("invalid adapter config")
)

var (
	log *adapter
)

// NewBuilder returns the builder for the default logging adapter.
func NewBuilder() adapters.Builder {
	return &builder{}
}

func (b *builder) Close() error { return nil }

func (b builder) Name() string { return name }

func (b builder) Description() string { return desc }

func (b builder) DefaultBuilderConfig() adapters.BuilderConfig { return &struct{}{} }

func (b builder) ValidateBuilderConfig(c adapters.BuilderConfig) error { return nil }

func (b builder) Configure(c adapters.BuilderConfig) error { return nil }

func (b builder) DefaultAdapterConfig() adapters.AdapterConfig {
	return &Config{Template: ""}
}

func (b builder) ValidateAdapterConfig(c adapters.AdapterConfig) error {
	vc, ok := c.(Config)
	if !ok {
		return ErrInvalidConfig
	}

	_, err := template.New(tmpl).Parse(vc.Template)
	return err
}

func (b builder) NewAdapter(c adapters.AdapterConfig) (adapters.Adapter, error) {
	if err := b.ValidateAdapterConfig(c); err != nil {
		return nil, err
	}

	return newLogger(c.(Config))
}

func newLogger(c Config) (adapters.Logger, error) {
	t, err := template.New(tmpl).Parse(c.Template)
	if err != nil {
		return nil, err
	}

	return &adapter{t}, nil
}

func (l *adapter) Log(reports []mixer.FactValues) error {
	var logsErr error
	m := make(map[string]interface{})
	for _, fv := range reports {
		for k, v := range fv.BoolValues {
			m[k] = v
		}
		for k, v := range fv.StringValues {
			m[k] = v
		}
		for k, v := range fv.IntValues {
			m[k] = v
		}
		for k, v := range fv.FloatValues {
			m[k] = v
		}
		if err := l.t.Execute(os.Stdout, m); err != nil {
			logsErr = err
			continue
		}
		os.Stdout.Write([]byte{'\n'})
	}
	return logsErr
}

func (l *adapter) Flush() {}

func (l *adapter) Close() error { return nil }
