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
	"fmt"
	"time"

	"github.com/golang/protobuf/ptypes"

	mixerpb "istio.io/mixer/api/v1"
)

// This code would be quite a bit nicer with generics... Oh well.

// AttributeContext maintains an independent set of attributes.
type AttributeContext interface {
	// GetString returns the named attribute if it exists.
	GetString(name string) (string, bool)

	// GetInt64 returns the named attribute if it exists.
	GetInt64(name string) (int64, bool)

	// GetFloat64 returns the named attribute if it exists.
	GetFloat64(name string) (float64, bool)

	// GetBool returns the named attribute if it exists.
	GetBool(name string) (bool, bool)

	// GetTime returns the named attribute if it exists.
	GetTime(name string) (time.Time, bool)

	// GetBytes returns the named attribute if it exists.
	GetBytes(name string) ([]uint8, bool)
}

type attributeContext struct {
	stringAttributes  map[string]string
	int64Attributes   map[string]int64
	float64Attributes map[string]float64
	boolAttributes    map[string]bool
	timeAttributes    map[string]time.Time
	bytesAttributes   map[string][]uint8
}

func newAttributeContext() *attributeContext {
	ac := &attributeContext{}
	ac.reset()
	return ac
}

// Ensure that all dictionary indices are valid and that all values
// are in range.
//
// Note that since we don't have the attribute schema, this doesn't validate
// that a given attribute is being treated as the right type. That is, an
// attribute called 'source.ip' which is of type IP_ADDRESS could be listed as
// a string or an int, and we wouldn't catch it here.
func checkPreconditions(dictionary dictionary, attrs *mixerpb.Attributes) error {
	for k := range attrs.StringAttributes {
		_, present := dictionary[k]
		if !present {
			return fmt.Errorf("attribute index %d is not defined in the current dictionary", k)
		}
	}

	for k := range attrs.Int64Attributes {
		_, present := dictionary[k]
		if !present {
			return fmt.Errorf("attribute index %d is not defined in the current dictionary", k)
		}
	}

	for k := range attrs.DoubleAttributes {
		_, present := dictionary[k]
		if !present {
			return fmt.Errorf("attribute index %d is not defined in the current dictionary", k)
		}
	}

	for k := range attrs.BoolAttributes {
		_, present := dictionary[k]
		if !present {
			return fmt.Errorf("attribute index %d is not defined in the current dictionary", k)
		}
	}

	for k, v := range attrs.TimestampAttributes {
		_, present := dictionary[k]
		if !present {
			return fmt.Errorf("attribute index %d is not defined in the current dictionary", k)
		}

		if _, err := ptypes.Timestamp(v); err != nil {
			return err
		}
	}

	for k := range attrs.BytesAttributes {
		_, present := dictionary[k]
		if !present {
			return fmt.Errorf("attribute index %d is not defined in the current dictionary", k)
		}
	}

	return nil
}

func (ac *attributeContext) update(dictionary dictionary, attrs *mixerpb.Attributes) error {
	// check preconditions up front and bail if there are any
	// errors without mutating the context.
	if err := checkPreconditions(dictionary, attrs); err != nil {
		return err
	}

	// apply all attributes
	if attrs.StringAttributes != nil {
		if ac.stringAttributes == nil {
			ac.stringAttributes = make(map[string]string)
		}

		for k, v := range attrs.StringAttributes {
			ac.stringAttributes[dictionary[k]] = v
		}
	}

	if attrs.Int64Attributes != nil {
		if ac.int64Attributes == nil {
			ac.int64Attributes = make(map[string]int64)
		}

		for k, v := range attrs.Int64Attributes {
			ac.int64Attributes[dictionary[k]] = v
		}
	}

	if attrs.DoubleAttributes != nil {
		if ac.float64Attributes == nil {
			ac.float64Attributes = make(map[string]float64)
		}

		for k, v := range attrs.DoubleAttributes {
			ac.float64Attributes[dictionary[k]] = v
		}
	}

	if attrs.BoolAttributes != nil {
		if ac.boolAttributes == nil {
			ac.boolAttributes = make(map[string]bool)
		}

		for k, v := range attrs.BoolAttributes {
			ac.boolAttributes[dictionary[k]] = v
		}
	}

	if attrs.TimestampAttributes != nil {
		if ac.timeAttributes == nil {
			ac.timeAttributes = make(map[string]time.Time)
		}

		for k, v := range attrs.TimestampAttributes {
			t, _ := ptypes.Timestamp(v)
			ac.timeAttributes[dictionary[k]] = t
		}
	}

	if attrs.BytesAttributes != nil {
		if ac.bytesAttributes == nil {
			ac.bytesAttributes = make(map[string][]uint8)
		}

		for k, v := range attrs.BytesAttributes {
			ac.bytesAttributes[dictionary[k]] = v
		}
	}

	// delete requested attributes
	for _, d := range attrs.DeletedAttributes {
		name, present := dictionary[d]
		if present {
			delete(ac.stringAttributes, name)
			delete(ac.int64Attributes, name)
			delete(ac.float64Attributes, name)
			delete(ac.boolAttributes, name)
			delete(ac.timeAttributes, name)
			delete(ac.bytesAttributes, name)
		}
	}

	return nil
}

func (ac *attributeContext) reset() {
	if len(ac.stringAttributes) > 0 {
		ac.stringAttributes = make(map[string]string)
	}

	if len(ac.int64Attributes) > 0 {
		ac.int64Attributes = make(map[string]int64)
	}

	if len(ac.float64Attributes) > 0 {
		ac.float64Attributes = make(map[string]float64)
	}

	if len(ac.boolAttributes) > 0 {
		ac.boolAttributes = make(map[string]bool)
	}

	if len(ac.timeAttributes) > 0 {
		ac.timeAttributes = make(map[string]time.Time)
	}

	if len(ac.bytesAttributes) > 0 {
		ac.bytesAttributes = make(map[string][]uint8)
	}
}

func (ac *attributeContext) GetString(name string) (string, bool) {
	r, b := ac.stringAttributes[name]
	return r, b
}

func (ac *attributeContext) GetInt64(name string) (int64, bool) {
	r, b := ac.int64Attributes[name]
	return r, b
}

func (ac *attributeContext) GetFloat64(name string) (float64, bool) {
	r, b := ac.float64Attributes[name]
	return r, b
}

func (ac *attributeContext) GetBool(name string) (bool, bool) {
	r, b := ac.boolAttributes[name]
	return r, b
}

func (ac *attributeContext) GetTime(name string) (time.Time, bool) {
	r, b := ac.timeAttributes[name]
	return r, b
}

func (ac *attributeContext) GetBytes(name string) ([]uint8, bool) {
	r, b := ac.bytesAttributes[name]
	return r, b
}
