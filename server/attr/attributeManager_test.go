// Copyright 2016 Google Inc.
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

package attr

import (
	"testing"
	"time"

	ts "github.com/golang/protobuf/ptypes/timestamp"

	mixerpb "istio.io/mixer/api/v1"
)

func TestAttributeManager(t *testing.T) {
	type getStringCase struct {
		name    string
		result  string
		present bool
	}

	type getInt64Case struct {
		name    string
		result  int64
		present bool
	}

	type getFloat64Case struct {
		name    string
		result  float64
		present bool
	}

	type getBoolCase struct {
		name    string
		result  bool
		present bool
	}

	type getTimeCase struct {
		name    string
		result  time.Time
		present bool
	}

	type getBytesCase struct {
		name    string
		result  []uint8
		present bool
	}

	type testCase struct {
		attrs      mixerpb.Attributes
		result     bool
		getString  []getStringCase
		getInt64   []getInt64Case
		getFloat64 []getFloat64Case
		getBool    []getBoolCase
		getTime    []getTimeCase
		getBytes   []getBytesCase
	}

	cases := []testCase{
		// 0: make sure reset works against a fresh state
		testCase{
			attrs: mixerpb.Attributes{
				ResetContext: true,
			},
			result: true,
		},

		// 1: basic case to try out adding one of everything
		testCase{
			attrs: mixerpb.Attributes{
				Dictionary:          dictionary{1: "name1", 2: "name2", 3: "name3", 4: "name4", 5: "name5", 6: "name6"},
				StringAttributes:    map[int32]string{1: "1"},
				Int64Attributes:     map[int32]int64{2: 2},
				DoubleAttributes:    map[int32]float64{3: 3.0},
				BoolAttributes:      map[int32]bool{4: true},
				TimestampAttributes: map[int32]*ts.Timestamp{5: &ts.Timestamp{Seconds: 5, Nanos: 5}},
				BytesAttributes:     map[int32][]uint8{6: []byte{6}},
				ResetContext:        false,
				AttributeContext:    0,
				DeletedAttributes:   nil,
			},
			result: true,
			getString: []getStringCase{
				getStringCase{"name1", "1", true},
				getStringCase{"name2", "", false},
				getStringCase{"name42", "", false},
			},

			getInt64: []getInt64Case{
				getInt64Case{"name2", 2, true},
				getInt64Case{"name1", 0, false},
				getInt64Case{"name42", 0, false},
			},

			getFloat64: []getFloat64Case{
				getFloat64Case{"name3", 3.0, true},
				getFloat64Case{"name1", 0.0, false},
				getFloat64Case{"name42", 0.0, false},
			},

			getBool: []getBoolCase{
				getBoolCase{"name4", true, true},
				getBoolCase{"name1", false, false},
				getBoolCase{"name42", false, false},
			},

			getTime: []getTimeCase{
				getTimeCase{"name5", time.Date(1970, time.January, 1, 0, 0, 5, 5, time.UTC), true},
				getTimeCase{"name1", time.Time{}, false},
				getTimeCase{"name42", time.Time{}, false},
			},

			getBytes: []getBytesCase{
				getBytesCase{"name6", []byte{6}, true},
				getBytesCase{"name1", nil, false},
				getBytesCase{"name42", nil, false},
			},
		},

		// 2: now switch dictionaries and make sure we can still find things
		testCase{
			attrs: mixerpb.Attributes{
				Dictionary: dictionary{11: "name1", 22: "name2", 33: "name3", 44: "name4", 55: "name5", 66: "name6"},
			},
			result: true,
			getString: []getStringCase{
				getStringCase{"name1", "1", true},
				getStringCase{"name2", "", false},
				getStringCase{"name42", "", false},
			},

			getInt64: []getInt64Case{
				getInt64Case{"name2", 2, true},
				getInt64Case{"name1", 0, false},
				getInt64Case{"name42", 0, false},
			},

			getFloat64: []getFloat64Case{
				getFloat64Case{"name3", 3.0, true},
				getFloat64Case{"name1", 0.0, false},
				getFloat64Case{"name42", 0.0, false},
			},

			getBool: []getBoolCase{
				getBoolCase{"name4", true, true},
				getBoolCase{"name1", false, false},
				getBoolCase{"name42", false, false},
			},

			getTime: []getTimeCase{
				getTimeCase{"name5", time.Date(1970, time.January, 1, 0, 0, 5, 5, time.UTC), true},
				getTimeCase{"name1", time.Time{}, false},
				getTimeCase{"name42", time.Time{}, false},
			},

			getBytes: []getBytesCase{
				getBytesCase{"name6", []byte{6}, true},
				getBytesCase{"name1", nil, false},
				getBytesCase{"name42", nil, false},
			},
		},

		// 3: now delete everything and make sure it's all gone
		testCase{
			attrs: mixerpb.Attributes{
				DeletedAttributes: []int32{11, 22, 33, 44, 55, 66},
			},
			result:     true,
			getString:  []getStringCase{getStringCase{"name1", "", false}},
			getInt64:   []getInt64Case{getInt64Case{"name2", 0, false}},
			getFloat64: []getFloat64Case{getFloat64Case{"name3", 0.0, false}},
			getBool:    []getBoolCase{getBoolCase{"name4", false, false}},
			getTime:    []getTimeCase{getTimeCase{"name5", time.Time{}, false}},
			getBytes:   []getBytesCase{getBytesCase{"name6", []byte{}, false}},
		},

		// 4: add stuff back in
		testCase{
			attrs: mixerpb.Attributes{
				Dictionary:          dictionary{1: "name1", 2: "name2", 3: "name3", 4: "name4", 5: "name5", 6: "name6"},
				StringAttributes:    map[int32]string{1: "1"},
				Int64Attributes:     map[int32]int64{2: 2},
				DoubleAttributes:    map[int32]float64{3: 3.0},
				BoolAttributes:      map[int32]bool{4: true},
				TimestampAttributes: map[int32]*ts.Timestamp{5: &ts.Timestamp{Seconds: 5, Nanos: 5}},
				BytesAttributes:     map[int32][]uint8{6: []byte{6}},
				ResetContext:        false,
				AttributeContext:    0,
				DeletedAttributes:   nil,
			},
			result: true,
		},

		// 5: make sure reset works
		testCase{
			attrs: mixerpb.Attributes{
				ResetContext: true,
			},
			result:     true,
			getString:  []getStringCase{getStringCase{"name1", "", false}},
			getInt64:   []getInt64Case{getInt64Case{"name2", 0, false}},
			getFloat64: []getFloat64Case{getFloat64Case{"name3", 0.0, false}},
			getBool:    []getBoolCase{getBoolCase{"name4", false, false}},
			getTime:    []getTimeCase{getTimeCase{"name5", time.Time{}, false}},
			getBytes:   []getBytesCase{getBytesCase{"name6", []byte{}, false}},
		},

		// 6: make sure reset works against a reset state
		testCase{
			attrs: mixerpb.Attributes{
				ResetContext: true,
			},
			result: true,
		},

		// 7: try out bad dictionary index for strings
		testCase{
			attrs:  mixerpb.Attributes{StringAttributes: map[int32]string{42: "1"}},
			result: false,
		},

		// 8: try out bad dictionary index for int64
		testCase{
			attrs:  mixerpb.Attributes{Int64Attributes: map[int32]int64{42: 0}},
			result: false,
		},

		// 9: try out bad dictionary index for float64
		testCase{
			attrs:  mixerpb.Attributes{DoubleAttributes: map[int32]float64{42: 0.0}},
			result: false,
		},

		// 10: try out bad dictionary index for bool
		testCase{
			attrs:  mixerpb.Attributes{BoolAttributes: map[int32]bool{42: false}},
			result: false,
		},

		// 11: try out bad dictionary index for timestamp
		testCase{
			attrs:  mixerpb.Attributes{TimestampAttributes: map[int32]*ts.Timestamp{42: &ts.Timestamp{}}},
			result: false,
		},

		// 12: try out bad dictionary index for bytes
		testCase{
			attrs:  mixerpb.Attributes{BytesAttributes: map[int32][]uint8{42: []uint8{}}},
			result: false,
		},

		// 13: try to delete attributes that don't exist
		testCase{
			attrs:  mixerpb.Attributes{DeletedAttributes: []int32{111, 222, 333}},
			result: true,
		},
	}

	am := NewAttributeManager()
	at := am.NewTracker()
	for i, c := range cases {
		ac, err := at.Update(&c.attrs)
		if (err == nil) != c.result {
			if c.result {
				t.Errorf("Expected Update to succeed but it returned %v for test case %d", err, i)
			} else {
				t.Errorf("Expected Update to fail but it succeeded for test case %d", i)
			}
		}

		for j, g := range c.getString {
			result, present := ac.GetString(g.name)
			if result != g.result {
				t.Errorf("Expecting result='%v', got result='%v' for string test case %v:%v", g.result, result, i, j)
			}

			if present != g.present {
				t.Errorf("Expecting present=%v, got present=%v for string test case %v:%v", g.present, present, i, j)
			}
		}

		for j, g := range c.getInt64 {
			result, present := ac.GetInt64(g.name)
			if result != g.result {
				t.Errorf("Expecting result='%v', got result='%v' for int64 test case %v:%v", g.result, result, i, j)
			}

			if present != g.present {
				t.Errorf("Expecting present=%v, got present=%v for int64 test case %v:%v", g.present, present, i, j)
			}
		}

		for j, g := range c.getFloat64 {
			result, present := ac.GetFloat64(g.name)
			if result != g.result {
				t.Errorf("Expecting result='%v', got result='%v' for float64 test case %v:%v", g.result, result, i, j)
			}

			if present != g.present {
				t.Errorf("Expecting present=%v, got present=%v for float64 test case %v:%v", g.present, present, i, j)
			}
		}

		for j, g := range c.getBool {
			result, present := ac.GetBool(g.name)
			if result != g.result {
				t.Errorf("Expecting result='%v', got result='%v' for bool test case %v:%v", g.result, result, i, j)
			}

			if present != g.present {
				t.Errorf("Expecting present=%v, got present=%v for bool test case %v:%v", g.present, present, i, j)
			}
		}

		for j, g := range c.getTime {
			result, present := ac.GetTime(g.name)
			if result != g.result {
				t.Errorf("Expecting result='%v', got result='%v' for time test case %v:%v", g.result, result, i, j)
			}

			if present != g.present {
				t.Errorf("Expecting present=%v, got present=%v for time test case %v:%v", g.present, present, i, j)
			}
		}

		for j, g := range c.getBytes {
			result, present := ac.GetBytes(g.name)

			same := len(result) == len(g.result)
			if same {
				for i := range result {
					if result[i] != g.result[i] {
						same = false
						break
					}
				}
			}

			if !same {
				t.Errorf("Expecting result='%v', got result='%v' for bytes test case %v:%v", g.result, result, i, j)
			}

			if present != g.present {
				t.Errorf("Expecting present=%v, got present=%v for bytes test case %v:%v", g.present, present, i, j)
			}
		}
	}
}

/*
func setup(t *testing.T, bc BuilderConfig, ac AdapterConfig) adapters.FactTracker {
	b := NewBuilder()
	b.Configure(&bc)

	a, err := b.NewAdapter(&ac)
	if err != nil {
		t.Error("Expected to successfully create a mapper")
	}
	fc := a.(adapters.FactConverter)
	return fc.NewTracker()
}

func TestNoRules(t *testing.T) {
	rules := make(map[string]string)
	tracker := setup(t, BuilderConfig{}, AdapterConfig{Rules: rules})

	labels := tracker.GetLabels()
	if len(labels) != 0 {
		t.Error("Got labels when expecting none")
	}

	// pretend to add some facts and try again
	facts := make(map[string]string)
	tracker.UpdateFacts(facts)

	labels = tracker.GetLabels()
	if len(labels) != 0 {
		t.Error("Got labels when expecting none")
	}

	// add some actual facts and try again
	facts = make(map[string]string)
	facts["Fact1"] = "One"
	facts["Fact2"] = "Two"
	facts["Fact4"] = "Four"
	tracker.UpdateFacts(facts)

	labels = tracker.GetLabels()
	if len(labels) != 0 {
		t.Error("Got labels when expecting none")
	}
}

func TestOddballRules(t *testing.T) {
	b := NewBuilder()
	b.Configure(b.DefaultBuilderConfig())

	rules := make(map[string]string)

	badOddballs := []string{"|A|B|C", "|A|B|", "A| |C", "A||C"}
	for _, oddball := range badOddballs {
		rules["Lab1"] = oddball
		if _, err := b.NewAdapter(&AdapterConfig{Rules: rules}); err == nil {
			t.Errorf("Expecting to not be able to create a mapper for rule %s", oddball)
		}
	}

	goodOddballs := []string{"A", "A|B", "A | B | C", "A| B| C"}
	for _, oddball := range goodOddballs {
		rules["Lab1"] = oddball
		if _, err := b.NewAdapter(&AdapterConfig{Rules: rules}); err != nil {
			t.Errorf("Expecting to be able to create a mapper for rule %s: %v", oddball, err)
		}
	}
}

func TestNoFacts(t *testing.T) {
	rules := make(map[string]string)
	rules["Lab1"] = "Fact1|Fact2|Fact3"
	rules["Lab2"] = "Fact3|Fact2|Fact1"
	tracker := setup(t, BuilderConfig{}, AdapterConfig{Rules: rules})

	labels := tracker.GetLabels()
	if len(labels) != 0 {
		t.Error("Got labels when expecting none")
	}

	// pretend to add some facts and try again
	facts := make(map[string]string)
	tracker.UpdateFacts(facts)

	labels = tracker.GetLabels()
	if len(labels) != 0 {
		t.Error("Got labels when expecting none")
	}

	// add some actual facts and try again
	facts["Fact1"] = "One"
	facts["Fact2"] = "Two"
	facts["Fact4"] = "Four"
	tracker.UpdateFacts(facts)

	labels = tracker.GetLabels()
	if len(labels) != 2 {
		t.Error("Got no labels, expecting 2")
	}

	if labels["Lab1"] != "One" || labels["Lab2"] != "Two" {
		t.Error("Didn't get the expected label values")
	}
}

func TestAddRemoveFacts(t *testing.T) {
	rules := make(map[string]string)
	rules["Lab1"] = "Fact1|Fact2|Fact3"
	rules["Lab2"] = "Fact3|Fact2|Fact1"
	tracker := setup(t, BuilderConfig{}, AdapterConfig{Rules: rules})

	// add some facts and try again
	facts := make(map[string]string)
	facts["Fact1"] = "One"
	tracker.UpdateFacts(facts)

	labels := tracker.GetLabels()
	if len(labels) != 2 || labels["Lab1"] != "One" || labels["Lab2"] != "One" {
		t.Error("Got unexpected labels")
	}

	facts["Fact2"] = "Two"
	tracker.UpdateFacts(facts)

	labels = tracker.GetLabels()
	if len(labels) != 2 || labels["Lab1"] != "One" || labels["Lab2"] != "Two" {
		t.Error("Got unexpected labels")
	}

	facts["Fact2"] = ""
	tracker.UpdateFacts(facts)

	labels = tracker.GetLabels()
	if len(labels) != 2 || labels["Lab1"] != "One" || labels["Lab2"] != "" {
		t.Error("Got unexpected labels")
	}

	facts["Fact2"] = ""
	tracker.PurgeFacts([]string{"Fact2"})

	labels = tracker.GetLabels()
	if len(labels) != 2 || labels["Lab1"] != "One" || labels["Lab2"] != "One" {
		t.Error("Got unexpected labels")
	}

	// purge a fact that doesn't exist
	tracker.PurgeFacts([]string{"Fact42"})
}

func TestReset(t *testing.T) {
	rules := make(map[string]string)
	rules["Lab1"] = "Fact1|Fact2|Fact3"
	rules["Lab2"] = "Fact3|Fact2|Fact1"
	tracker := setup(t, BuilderConfig{}, AdapterConfig{Rules: rules})

	// add some actual facts and try again
	facts := make(map[string]string)
	facts["Fact1"] = "One"
	facts["Fact2"] = "Two"
	facts["Fact4"] = "Four"
	tracker.UpdateFacts(facts)

	labels := tracker.GetLabels()
	if len(labels) != 2 || labels["Lab1"] != "One" || labels["Lab2"] != "Two" {
		t.Error("Got unexpected labels")
	}

	tracker.Reset()

	labels = tracker.GetLabels()
	if len(labels) != 0 {
		t.Error("Got unexpected labels")
	}

	numFacts, numLabels := tracker.Stats()
	if numFacts != 0 || numLabels != 0 {
		t.Error("Expected no facts and no labels")
	}
}
*/
