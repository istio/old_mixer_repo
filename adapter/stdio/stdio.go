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

package stdio // import "istio.io/mixer/adapter/stdio"

import (
	"context"
	"fmt"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"istio.io/mixer/adapter/stdio/config"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/template/logentry"
	"istio.io/mixer/template/metric"
)

type (
	zapBuilderFn func(outputPath string) (*zap.Logger, error)

	builder struct {
		zapBuilder zapBuilderFn // indirection to allow override in tests
	}

	handler struct {
		zapper         *zap.Logger
		severityLevels map[string]zapcore.Level
		metricLevel    zapcore.Level
		omitEmpty      bool
	}
)

// ensure our types implement the requisite interfaces
var _ logentry.HandlerBuilder = builder{}
var _ logentry.Handler = &handler{}
var _ metric.HandlerBuilder = builder{}
var _ metric.Handler = &handler{}

///////////////// Configuration Methods ///////////////

func (b builder) Build(cfg adapter.Config, _ adapter.Env) (adapter.Handler, error) {
	c := cfg.(*config.Params)

	outputPath := "stdout"
	if c.LogStream == config.STDERR {
		outputPath = "stderr"
	}

	zapLogger, err := b.zapBuilder(outputPath)
	if err != nil {
		return nil, fmt.Errorf("could not build logger: %v", err)
	}

	sl := make(map[string]zapcore.Level)
	for k, v := range c.SeverityLevels {
		sl[k] = mapConfigLevel(v)
	}

	return &handler{
		omitEmpty:      c.OmitEmptyFields,
		severityLevels: sl,
		metricLevel:    mapConfigLevel(c.MetricLevel),
		zapper:         zapLogger,
	}, nil
}

func mapConfigLevel(l config.Params_Level) zapcore.Level {
	if l == config.WARNING {
		return zapcore.WarnLevel
	} else if l == config.ERROR {
		return zapcore.ErrorLevel
	}
	return zapcore.InfoLevel
}

func (builder) ConfigureLogEntryHandler(map[string]*logentry.Type) error {
	return nil
}

func (builder) ConfigureMetricHandler(map[string]*metric.Type) error {
	return nil
}

func newZapLogger(outputPath string) (*zap.Logger, error) {
	prodConfig := zap.NewProductionConfig()
	prodConfig.DisableCaller = true
	prodConfig.DisableStacktrace = true
	prodConfig.OutputPaths = []string{outputPath}
	prodConfig.EncoderConfig = zapcore.EncoderConfig{
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
	}

	return prodConfig.Build()
}

////////////////// Runtime Methods //////////////////////////

func (h *handler) HandleLogEntry(_ context.Context, instances []*logentry.Instance) error {
	var errors *multierror.Error

	fields := make([]zapcore.Field, 0, 6)
	for _, instance := range instances {
		entry := zapcore.Entry{
			LoggerName: instance.Name,
			Level:      h.mapSeverityLevel(instance.Severity),

			// TODO: disabled in template
			// Time: instance.Timestemp
		}

		for k, v := range instance.Variables {
			if v != "" || !h.omitEmpty {
				fields = append(fields, zap.Any(k, v))
			}
		}

		if err := h.zapper.Core().Write(entry, fields); err != nil {
			errors = multierror.Append(errors, err)
		}
		fields = fields[:0]
	}

	return errors.ErrorOrNil()
}

func (h *handler) HandleMetric(_ context.Context, instances []*metric.Instance) error {
	var errors *multierror.Error

	fields := make([]zapcore.Field, 0, 6)
	for _, instance := range instances {
		entry := zapcore.Entry{
			LoggerName: instance.Name,
			Level:      h.metricLevel,
			Time:       time.Now(),
		}

		fields = append(fields, zap.Any("value", instance.Value))
		for k, v := range instance.Dimensions {
			if v != "" || !h.omitEmpty {
				fields = append(fields, zap.Any(k, v))
			}
		}

		if err := h.zapper.Core().Write(entry, fields); err != nil {
			errors = multierror.Append(errors, err)
		}
		fields = fields[:0]
	}

	return errors.ErrorOrNil()
}

func (h *handler) Close() error { return nil }

func (h *handler) mapSeverityLevel(severity string) zapcore.Level {
	level, ok := h.severityLevels[severity]
	if !ok {
		level = zap.InfoLevel
	}

	return level
}

////////////////// Bootstrap //////////////////////////

// GetBuilderInfo returns the BuilderInfo associated with this adapter implementation.
func GetBuilderInfo() adapter.BuilderInfo {
	return adapter.BuilderInfo{
		Name:        "istio.io/mixer/adapter/stdio",
		Description: "Writes logs and metrics to a standard I/O stream",
		SupportedTemplates: []string{
			logentry.TemplateName,
			metric.TemplateName,
		},
		DefaultConfig: &config.Params{
			OmitEmptyFields: false,
			LogStream:       config.STDOUT,
			MetricLevel:     config.INFO,
		},
		CreateHandlerBuilder: func() adapter.HandlerBuilder { return builder{newZapLogger} },
		ValidateConfig:       func(adapter.Config) *adapter.ConfigErrors { return nil },
	}
}
