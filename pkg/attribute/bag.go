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

package attribute

import (
	"time"
)

// Bag is a generic mechanism to track a set of attributes.
type Bag interface {
	// String returns the named attribute if it exists.
	String(name string) (string, bool)

	// SetString sets an override for a named attribute.
	SetString(name string, value string)

	// Int64 returns the named attribute if it exists.
	Int64(name string) (int64, bool)

	// SetInt64 sets an override for a named attribute.
	SetInt64(name string, value int64)

	// Float64 returns the named attribute if it exists.
	Float64(name string) (float64, bool)

	// SetFloat64 sets an override for a named attribute.
	SetFloat64(name string, value float64)

	// Bool returns the named attribute if it exists.
	Bool(name string) (bool, bool)

	// SetBool sets an override for a named attribute.
	SetBool(name string, value bool)

	// Time returns the named attribute if it exists.
	Time(name string) (time.Time, bool)

	// SetTime sets an override for a named attribute.
	SetTime(name string, value time.Time)

	// Bytes returns the named attribute if it exists.
	Bytes(name string) ([]uint8, bool)

	// SetBytes sets an override for a named attribute.
	SetBytes(name string, value []uint8)

	// Reset removes all overrides
	Reset()
}

type bag struct {
	context  *context
	strings  map[string]string
	int64s   map[string]int64
	float64s map[string]float64
	bools    map[string]bool
	times    map[string]time.Time
	bytes    map[string][]uint8
}

func newBag(ac *context) *bag {
	var ab = &bag{context: ac}
	ab.Reset()
	return ab
}

func (ab *bag) String(name string) (string, bool) {
	var r string
	var b bool
	if r, b = ab.strings[name]; !b {
		r, b = ab.context.String(name)
	}
	return r, b
}

func (ab *bag) SetString(name string, value string) {
	ab.strings[name] = value
}

func (ab *bag) Int64(name string) (int64, bool) {
	var r int64
	var b bool
	if r, b = ab.int64s[name]; !b {
		r, b = ab.context.Int64(name)
	}
	return r, b
}

func (ab *bag) SetInt64(name string, value int64) {
	ab.int64s[name] = value
}

func (ab *bag) Float64(name string) (float64, bool) {
	var r float64
	var b bool
	if r, b = ab.float64s[name]; !b {
		r, b = ab.context.Float64(name)
	}
	return r, b
}

func (ab *bag) SetFloat64(name string, value float64) {
	ab.float64s[name] = value
}

func (ab *bag) Bool(name string) (bool, bool) {
	var r bool
	var b bool
	if r, b = ab.bools[name]; !b {
		r, b = ab.context.Bool(name)
	}
	return r, b
}

func (ab *bag) SetBool(name string, value bool) {
	ab.bools[name] = value
}

func (ab *bag) Time(name string) (time.Time, bool) {
	var r time.Time
	var b bool
	if r, b = ab.times[name]; !b {
		r, b = ab.context.Time(name)
	}
	return r, b
}

func (ab *bag) SetTime(name string, value time.Time) {
	ab.times[name] = value
}

func (ab *bag) Bytes(name string) ([]uint8, bool) {
	var r []uint8
	var b bool
	if r, b = ab.bytes[name]; !b {
		r, b = ab.context.Bytes(name)
	}
	return r, b
}

func (ab *bag) SetBytes(name string, value []uint8) {
	ab.bytes[name] = value
}

func (ab *bag) Reset() {
	// Ideally, this would be merely clearing lists instead of reallocating 'em,
	// but there's no way to do that in Go :-(

	ab.strings = make(map[string]string)
	ab.int64s = make(map[string]int64)
	ab.float64s = make(map[string]float64)
	ab.bools = make(map[string]bool)
	ab.times = make(map[string]time.Time)
	ab.bytes = make(map[string][]uint8)
}
