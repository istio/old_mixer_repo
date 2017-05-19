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
	"fmt"
	"reflect"
	"strings"
	"testing"

	dpb "istio.io/api/mixer/v1/config/descriptor"
	"istio.io/mixer/pkg/adapter"
	aconfig "istio.io/mixer/pkg/aspect/config"
	"istio.io/mixer/pkg/aspect/test"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/config/descriptor"
	cfgpb "istio.io/mixer/pkg/config/proto"
	"istio.io/mixer/pkg/expr"
)

var params = func(resources ...*aconfig.MonitoredResources_MonitoredResource) *aconfig.MonitoredResources {
	return &aconfig.MonitoredResources{Resources: resources}
}

func TestMRFind(t *testing.T) {
	eval := test.NewFakeEval(func(expr string, _ attribute.Bag) (interface{}, error) {
		if expr == "invalid" {
			return nil, errors.New("expected")
		}
		return expr, nil
	})

	tests := []struct {
		name   string
		params *aconfig.MonitoredResources
		out    *adapter.MonitoredResource
		err    string
	}{
		{"empty", params(), nil, "could not find"},
		{"valid", params(&aconfig.MonitoredResources_MonitoredResource{
			DescriptorName: "valid",
			NameExpression: "name",
			Labels:         map[string]string{"1": "label1val"},
		}), &adapter.MonitoredResource{
			Name:   "name",
			Labels: map[string]interface{}{"1": "label1val"},
		}, ""},
		{"invalid-name", params(&aconfig.MonitoredResources_MonitoredResource{
			DescriptorName: "invalid-name",
			NameExpression: "invalid",
			Labels:         map[string]string{"1": "label1val"},
		}), nil, "expected"},
		{"invalid-labels", params(&aconfig.MonitoredResources_MonitoredResource{
			DescriptorName: "invalid-labels",
			NameExpression: "name",
			Labels:         map[string]string{"1": "invalid"},
		}), nil, "expected"},
	}

	for idx, tt := range tests {
		t.Run(fmt.Sprintf("[%d] %s", idx, tt.name), func(t *testing.T) {
			finder := NewMRFinder(tt.params)
			if res, err := finder.Find(tt.name, test.NewBag(), eval); err != nil || tt.err != "" {
				if tt.err == "" {
					t.Fatalf("Find(%s, ...) = '%s', wanted no err", tt.name, err.Error())
				} else if err == nil {
					t.Fatalf("Find(%s, ...) = nil, wanted %s", tt.name, tt.err)
				} else if !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("Expected errors containing the string '%s', actual: '%s'", tt.err, err.Error())
				}
			} else if !reflect.DeepEqual(res, tt.out) {
				t.Fatalf("Find(%s, ...) = %v, wanted %v", tt.name, res, tt.out)
			}
		})
	}
}

func TestValidateMRConfig(t *testing.T) {
	f := test.NewDescriptorFinder(map[string]interface{}{
		"desc": &dpb.MonitoredResourceDescriptor{
			Name: "desc",
			Labels: map[string]dpb.ValueType{
				"str": dpb.STRING,
				"i64": dpb.INT64,
			},
		},
		"string": &cfgpb.AttributeManifest_AttributeInfo{ValueType: dpb.STRING},
		"int64":  &cfgpb.AttributeManifest_AttributeInfo{ValueType: dpb.INT64},
	})
	v := expr.NewCEXLEvaluator()

	tests := []struct {
		name string
		cfg  *aconfig.MonitoredResources
		v    expr.Validator
		df   descriptor.Finder
		err  string
	}{
		{"empty config", params(), v, f, ""},
		{"valid", params(&aconfig.MonitoredResources_MonitoredResource{
			DescriptorName: "desc",
			NameExpression: "string",
			Labels:         map[string]string{"str": "string", "i64": "int64"},
		}), v, f, ""},
		{"invalid name", params(&aconfig.MonitoredResources_MonitoredResource{
			DescriptorName: "desc",
			NameExpression: "name",
			Labels:         map[string]string{"str": "string", "i64": "int64"},
		}), v, f, "MonitoredResources.Resources[0].NameExpression"},
		{"no desc", params(&aconfig.MonitoredResources_MonitoredResource{
			DescriptorName: "missing",
			NameExpression: "string",
			Labels:         map[string]string{"str": "string", "i64": "int64"},
		}), v, f, "MonitoredResources.Resources[0].DescriptorName"},
		{"bad label", params(&aconfig.MonitoredResources_MonitoredResource{
			DescriptorName: "desc",
			NameExpression: "string",
			Labels:         map[string]string{"str": "missing", "i64": "int64"},
		}), v, f, "MonitoredResources.Resources[0].Labels"},
	}
	for idx, tt := range tests {
		t.Run(fmt.Sprintf("[%d] %s", idx, tt.name), func(t *testing.T) {
			errs := (&mrProvider{}).ValidateConfig(tt.cfg, tt.v, tt.df)
			if errs != nil || tt.err != "" {
				if tt.err == "" {
					t.Fatalf("ValidateConfig(tt.cfg, tt.v, tt.df) = '%s', wanted no err", errs.Error())
				} else if !strings.Contains(errs.Error(), tt.err) {
					t.Fatalf("Expected errors containing the string '%s', actual: '%s'", tt.err, errs.Error())
				}
			}

		})
	}
}
