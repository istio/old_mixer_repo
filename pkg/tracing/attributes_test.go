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

package tracing

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/golang/glog"

	mixerpb "istio.io/api/mixer/v1"
	"istio.io/mixer/pkg/attribute"
)

var (
	wordList = []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", headerAttributeName}

	attrs = &mixerpb.Attributes{
		Words:   []string{"1", "2"},
		Strings: map[int32]int32{1: -1, 2: -2},
		Int64S:  map[int32]int64{3: 3, 4: 4},
		Doubles: map[int32]float64{5: 5.0, 6: 6.0},
		Bools:   map[int32]bool{7: true, 8: false},
		Bytes:   map[int32][]uint8{9: {9}, 10: {10}},
		StringMaps: map[int32]mixerpb.StringMap{
			11: {}, // make sure request.headers is present/an empty map.
		},
	}
)

func TestContext(t *testing.T) {
	bag := attribute.GetMutableBag(nil)
	if err := bag.UpdateBagFromProto(attrs, wordList); err != nil {
		t.Errorf("Failed to construct bag: %v", err)
	}

	if c := FromContext(context.Background()); c != nil {
		t.Errorf("FromContext(context.Background()) = %v; wanted nil", c)
	}

	c := NewCarrier(bag, bag, wordList)

	ctx := context.Background()
	ctx = NewContext(ctx, c)
	if cc := FromContext(ctx); cc != c {
		t.Errorf("FromContext(ctx) = %v; wanted %v; context: %v", cc, c, ctx)
	}
}

func TestSet(t *testing.T) {
	bag := attribute.GetMutableBag(nil)
	if err := bag.UpdateBagFromProto(attrs, wordList); err != nil {
		t.Errorf("Failed to construct bag: %v", err)
	}

	cases := []struct {
		key string
		val string
	}{
		{"a", "b"},
		{"foo", "bar"},
		{"spanID", "1235"},
	}
	for _, c := range cases {
		cr := NewCarrier(nil, bag, nil)
		cr.Set(c.key, c.val)
		val, _ := cr.resp.Get(headerAttributeName)
		headers := val.(map[string]string)
		if v, ok := headers[c.key]; !ok || v != c.val {
			t.Errorf("carrier.Set(%s, %s); bag.String(%s) = %s; wanted %s", c.key, c.val, c.key, val, c.val)
		}
	}
}

func TestForeachKey(t *testing.T) {
	bag := attribute.GetMutableBag(nil)
	if err := bag.UpdateBagFromProto(attrs, wordList); err != nil {
		t.Errorf("Failed to construct bag: %v", err)
	}

	cases := []struct {
		data map[string]string
	}{
		{map[string]string{"a": "b", "c": "d"}},
	}
	for _, c := range cases {
		b := bag.Child()
		for k, v := range c.data {
			b.Set(k, v)
		}

		err := NewCarrier(b, nil, wordList).ForeachKey(func(key, val string) error {
			if dval, found := c.data[key]; !found || dval != val {
				t.Errorf("ForeachKey(func(%s, %s)); wanted func(%s, %s)", key, val, key, dval)
			}
			return nil
		})
		if err != nil {
			t.Errorf("ForeachKey failed with unexpected err: %v", err)
		}
	}
}

func TestForeachKey_PropagatesErr(t *testing.T) {
	bag := attribute.GetMutableBag(nil)
	if err := bag.UpdateBagFromProto(attrs, wordList); err != nil {
		t.Errorf("Failed to construct bag: %v", err)
	}

	c := NewCarrier(bag, bag, []string{"k"})
	bag.Set(headerAttributeName, map[string]string{"k": "v"})

	err := errors.New("expected")
	propErr := c.ForeachKey(func(key, val string) error {
		return err
	})

	if propErr != err {
		t.Errorf("ForeachKey(...) = %v; wanted err %v", propErr, err)
	}
}

func TestForeachKey_MissingHeaders(t *testing.T) {
	bag := attribute.GetMutableBag(nil)
	if err := bag.UpdateBagFromProto(attrs, wordList); err != nil {
		t.Errorf("Failed to construct bag: %v", err)
	}

	c := NewCarrier(bag, bag, []string{"k"})
	bag.Reset()

	err := c.ForeachKey(func(key, val string) error {
		panic("Should never be called because we failed earlier getting the headers.")
	})

	if err == nil || !strings.Contains(err.Error(), "no header attribute named") {
		t.Errorf("ForeachKey(...) = %v; wanted err containing 'No header attribute named'", err)
	}
}

func TestSet_BadHeaders(t *testing.T) {
	bag := attribute.GetMutableBag(nil)
	if err := bag.UpdateBagFromProto(attrs, wordList); err != nil {
		t.Errorf("Failed to construct bag: %v", err)
	}

	c := NewCarrier(bag, bag, []string{"k"})
	bag.Reset()

	initial := glog.Stats.Warning.Lines()
	c.Set("foo", "bar")
	glog.Flush()
	final := glog.Stats.Warning.Lines()
	if initial == final {
		t.Fatalf("Should've logged failure to extract header attribute.")
	}
}
