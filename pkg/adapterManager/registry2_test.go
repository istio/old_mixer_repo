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

package adapterManager

import (
	"errors"
	"reflect"
	"testing"

	"github.com/golang/protobuf/proto"

	"istio.io/mixer/pkg/adapter"
	sample_report "istio.io/mixer/pkg/templates/sample/report"
)

type testHandler struct {
	name string
}

func (t testHandler) Name() string                       { return t.name }
func (testHandler) Close() error                         { return nil }
func (testHandler) Description() string                  { return "mock adapter for testing" }
func (testHandler) DefaultConfig() proto.Message         { return nil }
func (testHandler) ValidateConfig(c proto.Message) error { return nil }
func (testHandler) Configure(m proto.Message) error      { return nil }

type sampleReportProcessingAdapter struct{ testHandler }
type sampleReportProcessingAdapter2 struct{ sampleReportProcessingAdapter }

func (sampleReportProcessingAdapter) ConfigureSample(typeParams map[string]*sample_report.Type) error {
	return errors.New("not implemented")
}

func (sampleReportProcessingAdapter) ReportSample(instances []*sample_report.Instance) error {
	return errors.New("not implemented")
}

func TestRegisterSampleProcessor(t *testing.T) {
	reg := newRegistry2(nil)
	sampleReportAdapter := sampleReportProcessingAdapter{}
	reg.RegisterSampleProcessor(sampleReportAdapter)

	handler, ok := reg.FindHandler(sampleReportAdapter.Name())
	if !ok {
		t.Errorf("No adapter by name %s, expected %v", sampleReportAdapter.Name(), sampleReportAdapter)
	}

	if sampleReport, ok := handler.(sample_report.SampleProcessor); !ok || sampleReport != sampleReportAdapter {
		t.Errorf("reg.ByImpl(%s) expected handler '%v', actual '%v'", sampleReportAdapter.Name(), sampleReportAdapter, handler)
	}
}

func TestCollisionSameNameAdapter(t *testing.T) {
	reg := newRegistry2(nil)
	name := "some name that they both have"

	a1 := sampleReportProcessingAdapter{testHandler{name}}
	reg.RegisterSampleProcessor(a1)

	if a, ok := reg.FindHandler(name); !ok || a != a1 {
		t.Errorf("Failed to get first adapter by impl name; expected: '%v', actual: '%v'", a1, a)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected to recover from panic registering duplicate adapter, but recover was nil.")
		}
	}()

	a2 := sampleReportProcessingAdapter2{sampleReportProcessingAdapter{testHandler{name}}}
	reg.RegisterSampleProcessor(a2)
	t.Error("Should not reach this statement due to panic.")
}

func TestMultiTemplateRegistration(t *testing.T) {
	sampleReportAdapter := sampleReportProcessingAdapter{}
	reg := newRegistry2([]adapter.RegisterFn2{
		func(r adapter.Registrar2) {
			r.RegisterSampleProcessor(sampleReportAdapter)
		},
	})

	handler, ok := reg.FindHandler(sampleReportAdapter.Name())
	if !ok {
		t.Errorf("No adapter by name %s, expected %v", sampleReportAdapter.Name(), sampleReportAdapter)
	}

	var sampleReport sampleReportProcessingAdapter
	if sampleReport, ok = handler.(sampleReportProcessingAdapter); !ok || sampleReport != sampleReportAdapter {
		t.Errorf("reg.ByImpl(%s) expected handler '%v', actual '%v'", sampleReportAdapter.Name(), sampleReportAdapter, handler)
	}

	// register as "foo.bar2" template processor
	expectedTemplateNames := []string{"istio.mixer.adapter.sample.report.Sample", "foo.bar2"}

	reg.insertHandler("foo.bar2", sampleReport)

	if !reflect.DeepEqual(reg.handlersByName[sampleReport.name].Templates, expectedTemplateNames) {
		t.Errorf("supported templates: got %s\nwant %s", reg.handlersByName[sampleReport.name].Templates, expectedTemplateNames)
	}

	// register again and should be no chang
	reg.RegisterSampleProcessor(sampleReportAdapter)
	if !reflect.DeepEqual(reg.handlersByName[sampleReport.name].Templates, expectedTemplateNames) {
		t.Errorf("supported templates: got %s\nwant %s", reg.handlersByName[sampleReport.name].Templates, expectedTemplateNames)
	}

	if _, ok = reg.FindHandler("DOES_NOT_EXIST"); ok {
		t.Error("Unexpectedly found adapter: DOES_NOT_EXIST")
	}
}

func TestHandlerMap(t *testing.T) {
	mp := HandlerMap([]adapter.RegisterFn2{
		func(r adapter.Registrar2) {
			r.RegisterSampleProcessor(sampleReportProcessingAdapter{testHandler{name: "foo"}})
		},
		func(r adapter.Registrar2) {
			r.RegisterSampleProcessor(sampleReportProcessingAdapter{testHandler{name: "bar"}})
		},
	})

	if _, found := mp["foo"]; !found {
		t.Error("got nil, want foo")
	}
	if _, found := mp["bar"]; !found {
		t.Error("got nil, want bar")
	}
}
