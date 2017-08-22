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

package store

import (
	"reflect"
	"testing"

	"github.com/gogo/protobuf/proto"

	cfg "istio.io/mixer/pkg/config/proto"
)

func TestConvert(t *testing.T) {
	for _, tt := range []struct {
		title    string
		source   map[string]interface{}
		dest     proto.Message
		expected proto.Message
	}{
		{
			"base",
			map[string]interface{}{"name": "foo", "adapter": "a", "params": nil},
			&cfg.Handler{},
			&cfg.Handler{Name: "foo", Adapter: "a"},
		},
		{
			"empty",
			map[string]interface{}{},
			&cfg.Handler{},
			&cfg.Handler{},
		},
	} {
		if err := convert(tt.source, tt.dest); err != nil {
			t.Errorf("Failed to convert %s: %v", tt.title, err)
		}
		if !reflect.DeepEqual(tt.dest, tt.expected) {
			t.Errorf("%s: Got %+v, Want %+v", tt.title, tt.dest, tt.expected)
		}
	}
}

func TestConvertWithKind(t *testing.T) {
	kinds := map[string]proto.Message{
		"Handler": &cfg.Handler{},
		"Action":  &cfg.Action{},
	}
	for _, c := range []struct {
		title    string
		source   map[string]interface{}
		kind     string
		ok       bool
		expected proto.Message
	}{
		{
			"base",
			map[string]interface{}{"name": "default", "adapter": "noop"},
			"Handler",
			true,
			&cfg.Handler{Name: "default", Adapter: "noop"},
		},
		{
			"wrong kind",
			map[string]interface{}{"name": "default", "adapter": "noop"},
			"Action",
			false,
			nil,
		},
		{
			"unknown kind",
			map[string]interface{}{"name": "default", "adapter": "noop"},
			"foo",
			false,
			nil,
		},
	} {
		t.Run(c.title, func(tt *testing.T) {
			converted, err := convertWithKind(c.source, c.kind, kinds)
			if !c.ok {
				if err == nil {
					tt.Errorf("Expected to fail, succeeded")
				}
				return
			}
			if err != nil {
				tt.Errorf("Failed to convert: %v", err)
			}
			if !reflect.DeepEqual(converted, c.expected) {
				tt.Errorf("Got %+v Want %+v", converted, c.expected)
			}
		})
	}
}
