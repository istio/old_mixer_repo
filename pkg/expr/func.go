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
	"strings"

	config "istio.io/api/mixer/v1/config/descriptor"
)

// Func every expression function provider should have this interface.
type Func interface {
	// Name uniquely identified the function.
	Name() string

	// ReturnType specifies the return type of this function.
	ReturnType() config.ValueType

	// ArgTypes specifies the argument types in order expected by the function.
	ArgTypes() []config.ValueType

	// NullArgs specifies if the function accepts null args
	NullArgs() bool

	// Call performs the function call. It is guaranteed
	// that call will be made with correct types and arity.
	// may panic.
	Call([]interface{}) interface{}
}

type baseFunc struct {
	name     string
	argTypes []config.ValueType
	retType  config.ValueType
	nullArgs bool
}

func (f *baseFunc) Name() string                 { return f.name }
func (f *baseFunc) ReturnType() config.ValueType { return f.retType }
func (f *baseFunc) ArgTypes() []config.ValueType { return f.argTypes }
func (f *baseFunc) NullArgs() bool               { return f.nullArgs }

type eqFunc struct {
	*baseFunc
	invert bool
}

func newEQFunc() Func {
	return &eqFunc{
		baseFunc: &baseFunc{
			name:     "EQ",
			retType:  config.BOOL,
			argTypes: []config.ValueType{config.VALUE_TYPE_UNSPECIFIED, config.VALUE_TYPE_UNSPECIFIED},
		},
	}
}

func newNEQFunc() Func {
	return &eqFunc{
		baseFunc: &baseFunc{
			name:     "NEQ",
			retType:  config.BOOL,
			argTypes: []config.ValueType{config.VALUE_TYPE_UNSPECIFIED, config.VALUE_TYPE_UNSPECIFIED},
		},
		invert: true,
	}
}

func (f *eqFunc) Call(args []interface{}) interface{} {
	res := f.call(args)
	if f.invert {
		return !res
	}
	return res
}

// Call
func (f *eqFunc) call(args []interface{}) bool {
	var s0 string
	var s1 string
	var ok bool

	if s0, ok = args[0].(string); !ok {
		return args[0] == args[1]
	}
	if s1, ok = args[1].(string); !ok {
		// s0 is string and s1 is not
		return false
	}
	return matchWithWildcards(s0, s1)
}

// matchWithWildcards     s0 == ns1.*   --> ns1 should be a prefix of s0
// s0 == *.ns1 --> ns1 should be a suffix of s0
func matchWithWildcards(s0 string, s1 string) bool {
	if strings.HasSuffix(s1, "*") {
		return strings.HasPrefix(s0, s1[:len(s1)-1])
	}

	if strings.HasPrefix(s1, "*") {
		return strings.HasSuffix(s0, s1[1:])
	}
	return s0 == s1
}

type lAnd struct {
	*baseFunc
}

func newLANDFunc() Func {
	return &lAnd{
		&baseFunc{
			name:     "LAND",
			retType:  config.BOOL,
			argTypes: []config.ValueType{config.BOOL, config.BOOL},
			nullArgs: false,
		},
	}
}

// Call should return thru if at least one element is true
func (f *lAnd) Call(args []interface{}) interface{} {
	return args[0].(bool) && args[1].(bool)
}

type lOr struct {
	*baseFunc
}

func newLORFunc() Func {
	return &lOr{
		&baseFunc{
			name:     "LOR",
			retType:  config.BOOL,
			argTypes: []config.ValueType{config.BOOL, config.BOOL},
			nullArgs: false,
		},
	}
}

// Call should return thru if at least one element is true
func (f *lOr) Call(args []interface{}) interface{} {
	return args[0].(bool) || args[1].(bool)
}

//
type nonboolOr struct {
	*baseFunc
}

// selects first non empty string.
func newORFunc() Func {
	return &nonboolOr{
		baseFunc: &baseFunc{
			name:     "OR",
			retType:  config.VALUE_TYPE_UNSPECIFIED,
			argTypes: []config.ValueType{config.VALUE_TYPE_UNSPECIFIED, config.VALUE_TYPE_UNSPECIFIED},
			nullArgs: true,
		},
	}
}

// Call selects first non empty string
func (f *nonboolOr) Call(args []interface{}) interface{} {
	if args[0] == nil {
		return args[1]
	}
	v := reflect.ValueOf(args[0])
	if v.Interface() != reflect.Zero(v.Type()).Interface() {
		return args[0]
	}
	return args[1]
}

func inventory() []Func {
	return []Func{
		newEQFunc(),
		newNEQFunc(),
		newORFunc(),
		newLORFunc(),
		newLANDFunc(),
	}
}

func funcMap() map[string]Func {
	m := make(map[string]Func)
	for _, fn := range inventory() {
		m[fn.Name()] = fn
	}
	return m
}
