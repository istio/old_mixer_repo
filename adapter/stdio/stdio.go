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

// Package stdioLogger provides an implementation of Mixer's logger aspect that
// writes logs (serialized as JSON) to a standard stream (stdout | stderr).
package stdio

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	multierror "github.com/hashicorp/go-multierror"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"istio.io/mixer/adapter/stdio/config"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/template/logentry"
	"istio.io/mixer/template/metric"
)

type (
	builder struct{}

	handler struct {
		omitEmpty      bool
		severityLevels map[string]zapcore.Level
		metricLevel    zapcore.Level
		zapper         *zap.Logger
	}

	zapBuilderFn func(outputPaths ...string) (*zap.Logger, error)
)

const (
	stdErr = "stderr"
	stdOut = "stdout"
)

// ensure our types implement the requisite interfaces
var _ logentry.HandlerBuilder = builder{}
var _ logentry.Handler = &handler{}
var _ metric.HandlerBuilder = builder{}
var _ metric.Handler = &handler{}

///////////////// Configuration Methods ///////////////

func (builder) Build(cfg proto.Message, _ adapter.Env) (adapter.Handler, error) {
	c := cfg.(*config.Params)

	outputPath := stdErr
	if c.LogStream == config.STDOUT {
		outputPath = stdOut
	}

	buildZap := newZapLogger

	zapLogger, err := buildZap(outputPath)
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

func newZapLogger(outputPaths ...string) (*zap.Logger, error) {
	prodConfig := zap.NewProductionConfig()
	prodConfig.DisableCaller = true
	prodConfig.DisableStacktrace = true
	prodConfig.OutputPaths = outputPaths
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
			if v == "" && h.omitEmpty {
				continue
			}
			fields = append(fields, zap.Any(k, v))
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
			if v == "" && h.omitEmpty {
				continue
			}
			fields = append(fields, zap.Any(k, v))
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
		DefaultConfig:        &config.Params{},
		CreateHandlerBuilder: func() adapter.HandlerBuilder { return builder{} },
		ValidateConfig:       func(msg proto.Message) *adapter.ConfigErrors { return nil },
	}
}
