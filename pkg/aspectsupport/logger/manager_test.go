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

package logger

import (
	"errors"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/golang/protobuf/ptypes/struct"
	"istio.io/mixer/pkg/aspect"
	"istio.io/mixer/pkg/aspect/logger"
	"istio.io/mixer/pkg/aspectsupport"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/expr"

	jtpb "github.com/golang/protobuf/jsonpb/jsonpb_test_proto"
	configpb "istio.io/api/mixer/v1/config"
	aspectpb "istio.io/api/mixer/v1/config/aspect"
	dpb "istio.io/api/mixer/v1/config/descriptor"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m.Kind() != "istio/logger" {
		t.Error("Wrong kind of adapter")
	}
}

func TestManager_NewAspect(t *testing.T) {
	newAspectShouldSucceed := []aspectTestCase{
		{"empty", widget, &structpb.Struct{}},
		{"nil", widget, nil},
		{"override", widget, widgetStruct},
	}

	m := NewManager()

	for _, v := range newAspectShouldSucceed {
		c := aspectsupport.CombinedConfig{Adapter: &configpb.Adapter{Params: v.params}, Aspect: &configpb.Aspect{Inputs: map[string]string{}}}
		if _, err := m.NewAspect(&c, &testLogger{defaultCfg: v.defaultCfg}, testEnv{}); err != nil {
			t.Errorf("NewAspect(): should not have received error for %s (%v)", v.name, err)
		}
	}
}

func TestManager_NewAspectFailures(t *testing.T) {

	defaultCfg := &aspectsupport.CombinedConfig{
		Adapter: &configpb.Adapter{},
		Aspect:  &configpb.Aspect{},
	}

	var generic aspect.Adapter
	errLogger := &testLogger{defaultCfg: &structpb.Struct{}, errOnNewAspect: true}

	failureCases := []struct {
		cfg   *aspectsupport.CombinedConfig
		adptr aspect.Adapter
	}{
		{defaultCfg, generic},
		{defaultCfg, errLogger},
	}

	m := NewManager()
	for _, v := range failureCases {
		if _, err := m.NewAspect(v.cfg, v.adptr, testEnv{}); err == nil {
			t.Error("NewAspect(): expected error for bad adapter")
		}
	}
}

func TestExecutor_Execute(t *testing.T) {
	noPayloadDesc := dpb.LogEntryDescriptor{
		Name:       "test",
		Attributes: []string{"key"},
	}
	payloadDesc := dpb.LogEntryDescriptor{
		Name:             "test",
		Attributes:       []string{"key"},
		PayloadAttribute: "payload",
	}
	withInputsDesc := dpb.LogEntryDescriptor{
		Name:             "test",
		Attributes:       []string{"attr"},
		PayloadAttribute: "payload",
	}

	noDescriptorExec := &executor{"istio_log", []dpb.LogEntryDescriptor{}, map[string]string{}, nil}
	noPayloadExec := &executor{"istio_log", []dpb.LogEntryDescriptor{noPayloadDesc}, map[string]string{"attr": "val"}, nil}
	payloadExec := &executor{"istio_log", []dpb.LogEntryDescriptor{payloadDesc}, map[string]string{"attr": "val"}, nil}
	withInputsExec := &executor{"istio_log", []dpb.LogEntryDescriptor{withInputsDesc}, map[string]string{"attr": "val"}, nil}

	executeShouldSucceed := []executeTestCase{
		{"no descriptors", noDescriptorExec, &testBag{}, &testEvaluator{}, 0},
		{"no payload", noPayloadExec, &testBag{strs: map[string]string{"key": "value"}}, &testEvaluator{}, 1},
		{"payload not found", payloadExec, &testBag{strs: map[string]string{"key": "value"}}, &testEvaluator{}, 0},
		{"with payload", payloadExec, &testBag{strs: map[string]string{"key": "value", "payload": "test"}}, &testEvaluator{}, 1},
		{"with inputs", withInputsExec, &testBag{strs: map[string]string{"key": "value", "payload": "test"}}, &testEvaluator{}, 1},
	}

	for _, v := range executeShouldSucceed {
		l := &testLogger{}
		v.exec.aspect = l

		if _, err := v.exec.Execute(v.bag, v.mapper); err != nil {
			t.Errorf("Execute(): should not have received error for %s (%v)", v.name, err)
		}
		if l.entryCount != v.wantEntryCount {
			t.Errorf("Execute(): got %d entries, wanted %d", l.entryCount, v.wantEntryCount)
		}
	}
}

func TestExecutor_ExecuteFailures(t *testing.T) {

	desc := dpb.LogEntryDescriptor{
		Name:       "test",
		Attributes: []string{"key"},
	}

	errorExec := &executor{"istio_log", []dpb.LogEntryDescriptor{desc}, map[string]string{}, &testLogger{errOnLog: true}}

	executeShouldFail := []executeTestCase{
		{"log failure", errorExec, &testBag{strs: map[string]string{"key": "value"}}, &testEvaluator{}, 1},
	}

	for _, v := range executeShouldFail {
		if _, err := v.exec.Execute(v.bag, v.mapper); err == nil {
			t.Errorf("Execute(): should have received error for %s", v.name)
		}
	}
}

func TestManager_DefaultConfig(t *testing.T) {
	m := NewManager()
	o := m.DefaultConfig()
	if !proto.Equal(o, &aspectpb.LoggerConfig{LogName: "istio_log"}) {
		t.Errorf("DefaultConfig(): wanted empty proto, got %v", o)
	}
}

func TestManager_ValidateConfig(t *testing.T) {
	m := NewManager()
	if err := m.ValidateConfig(&empty.Empty{}); err != nil {
		t.Errorf("ValidateConfig(): unexpected error: %v", err)
	}
}

type (
	structMap      map[string]*structpb.Value
	aspectTestCase struct {
		name       string
		defaultCfg proto.Message
		params     *structpb.Struct
	}
	executeTestCase struct {
		name           string
		exec           *executor
		bag            attribute.Bag
		mapper         expr.Evaluator
		wantEntryCount int
	}
	testLogger struct {
		logger.Adapter
		logger.Aspect

		defaultCfg     proto.Message
		entryCount     int
		errOnNewAspect bool
		errOnLog       bool
	}
	testEvaluator struct {
		expr.Evaluator
	}
	testBag struct {
		attribute.Bag

		strs map[string]string
	}
	testEnv struct {
		aspect.Env
	}
)

var (
	widget       = &jtpb.Widget{Color: jtpb.Widget_RED.Enum(), RColor: []jtpb.Widget_Color{jtpb.Widget_GREEN}}
	widgetStruct = newStruct(structMap{"color": newStringVal("BLUE"), "simple": newStructVal(structMap{"o_bool": newBoolVal(false)})})
)

func (t *testLogger) NewAspect(e aspect.Env, m proto.Message) (logger.Aspect, error) {
	if t.errOnNewAspect {
		return nil, errors.New("new aspect error")
	}
	return t, nil
}
func (t *testLogger) DefaultConfig() proto.Message { return t.defaultCfg }
func (t *testLogger) Log([]logger.Entry) error {
	if t.errOnLog {
		return errors.New("log error")
	}
	t.entryCount++
	return nil
}
func (t *testLogger) Close() error { return nil }

func (t *testEvaluator) Eval(e string, bag attribute.Bag) (interface{}, error) {
	return e, nil
}

func (t *testBag) String(name string) (string, bool) {
	v, found := t.strs[name]
	return v, found
}

func newStringVal(s string) *structpb.Value {
	return &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: s}}
}

func newBoolVal(b bool) *structpb.Value {
	return &structpb.Value{Kind: &structpb.Value_BoolValue{BoolValue: b}}
}

func newStruct(fields map[string]*structpb.Value) *structpb.Struct {
	return &structpb.Struct{Fields: fields}
}

func newStructVal(fields map[string]*structpb.Value) *structpb.Value {
	return &structpb.Value{Kind: &structpb.Value_StructValue{StructValue: newStruct(fields)}}
}
