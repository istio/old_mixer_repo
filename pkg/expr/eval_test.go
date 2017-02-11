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
	"strings"
	"testing"
	"time"

	"istio.io/mixer/pkg/attribute"
)

type evalTest struct {
	src    string
	tmap   map[string]interface{}
	result interface{}
	err    string
}

func TestGoodEval(t *testing.T) {
	tests := []evalTest{
		{
			"a == 2",
			map[string]interface{}{
				"a": int64(2),
			},
			true, "",
		},
		{
			"a != 2",
			map[string]interface{}{
				"a": int64(2),
			},
			false, "",
		},
		{
			"a != 2",
			map[string]interface{}{
				"d": int64(2),
			},
			false, "unresolved attribute",
		},
		{
			"a ",
			map[string]interface{}{
				"a": int64(2),
			},
			int64(2), "",
		},
		{
			"2 ",
			map[string]interface{}{
				"a": int64(2),
			},
			int64(2), "",
		},
		{
			`request.user == "user1"`,
			map[string]interface{}{
				"request.user": "user1",
			},
			true, "",
		},
		{
			`request.user2| request.user | "user1"`,
			map[string]interface{}{
				"request.user": "user2",
			},
			"user2", "",
		},
		{
			`request.user2| request.user3 | "user1"`,
			map[string]interface{}{
				"request.user": "user2",
			},
			"user1", "",
		},
		{
			`request.size| 200`,
			map[string]interface{}{
				"request.size": int64(120),
			},
			int64(120), "",
		},
		{
			`request.size| 200`,
			map[string]interface{}{
				"request.size": int64(0),
			},
			int64(200), "",
		},
		{
			`request.size| 200`,
			map[string]interface{}{
				"request.size1": int64(0),
			},
			int64(200), "",
		},
		{
			`(x == 20 && y == 10) || x == 30`,
			map[string]interface{}{
				"x": int64(20),
				"y": int64(10),
			},
			true, "",
		},
		{
			`service.name == "*.ns1.cluster" && service.user == "admin"`,
			map[string]interface{}{
				"service.name": "svc1.ns1.cluster",
				"service.user": "admin",
			},
			true, "",
		},
		{
			`(x/y) == 30`,
			map[string]interface{}{
				"x": int64(20),
				"y": int64(10),
			},
			false, "unknown function: QUO",
		},
		// The following will not work until we support
		// map as an attribute type
		{
			`request.header["X-FORWARDED-HOST"] == "aaa"`,
			map[string]interface{}{
				"request.header": map[string]string{
					"X-FORWARDED-HOST": "bbb",
				},
				"y": int64(10),
			},
			true, "unresolved attribute",
		},
	}

	for idx, tst := range tests {
		attrs := &bag{attrs: tst.tmap}
		exp, err := Parse(tst.src)
		if err != nil {
			t.Errorf("[%d] unexpected error: %s", idx, err)
			continue
		}
		res, err := exp.Eval(attrs, funcMap())
		if err != nil {
			if tst.err == "" {
				t.Errorf("[%d] unexpected error: %s", idx, err)
			} else if !strings.Contains(err.Error(), tst.err) {
				t.Errorf("[%d] got %s\nwant %s", idx, err, tst.err)
			}
			continue
		}
		if res != tst.result {
			t.Errorf("[%d] got %s\nwant %s", idx, res, tst.result)
		}
	}

}

// fake bag
type bag struct {
	attribute.Bag
	attrs map[string]interface{}
}

func (b *bag) String(name string) (string, bool) {
	c, found := b.attrs[name]
	if !found {
		return "", false
	}
	s, found := c.(string)
	if !found {
		return "", false
	}
	return s, true
}

// Int64 returns the named attribute if it exists.
func (b *bag) Int64(name string) (int64, bool) {
	c, found := b.attrs[name]
	if !found {
		return 0, false
	}
	if _, found = c.(int64); !found {
		return 0, false
	}

	return c.(int64), true
}

// Float64 returns the named attribute if it exists.
func (b *bag) Float64(name string) (float64, bool) {
	c, found := b.attrs[name]
	if !found {
		return 0.0, false
	}
	if _, found = c.(float64); !found {
		return 0.0, false
	}

	return c.(float64), true
}

// Bool returns the named attribute if it exists.
func (b *bag) Bool(name string) (bool, bool) {
	c, found := b.attrs[name]
	if !found {
		return false, false
	}
	if _, found = c.(bool); !found {
		return false, false
	}

	return c.(bool), true
}

func (b *bag) StringMap(name string) (map[string]string, bool) {
	c, found := b.attrs[name]
	if !found {
		return nil, false
	}
	if _, found = c.(map[string]string); !found {
		return nil, false
	}

	return c.(map[string]string), true

}
func (b *bag) Time(name string) (tt time.Time, bb bool) { return }

// Duration returns the named attribute if it exists.
func (b *bag) Duration(name string) (tt time.Duration, bb bool) { return }

// Bytes returns the named attribute if it exists.
func (b *bag) Bytes(name string) (u []uint8, bb bool) { return }
