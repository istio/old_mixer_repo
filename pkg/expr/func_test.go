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

package expr

import (
	"reflect"
	"testing"

	config "istio.io/api/mixer/v1/config/descriptor"
)

func TestIndexFunc(t *testing.T) {
	fn := newIndex()
	mp := map[string]interface{}{
		"X-FORWARDED-HOST": "aaa",
		"X-request-size":   2000,
	}
	tbl := []struct {
		key  interface{}
		want interface{}
	}{
		{"Does not Exists", nil},
		{"X-FORWARDED-HOST", "aaa"},
		{"X-request-size", 2000},
	}

	for idx, tt := range tbl {
		rv := fn.Call([]interface{}{mp, tt.key})
		if rv != tt.want {
			t.Errorf("[%d] got %#v\nwant %#v", idx, rv, tt.want)
		}
	}

	check(t, "ReturnType", fn.ReturnType(), config.VALUE_TYPE_UNSPECIFIED)
	check(t, "ArgTypes", fn.ArgTypes(), []config.ValueType{config.VALUE_TYPE_UNSPECIFIED, config.STRING})
}

func check(t *testing.T, msg string, got interface{}, want interface{}) {
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s got %#v\nwant %#v", msg, got, want)
	}
}

func TestEQFunc(t *testing.T) {
	fn := newEQ()
	tbl := []struct {
		val   interface{}
		match interface{}
		equal bool
	}{
		{"abc", "abc", true},
		{"abc", 5, false},
		{5, 5, true},
		{"ns1.svc.local", "ns1.*", true},
		{"ns1.svc.local", "ns2.*", false},
		{"svc1.ns1.cluster", "*.ns1.cluster", true},
		{"svc1.ns1.cluster", "*.ns1.cluster1", false},
	}
	for idx, tt := range tbl {
		rv := fn.Call([]interface{}{tt.val, tt.match})
		if rv != tt.equal {
			t.Errorf("[%d] %v ?= %v -- got %#v\nwant %#v", idx, tt.val, tt.match, rv, tt.equal)
		}

	}
	check(t, "ReturnType", fn.ReturnType(), config.BOOL)
	check(t, "ArgTypes", fn.ArgTypes(), []config.ValueType{config.VALUE_TYPE_UNSPECIFIED, config.VALUE_TYPE_UNSPECIFIED})
}
