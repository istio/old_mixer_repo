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

// Overrider is a generic mechanism to locally override attribute
// valuesvfrom a context, without mutating the underlying context.
type Overrider interface {
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

type overrider struct {
	context  Context
	strings  map[string]string
	int64s   map[string]int64
	float64s map[string]float64
	bools    map[string]bool
	times    map[string]time.Time
	bytes    map[string][]uint8
}

func newOverrider(ac *context) *overrider {
	var ao = &overrider{context: ac}
	ao.Reset()
	return ao
}

func (ao *overrider) String(name string) (string, bool) {
	var r string
	var b bool
	if r, b = ao.strings[name]; !b {
		r, b = ao.context.String(name)
	}
	return r, b
}

func (ao *overrider) SetString(name string, value string) {
	ao.strings[name] = value
}

func (ao *overrider) Int64(name string) (int64, bool) {
	var r int64
	var b bool
	if r, b = ao.int64s[name]; !b {
		r, b = ao.context.Int64(name)
	}
	return r, b
}

func (ao *overrider) SetInt64(name string, value int64) {
	ao.int64s[name] = value
}

func (ao *overrider) Float64(name string) (float64, bool) {
	var r float64
	var b bool
	if r, b = ao.float64s[name]; !b {
		r, b = ao.context.Float64(name)
	}
	return r, b
}

func (ao *overrider) SetFloat64(name string, value float64) {
	ao.float64s[name] = value
}

func (ao *overrider) Bool(name string) (bool, bool) {
	var r bool
	var b bool
	if r, b = ao.bools[name]; !b {
		r, b = ao.context.Bool(name)
	}
	return r, b
}

func (ao *overrider) SetBool(name string, value bool) {
	ao.bools[name] = value
}

func (ao *overrider) Time(name string) (time.Time, bool) {
	var r time.Time
	var b bool
	if r, b = ao.times[name]; !b {
		r, b = ao.context.Time(name)
	}
	return r, b
}

func (ao *overrider) SetTime(name string, value time.Time) {
	ao.times[name] = value
}

func (ao *overrider) Bytes(name string) ([]uint8, bool) {
	var r []uint8
	var b bool
	if r, b = ao.bytes[name]; !b {
		r, b = ao.context.Bytes(name)
	}
	return r, b
}

func (ao *overrider) SetBytes(name string, value []uint8) {
	ao.bytes[name] = value
}

func (ao *overrider) Reset() {
	// Ideally, this would be merely clearing lists instead of reallocating 'em,
	// but there's no way to do that in Go :-(

	ao.strings = make(map[string]string)
	ao.int64s = make(map[string]int64)
	ao.float64s = make(map[string]float64)
	ao.bools = make(map[string]bool)
	ao.times = make(map[string]time.Time)
	ao.bytes = make(map[string][]uint8)
}
