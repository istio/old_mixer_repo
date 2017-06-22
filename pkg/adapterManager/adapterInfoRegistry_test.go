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
	//"reflect"
	"fmt"
	"reflect"
	"testing"

	"github.com/golang/protobuf/proto"

	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/adapter/config"
	sample_report "istio.io/mixer/pkg/templates/sample/report"
)

type TestAdapterInfoInventory struct {
	name string
}

func (t *TestAdapterInfoInventory) GetAdapterInfo2() adapter.BuilderInfo {
	return adapter.BuilderInfo{
		Name:                   t.name,
		Description:            "mock adapter for testing",
		CreateHandlerBuilderFn: func() config.HandlerBuilder { return testHandlerBuilder{} },
		SupportedTemplates:     []adapter.SupportedTemplates{adapter.SampleProcessorTemplate},
	}
}

type testHandlerBuilder struct{}

func (testHandlerBuilder) DefaultConfig() proto.Message                                    { return nil }
func (testHandlerBuilder) ValidateConfig(c proto.Message) error                            { return nil }
func (testHandlerBuilder) ConfigureSample(typeParams map[string]*sample_report.Type) error { return nil }
func (testHandlerBuilder) Build(cnfg proto.Message) (config.Handler, error)                { return testHandler{}, nil }

type testHandler struct{}

func (testHandler) Close() error { return nil }
func (testHandler) ReportSample(instances []*sample_report.Instance) error {
	return errors.New("not implemented")
}

func TestRegisterSampleProcessor(t *testing.T) {
	var a *sample_report.SampleProcessorBuilder
	fmt.Println(reflect.TypeOf(a).Elem())

	testAdapterInfoInventory := TestAdapterInfoInventory{"foo"}
	reg := newRegistry2([]adapter.GetAdapterInfoFn{testAdapterInfoInventory.GetAdapterInfo2}, DoesBuilderSupportsTemplate)

	adapterInfo, ok := reg.FindAdapterInfo(testAdapterInfoInventory.name)
	if !ok {
		t.Errorf("No adapterInfo by name %s, expected %v", testAdapterInfoInventory.name, testAdapterInfoInventory)
	}

	testAdapterInfoObj := testAdapterInfoInventory.GetAdapterInfo2()
	if testAdapterInfoObj.Name != adapterInfo.Name {
		t.Errorf("reg.FindAdapterInfo(%s) expected adapterInfo '%v', actual '%v'", testAdapterInfoObj.Name, testAdapterInfoObj, adapterInfo)
	}
}

func TestCollisionSameNameAdapter(t *testing.T) {
	testAdapterInfoInventory := TestAdapterInfoInventory{"some name that they both have"}
	testAdapterInfoInventory2 := TestAdapterInfoInventory{"some name that they both have"}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected to recover from panic registering duplicate adapter, but recover was nil.")
		}
	}()

	_ = newRegistry2([]adapter.GetAdapterInfoFn{
		testAdapterInfoInventory.GetAdapterInfo2,
		testAdapterInfoInventory2.GetAdapterInfo2}, DoesBuilderSupportsTemplate,
	)

	t.Error("Should not reach this statement due to panic.")
}

func TestHandlerMap(t *testing.T) {
	testAdapterInfoInventory := TestAdapterInfoInventory{"foo"}
	testAdapterInfoInventory2 := TestAdapterInfoInventory{"bar"}

	mp := AdapterInfoMap([]adapter.GetAdapterInfoFn{
		testAdapterInfoInventory.GetAdapterInfo2,
		testAdapterInfoInventory2.GetAdapterInfo2,
	}, DoesBuilderSupportsTemplate)

	if _, found := mp["foo"]; !found {
		t.Error("got nil, want foo")
	}
	if _, found := mp["bar"]; !found {
		t.Error("got nil, want bar")
	}
}

type badHandlerBuilder struct{}

func (badHandlerBuilder) DefaultConfig() proto.Message         { return nil }
func (badHandlerBuilder) ValidateConfig(c proto.Message) error { return nil }

// This misspelled function cause the Builder to not implement SampleProcessorBuilder
func (testHandlerBuilder) MisspelledXXConfigureSample(typeParams map[string]*sample_report.Type) error {
	return nil
}
func (badHandlerBuilder) Build(cnfg proto.Message) (config.Handler, error) { return testHandler{}, nil }

func TestBuilderNotImplementRightTemplateInterface(t *testing.T) {
	badHandlerBuilderAdapterInfo1 := func() adapter.BuilderInfo {
		return adapter.BuilderInfo{
			Name:                   "badAdapter1",
			Description:            "mock adapter for testing",
			CreateHandlerBuilderFn: func() config.HandlerBuilder { return badHandlerBuilder{} },
			SupportedTemplates:     []adapter.SupportedTemplates{adapter.SampleProcessorTemplate},
		}
	}
	badHandlerBuilderAdapterInfo2 := func() adapter.BuilderInfo {
		return adapter.BuilderInfo{
			Name:                   "badAdapter1",
			Description:            "mock adapter for testing",
			CreateHandlerBuilderFn: func() config.HandlerBuilder { return badHandlerBuilder{} },
			SupportedTemplates:     []adapter.SupportedTemplates{adapter.SampleProcessorTemplate},
		}
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected to recover from panic registering bad builder that does not implement Builders " +
				"for all supported templates, but recover was nil.")
		}
	}()

	_ = newRegistry2([]adapter.GetAdapterInfoFn{
		badHandlerBuilderAdapterInfo1, badHandlerBuilderAdapterInfo2}, DoesBuilderSupportsTemplate,
	)

	t.Error("Should not reach this statement due to panic.")
}
