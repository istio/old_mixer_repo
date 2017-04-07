// Copyright 2017 the Istio Authors.
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

package aspect

import (
	"errors"
	"testing"

	dpb "istio.io/api/mixer/v1/config/descriptor"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/adapter/test"
	apb "istio.io/mixer/pkg/aspect/config"
	atest "istio.io/mixer/pkg/aspect/test"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/config"
	cpb "istio.io/mixer/pkg/config/proto"
	"istio.io/mixer/pkg/expr"
	"istio.io/mixer/pkg/status"
)

func TestAttributeGeneratorManager(t *testing.T) {
	m := newAttrGenMgr()
	if m.Kind() != config.AttributesKind {
		t.Errorf("m.Kind() = %s; wanted %s", m.Kind(), config.AttributesKindName)
	}
	if err := m.ValidateConfig(m.DefaultConfig(), expr.NewCEXLEvaluator(), nil); err != nil {
		t.Errorf("ValidateConfig(DefaultConfig()) produced an error: %v", err)
	}
	if err := m.ValidateConfig(&apb.AttributeGeneratorsParams{}, expr.NewCEXLEvaluator(), df); err != nil {
		t.Error("ValidateConfig(AttributeGeneratorsParams{}) should not produce an error.")
	}
}

func TestAttrGenMgr_ValidateConfig(t *testing.T) {

	dfind := atest.NewDescriptorFinder(map[string]interface{}{
		"source_ip": &dpb.AttributeDescriptor{Name: "source_ip"},
	})

	newAttrs := &apb.AttributeGeneratorsParams{
		GeneratedAttributes: []*dpb.AttributeDescriptor{
			{Name: "no_conflict"},
		},
	}

	existingAttrs := &apb.AttributeGeneratorsParams{
		GeneratedAttributes: []*dpb.AttributeDescriptor{
			{Name: "source_ip"},
		},
	}

	m := newAttrGenMgr()
	if err := m.ValidateConfig(newAttrs, expr.NewCEXLEvaluator(), dfind); err != nil {
		t.Errorf("Unexpected error '%v' for config: %v", err, newAttrs)
	}

	if err := m.ValidateConfig(existingAttrs, expr.NewCEXLEvaluator(), dfind); err == nil {
		t.Errorf("Expected error for config: %#v", existingAttrs)
	}
}

type testAttrGen struct {
	adapter.AttributesGenerator

	out       map[string]interface{}
	closed    bool
	returnErr bool
}

type testAttrGenBuilder struct {
	adapter.DefaultBuilder
	returnErr bool
}

func newTestAttrGenBuilder(returnErr bool) testAttrGenBuilder {
	return testAttrGenBuilder{adapter.NewDefaultBuilder("test", "test", nil), returnErr}
}

func (t testAttrGenBuilder) BuildAttributesGenerator(env adapter.Env, c adapter.Config) (adapter.AttributesGenerator, error) {
	if t.returnErr {
		return nil, errors.New("error")
	}
	return &testAttrGen{}, nil
}

func TestAttributeGeneratorManager_NewPreprocessExecutor(t *testing.T) {
	tests := []struct {
		name    string
		builder adapter.Builder
		wantErr bool
	}{
		{"no error", newTestAttrGenBuilder(false), false},
		{"build error", newTestAttrGenBuilder(true), true},
	}

	m := newAttrGenMgr()
	c := &cpb.Combined{
		Builder: &cpb.Adapter{Params: &apb.AttributeGeneratorsParams{}},
		Aspect:  &cpb.Aspect{Params: &apb.AttributeGeneratorsParams{}},
	}

	for _, v := range tests {
		t.Run(v.name, func(t *testing.T) {
			_, err := m.NewPreprocessExecutor(c, v.builder, test.NewEnv(t), nil)
			if err == nil && v.wantErr {
				t.Error("Expected to receive error")
			}
			if err != nil && !v.wantErr {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func (t testAttrGen) Generate(in map[string]interface{}) (map[string]interface{}, error) {
	if t.returnErr {
		return nil, errors.New("generate error")
	}
	return t.out, nil
}

func TestAttributeGeneratorExecutor_Execute(t *testing.T) {

	genParams := &apb.AttributeGeneratorsParams{
		Labels: map[string]string{
			"input": "source_ip",
		},
		GeneratedAttributes: []*dpb.AttributeDescriptor{
			{Name: "found"},
			{Name: "source_service"},
		},
	}

	inBag := attribute.GetMutableBag(nil)
	inBag.Set("source_ip", "10.1.1.10")

	outMap := map[string]interface{}{
		"found":          true,
		"source_service": "service1",
	}
	outBag := attribute.GetMutableBag(nil)
	for k, v := range outMap {
		outBag.Set(k, v)
	}

	extraOutMap := map[string]interface{}{
		"found":              true,
		"source_service":     "service1",
		"should_be_stripped": "never_used",
	}

	tests := []struct {
		name      string
		exec      attrGenExec
		attrs     attribute.Bag
		eval      expr.Evaluator
		wantAttrs attribute.Bag
		wantErr   bool
	}{
		{"no error", attrGenExec{&testAttrGen{out: outMap}, genParams}, inBag, atest.NewIDEval(), outBag, false},
		{"strippped attrs", attrGenExec{&testAttrGen{out: extraOutMap}, genParams}, inBag, atest.NewIDEval(), outBag, false},
		{"generate error", attrGenExec{&testAttrGen{out: outMap, returnErr: true}, genParams}, inBag, atest.NewIDEval(), outBag, true},
		{"eval error", attrGenExec{&testAttrGen{}, genParams}, inBag, atest.NewErrEval(), outBag, true},
	}

	for _, v := range tests {
		t.Run(v.name, func(t *testing.T) {
			got, s := v.exec.Execute(v.attrs, v.eval)
			if status.IsOK(s) && v.wantErr {
				t.Fatal("Expected to receive error")
			}
			if !status.IsOK(s) {
				if !v.wantErr {
					t.Fatalf("Unexpected status returned: %v", s)
				}
				return
			}
			for _, n := range v.wantAttrs.Names() {
				wantVal, _ := v.wantAttrs.Get(n)
				gotVal, ok := got.Attrs.Get(n)
				if !ok {
					t.Errorf("Generated attribute.Bag missing attribute %s", n)
				}
				if gotVal != wantVal {
					t.Errorf("For attribute '%s': got value %v, want %v", n, gotVal, wantVal)
				}
			}
		})
	}
}

func (t *testAttrGen) Close() error {
	t.closed = true
	return nil
}

func TestAttributeGeneratorExecutor_Close(t *testing.T) {
	inner := &testAttrGen{closed: false}
	executor := &attrGenExec{aspect: inner}
	if err := executor.Close(); err != nil {
		t.Errorf("Close() returned an error: %v", err)
	}
	if !inner.closed {
		t.Error("Close() should propagate to wrapped aspect.")
	}
}
