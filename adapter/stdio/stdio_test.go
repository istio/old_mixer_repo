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

package stdio

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"istio.io/mixer/adapter/stdio/config"
	"istio.io/mixer/pkg/adapter/test"
	"istio.io/mixer/template/logentry"
	"istio.io/mixer/template/metric"
)

func TestBasic(t *testing.T) {
	info := GetBuilderInfo()

	if !contains(info.SupportedTemplates, logentry.TemplateName) ||
		!contains(info.SupportedTemplates, metric.TemplateName) {
		t.Error("Didn't find all expected supported templates")
	}

	builder := info.CreateHandlerBuilder()
	cfg := info.DefaultConfig

	if err := info.ValidateConfig(cfg); err != nil {
		t.Errorf("Got error %v, expecting success", err)
	}

	logEntryBuilder := builder.(logentry.HandlerBuilder)
	if err := logEntryBuilder.ConfigureLogEntryHandler(nil); err != nil {
		t.Errorf("Got error %v, expecting success", err)
	}

	metricBuilder := builder.(metric.HandlerBuilder)
	if err := metricBuilder.ConfigureMetricHandler(nil); err != nil {
		t.Errorf("Got error %v, expecting success", err)
	}

	handler, err := builder.Build(cfg, test.NewEnv(t))
	if err != nil {
		t.Errorf("Got error %v, expecting success", err)
	}

	logEntryHandler := handler.(logentry.Handler)
	err = logEntryHandler.HandleLogEntry(context.Background(), nil)
	if err != nil {
		t.Errorf("Got error %v, expecting success", err)
	}

	metricHandler := handler.(metric.Handler)
	err = metricHandler.HandleMetric(context.Background(), nil)
	if err != nil {
		t.Errorf("Got error %v, expecting success", err)
	}

	if err = handler.Close(); err != nil {
		t.Errorf("Got error %v, expecting success", err)
	}
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func TestBuilder(t *testing.T) {
	info := GetBuilderInfo()
	env := test.NewEnv(t)

	cases := []struct {
		config      config.Params
		outputPath  string
		metricLevel zapcore.Level
		induceError bool
		success     bool
	}{
		{config.Params{
			LogStream: config.STDOUT,
		}, "stdout", zapcore.InfoLevel, false, true},

		{config.Params{
			LogStream: config.STDERR,
		}, "stderr", zapcore.InfoLevel, false, true},

		{config.Params{
			MetricLevel: config.INFO,
		}, "stdout", zapcore.InfoLevel, false, true},

		{config.Params{
			MetricLevel: config.WARNING,
		}, "stdout", zapcore.WarnLevel, false, true},

		{config.Params{
			MetricLevel: config.ERROR,
		}, "stdout", zapcore.ErrorLevel, false, true},

		{config.Params{
			MetricLevel: config.ERROR,
		}, "stdout", zapcore.ErrorLevel, true, false},

		{config.Params{
			SeverityLevels: map[string]config.Params_Level{"WARNING": config.WARNING},
		}, "stdout", zapcore.InfoLevel, false, true},
	}

	for i, c := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			builder := info.CreateHandlerBuilder().(builder)
			oldZapBuilder := builder.zapBuilder
			capturedOutputPath := ""
			builder.zapBuilder = func(outputPath string) (*zap.Logger, error) {
				capturedOutputPath = outputPath

				if c.induceError {
					return nil, errors.New("expected")
				}

				return oldZapBuilder(outputPath)
			}

			h, err := builder.Build(&c.config, env)

			if capturedOutputPath != c.outputPath {
				t.Errorf("Got output path %s, expecting %s", capturedOutputPath, c.outputPath)
			}

			if (err != nil) && c.success {
				t.Errorf("Got %v, expecting success", err)
			} else if (err == nil) && !c.success {
				t.Errorf("Got success, expecting failure")
			}

			if (h == nil) && c.success {
				t.Errorf("Got nil, expecting valid handler")
			} else if (h != nil) && !c.success {
				t.Errorf("Got a handler, expecting nil")
			}

			if h != nil {
				handler := h.(*handler)
				if handler.metricLevel != c.metricLevel {
					t.Errorf("Got metric level %v, expecting %v", handler.metricLevel, c.metricLevel)
				}
			}
		})
	}
}

type testZap struct {
	zapcore.Core
	enc          zapcore.Encoder
	count        int
	lines        []string
	errorOnWrite bool
}

func newTestZap(errOnWrite bool) *testZap {
	zapConfig := zapcore.EncoderConfig{
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeTime:     zapcore.EpochTimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
	}

	return &testZap{
		errorOnWrite: errOnWrite,
		enc:          zapcore.NewJSONEncoder(zapConfig),
	}
}

func (tz *testZap) Write(e zapcore.Entry, f []zapcore.Field) error {
	if tz.errorOnWrite {
		return errors.New("write error")
	}

	buf, err := tz.enc.EncodeEntry(e, f)
	if err != nil {
		return err
	}

	tz.count++
	tz.lines = append(tz.lines, strings.Trim(buf.String(), "\n"))
	return nil
}

func TestLogEntry(t *testing.T) {
	info := GetBuilderInfo()
	builder := info.CreateHandlerBuilder()
	cfg := info.DefaultConfig
	env := test.NewEnv(t)
	h, err := builder.Build(cfg, env)
	handler := h.(*handler)
	tz := newTestZap(false)
	handler.zapper = zap.New(tz)

	cases := []struct {
		instances  []*logentry.Instance
		failWrites bool
	}{
		{
			[]*logentry.Instance{
				&logentry.Instance{
					Name:     "Foo",
					Severity: "WARNING",
					Variables: map[string]interface{}{
						"Foo": "Bar",
					},
				},
			},
			false,
		},

		{
			[]*logentry.Instance{
				&logentry.Instance{
					Name:     "Foo",
					Severity: "WARNING",
					Variables: map[string]interface{}{
						"Foo": "Bar",
					},
				},
			},
			true,
		},
	}

	for i, c := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			tz.errorOnWrite = c.failWrites

			err := handler.HandleLogEntry(context.Background(), c.instances)
			if err != nil && !c.failWrites {
				t.Errorf("Got %v, expecting success", err)
			} else if err == nil && c.failWrites {
				t.Errorf("Got success, expected failure")
			}
		})
	}

	if err = handler.Close(); err != nil {
		t.Errorf("Got error %v, expecting success", err)
	}
}

func TestMetricEntry(t *testing.T) {
	info := GetBuilderInfo()
	builder := info.CreateHandlerBuilder()
	cfg := info.DefaultConfig
	env := test.NewEnv(t)
	h, err := builder.Build(cfg, env)
	handler := h.(*handler)
	tz := newTestZap(false)
	handler.zapper = zap.New(tz)

	cases := []struct {
		instances  []*metric.Instance
		failWrites bool
	}{
		{
			[]*metric.Instance{
				&metric.Instance{
					Name: "Foo",
					Dimensions: map[string]interface{}{
						"Foo": "Bar",
					},
				},
			},
			false,
		},

		{
			[]*metric.Instance{
				&metric.Instance{
					Name: "Foo",
					Dimensions: map[string]interface{}{
						"Foo": "Bar",
					},
				},
			},
			true,
		},
	}

	for i, c := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			tz.errorOnWrite = c.failWrites

			err := handler.HandleMetric(context.Background(), c.instances)
			if err != nil && !c.failWrites {
				t.Errorf("Got %v, expecting success", err)
			} else if err == nil && c.failWrites {
				t.Errorf("Got success, expected failure")
			}
		})
	}

	if err = handler.Close(); err != nil {
		t.Errorf("Got error %v, expecting success", err)
	}
}

/*
func TestLogger_Log(t *testing.T) {
	structPayload := map[string]interface{}{"val": 42, "obj": map[string]interface{}{"val": false}}

	noPayloadEntry := adapter.LogEntry{LogName: "istio_log", Labels: map[string]interface{}{}, Timestamp: "2017-Jan-09", Severity: adapter.Info}
	textPayloadEntry := adapter.LogEntry{LogName: "istio_log", TextPayload: "text payload", Timestamp: "2017-Jan-09", Severity: adapter.Info}
	jsonPayloadEntry := adapter.LogEntry{LogName: "istio_log", StructPayload: structPayload, Timestamp: "2017-Jan-09", Severity: adapter.Info}
	labelEntry := adapter.LogEntry{LogName: "istio_log", Labels: map[string]interface{}{"label": 42}, Timestamp: "2017-Jan-09", Severity: adapter.Info}

	omitAll := "{}"
	noOmitAll := `{"logName":"","timestamp":"","severity":"DEFAULT","labels":null,"textPayload":"","structPayload":null}`
	baseLog := `{"logName":"istio_log","timestamp":"2017-Jan-09","severity":"INFO"}`
	textPayloadLog := `{"logName":"istio_log","timestamp":"2017-Jan-09","severity":"INFO","textPayload":"text payload"}`
	jsonPayloadLog := `{"logName":"istio_log","timestamp":"2017-Jan-09","severity":"INFO","structPayload":{"obj":{"val":false},"val":42}}`
	labelLog := `{"logName":"istio_log","timestamp":"2017-Jan-09","severity":"INFO","labels":{"label":42}}`

	tests := []struct {
		name      string
		input     []adapter.LogEntry
		omitEmpty bool
		want      []string
	}{
		{"empty_array", []adapter.LogEntry{}, true, nil},
		{"empty_omit", []adapter.LogEntry{{}}, true, []string{omitAll}},
		{"empty_include", []adapter.LogEntry{{}}, false, []string{noOmitAll}},
		{"no_payload", []adapter.LogEntry{noPayloadEntry}, true, []string{baseLog}},
		{"text_payload", []adapter.LogEntry{textPayloadEntry}, true, []string{textPayloadLog}},
		{"json_payload", []adapter.LogEntry{jsonPayloadEntry}, true, []string{jsonPayloadLog}},
		{"labels", []adapter.LogEntry{labelEntry}, true, []string{labelLog}},
	}

	for _, v := range tests {
		t.Run(v.name, func(t *testing.T) {
			tc := newTestCore(false)
			l := &logger{omitEmpty: v.omitEmpty, impl: newTestLogger(tc)}
			if err := l.Log(v.input); err != nil {
				t.Errorf("Log(%v) => unexpected error: %v", v.input, err)
			}
			if !reflect.DeepEqual(tc.(*testCore).lines, v.want) {
				t.Errorf("Log(%v) => %v, want %s", v.input, tc.(*testCore).lines, v.want)
			}
		})
	}
}

func TestLogger_LogFailure(t *testing.T) {
	textPayloadEntry := adapter.LogEntry{LogName: "istio_log", TextPayload: "text payload", Timestamp: "2017-Jan-09", Severity: adapter.Info}
	cases := []struct {
		name string
		core zapcore.Core
	}{
		{"write_error", newTestCore(true)},
	}

	for _, v := range cases {
		t.Run(v.name, func(t *testing.T) {
			l := &logger{impl: zap.New(v.core)}
			if err := l.Log([]adapter.LogEntry{textPayloadEntry}); err == nil {
				t.Fatal("Log() should have produced error")
			}
		})
	}
}

var (
	defaultParams   = &config.Params{LogStream: config.STDERR}
	overridesParams = &config.Params{LogStream: config.STDOUT}
)

func newTestLogger(t zapcore.Core) *zap.Logger {
	return zap.New(t)
}

*/
