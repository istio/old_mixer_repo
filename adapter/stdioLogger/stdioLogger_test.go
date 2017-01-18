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

package stdioLogger

import (
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"istio.io/mixer/adapter/stdioLogger/config"
	"istio.io/mixer/pkg/adaptertesting"
	"istio.io/mixer/pkg/aspect"
	"istio.io/mixer/pkg/aspect/logger"
)

func TestAdapterInvariants(t *testing.T) {
	adaptertesting.TestAdapterInvariants(&adapter{}, Register, t)
}

func TestAdapter_NewAspect(t *testing.T) {
	tests := []newAspectTests{
		{&config.Params{}, defaultAspectImpl},
		{defaultParams, defaultAspectImpl},
		{overridesParams, overridesAspectImpl},
	}

	e := testEnv{}
	a := &adapter{}
	for _, v := range tests {
		asp, err := a.NewAspect(e, v.config)
		if err != nil {
			t.Errorf("NewAspect(env, %s) => unexpected error: %v", v.config, err)
		}
		got := asp.(*aspectImpl)
		if !reflect.DeepEqual(got, v.want) {
			t.Errorf("NewAspect(env, %s) => %v, want %v", v.config, got, v.want)
		}
	}
}

func TestAspectImpl_Close(t *testing.T) {
	a := &aspectImpl{}
	if err := a.Close(); err != nil {
		t.Errorf("Close() => unexpected error: %v", err)
	}
}

func TestAspectImpl_Log(t *testing.T) {

	tw := &testWriter{lines: make([]string, 0)}

	stamp, _ := time.Parse("2006-Jan-02", "2017-Jan-09")
	jan10, _ := time.Parse("2006-Jan-02", "2017-Jan-10")

	structPayload := map[string]interface{}{"val": 42, "obj": map[string]interface{}{"val": false}}

	textPayloadEntry := logger.Entry{LogName: "istio_log", TextPayload: "text payload", Timestamp: stamp, Severity: logger.Info}
	jsonPayloadEntry := logger.Entry{LogName: "istio_log", StructPayload: structPayload, Timestamp: stamp, Severity: logger.Info}
	labelEntry := logger.Entry{LogName: "istio_log", Labels: map[string]interface{}{"label": 42}, Timestamp: stamp, Severity: logger.Info}
	timeOverrideEntry := logger.Entry{LogName: "istio_log", Labels: map[string]interface{}{"label": 42}, Timestamp: jan10, Severity: logger.Info}

	baseLog := `{"timestamp":"2017-01-09T00:00:00Z","logName":"istio_log","severity":"INFO"}`
	textPayloadLog := `{"timestamp":"2017-01-09T00:00:00Z","logName":"istio_log","severity":"INFO","textPayload":"text payload"}`
	jsonPayloadLog := `{"timestamp":"2017-01-09T00:00:00Z","logName":"istio_log","severity":"INFO","structPayload":{"obj":{"val":false},"val":42}}`
	labelLog := `{"timestamp":"2017-01-09T00:00:00Z","logName":"istio_log","labels":{"label":42},"severity":"INFO"}`
	timestampLog := `{"timestamp":"2017-Jan-10","logName":"istio_log","labels":{"label":42},"severity":"INFO"}`

	baseAspectImpl := &aspectImpl{tw, time.RFC3339}
	timestampAspectImpl := &aspectImpl{tw, "2006-Jan-02"}

	tests := []logTests{
		{baseAspectImpl, []logger.Entry{}, []string{}},
		{baseAspectImpl, []logger.Entry{{LogName: "istio_log", Labels: map[string]interface{}{}, Timestamp: stamp, Severity: logger.Info}}, []string{baseLog}},
		{baseAspectImpl, []logger.Entry{textPayloadEntry}, []string{textPayloadLog}},
		{baseAspectImpl, []logger.Entry{jsonPayloadEntry}, []string{jsonPayloadLog}},
		{baseAspectImpl, []logger.Entry{labelEntry}, []string{labelLog}},
		{timestampAspectImpl, []logger.Entry{timeOverrideEntry}, []string{timestampLog}},
	}

	for _, v := range tests {
		if err := v.asp.Log(v.input); err != nil {
			t.Errorf("Log(%v) => unexpected error: %v", v.input, err)
		}
		if !reflect.DeepEqual(tw.lines, v.want) {
			t.Errorf("Log(%v) => %v, want %s", v.input, tw.lines, v.want)
		}
		tw.lines = make([]string, 0)
	}
}

type (
	testEnv struct {
		aspect.Env
	}
	newAspectTests struct {
		config *config.Params
		want   *aspectImpl
	}
	logTests struct {
		asp   *aspectImpl
		input []logger.Entry
		want  []string
	}
	testWriter struct {
		io.Writer

		count        int
		lines        []string
		errorOnWrite bool
	}
)

var (
	defaultParams = &config.Params{
		LogStream: config.Params_STDERR,
	}
	defaultAspectImpl = &aspectImpl{os.Stderr, time.RFC3339}

	overridesParams = &config.Params{
		LogStream:       config.Params_STDOUT,
		TimestampFormat: "2006-Jan-02",
	}
	overridesAspectImpl = &aspectImpl{os.Stdout, "2006-Jan-02"}
)

func (t *testWriter) Write(p []byte) (n int, err error) {
	if t.errorOnWrite {
		return 0, errors.New("write error")
	}
	t.count++
	t.lines = append(t.lines, strings.Trim(string(p), "\n"))
	return len(p), nil
}
