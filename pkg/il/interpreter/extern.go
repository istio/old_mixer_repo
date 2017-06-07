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

package interpreter

import (
	"fmt"
	"math"
	"reflect"

	"istio.io/mixer/pkg/il"
)

// Extern represents an external, native function that is callable from within the interpreter,
// during program execution.
type Extern struct {
	name   string
	pTypes []il.Type
	rType  il.Type

	v reflect.Value
}

// ExternFromFn creates a new reflection based Extern, based on the given function. It panics if the
// function signature is incompatible to be an extern.
//
// A function can be extern under following conditions:
// - Input parameter types are one of the supported types: string, bool, int64, float64, map[string]string.
// - The return types can be:
//   - none                                 (i.e. func (...) {...})
//   - a supported type                     (i.e. func (...) string {...})
//   - error                                (i.e. func (...) error {...})
//   - suported type and error              (i.e. func (...) string, error {...})
//
func ExternFromFn(name string, fn interface{}) Extern {
	t := reflect.TypeOf(fn)
	if t.Kind() != reflect.Func {
		panic(fmt.Errorf("not a function: '%v'", fn))
	}

	if t.NumOut() > 2 {
		panic(fmt.Errorf("number of output parameters can only be 0, 1 or 2"))
	}

	iErr := reflect.TypeOf((*error)(nil)).Elem()
	rt := il.Void
	switch t.NumOut() {
	case 0:

	case 1:
		if !t.Out(0).Implements(iErr) {
			rt = ilType(t.Out(0))
		}

	case 2:
		rt = ilType(t.Out(0))
		if !t.Out(1).Implements(iErr) {
			panic(fmt.Errorf("the second return value must be an error: '%v'", t))
		}

	default:
		panic(fmt.Errorf("more than two return values are not allowed:'%v'", t))
	}

	if rt == il.Unknown {
		panic(fmt.Errorf("incompatible type: '%v'", t.Out(0)))
	}

	pts := make([]il.Type, t.NumIn())
	for i := 0; i < t.NumIn(); i++ {
		pt := t.In(i)
		ilt := ilType(pt)
		if ilt == il.Unknown {
			panic(fmt.Errorf("incompatible type: '%v'", pt))
		}
		pts[i] = ilt
	}

	v := reflect.ValueOf(fn)

	return Extern{
		name:   name,
		pTypes: pts,
		rType:  rt,
		v:      v,
	}
}

func ilType(t reflect.Type) il.Type {
	switch t.Kind() {
	case reflect.String:
		return il.String
	case reflect.Bool:
		return il.Bool
	case reflect.Int64:
		return il.Integer
	case reflect.Float64:
		return il.Double
	case reflect.Map:
		if t.Key().Kind() == reflect.String || t.Elem().Kind() == reflect.String {
			return il.Record
		}
	}

	return il.Unknown
}

func (e Extern) invoke(s *il.StringsTable, h []interface{}, hp *uint32, stack []uint32, sp uint32) (uint32, uint32, error) {

	ins := make([]reflect.Value, len(e.pTypes))

	ap := sp - typesStackAllocSize(e.pTypes)
	for i := 0; i < len(e.pTypes); i++ {
		ins[i] = toValue(e.pTypes[i], s, h, hp, stack, &ap)
	}

	outs := e.v.Call(ins)
	var rv reflect.Value
	switch len(outs) {
	case 1:
		if e.rType != il.Void {
			rv = outs[0]
			break
		}
		i := outs[0].Interface()
		if i != nil {
			return 0, 0, i.(error)

		}
	case 2:
		rv = outs[0]
		i := outs[1].Interface()
		if i != nil {
			return 0, 0, i.(error)

		}
	}

	switch e.rType {
	case il.String:
		str := rv.String()
		id := s.GetID(str)
		return id, 0, nil

	case il.Bool:
		b := rv.Bool()
		var v uint32
		if b {
			v = 1
		}
		return v, 0, nil

	case il.Integer:
		i := rv.Int()
		return uint32(i >> 32), uint32(i & 0xFFFFFFFF), nil

	case il.Double:
		d := rv.Float()
		u64 := math.Float64bits(d)
		return uint32(u64 >> 32), uint32(u64 & 0xFFFFFFFF), nil

	case il.Record:
		// TODO(ozben): Single instance records
		r, done := rv.Interface().(map[string]string)
		if !done {
			return 0, 0, fmt.Errorf("unable to convert value to record: '%v'", r)
		}
		h[*hp] = r
		*hp++
		return *hp - 1, 0, nil

	case il.Void:
		return 0, 0, nil

	default:
		panic(fmt.Errorf("unrecognized type: %v", e.rType))
	}
}

func toValue(t il.Type, s *il.StringsTable, h []interface{}, hp *uint32, stack []uint32, ap *uint32) reflect.Value {
	var v reflect.Value

	switch t {
	case il.String:
		v = reflect.ValueOf(s.GetString(stack[*ap]))
		*ap += 2

	case il.Bool:
		b := true
		if stack[*ap] == 0 {
			b = false
		}
		v = reflect.ValueOf(b)
		*ap++

	case il.Integer:
		i := (int64(stack[*ap]) << 32) + int64(stack[*ap+1])
		v = reflect.ValueOf(i)
		*ap += 2

	case il.Double:
		d := math.Float64frombits((uint64(stack[*ap]) << 32) + uint64(stack[*ap+1]))
		v = reflect.ValueOf(d)
		*ap += 2

	case il.Record:
		r := h[stack[*ap]]
		v = reflect.ValueOf(r)
		*ap++

	default:
		panic(fmt.Errorf("unrecognized type: %v", t))
	}

	return v
}
