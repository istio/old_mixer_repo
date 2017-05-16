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
	"strings"
	"testing"

	"istio.io/mixer/pkg/il"
)

// test is a common struct used by many tests in this context.
type test struct {
	// code is the program input to the interpreter, in assembly form.
	code string

	// externs is the extern input to the interpreter.
	externs map[string]Extern

	// input is the input parameters to the evaluation.
	input map[string]interface{}

	// expected is the expected result value upon successful evaluation completion.
	expected interface{}

	// err is the expected error value upon unsuccessful evaluation completion.
	err string
}

func TestInterpreter_EvalFnID(t *testing.T) {
	p, _ := il.ReadText(`
	fn main() bool
		ipush_b false
		ret
	end
	`)

	i := New(p, map[string]Extern{})
	fnID := p.Functions.IDOf("main")
	r, e := i.EvalFnID(fnID, &bag{})

	if e != nil {
		t.Fatal(e)
	}
	if r.Interface() != false {
		t.Fatalf("unexpected output from program: '%v'", r.Interface())
	}
}

func TestInterpreter_Eval_FunctionNotFound(t *testing.T) {
	p, _ := il.ReadText(`
	fn main() bool
		ipush_b false
		ret
	end
	`)

	i := New(p, map[string]Extern{})
	_, e := i.Eval("foo", &bag{})
	if e == nil {
		t.Fatal("expected error during Eval()")
	}

	if e.Error() != "function not found: 'foo'" {
		t.Fatalf("unexpected error: '%v'", e)
	}
}

func TestInterpreter_Eval(t *testing.T) {

	var tests = map[string]test{
		"halt": {
			code: `
		fn main () integer
			halt
			ret
		end`,
			err: "catching fire as instructed",
		},

		"nop": {
			code: `
		fn main () void
			nop
			ret
		end`,
			expected: nil,
		},

		"err": {
			code: `
		fn main () integer
			err "woah!"
			ret
		end`,
			err: "woah!",
		},

		"errz/0": {
			code: `
		fn main () integer
			ipush_b false
			errz "woah!"
			ret
		end`,
			err: "woah!",
		},
		"errz/1": {
			code: `
		fn main () integer
			ipush_b true
			errz "woah!"
			ipush_i 10
			ret
		end`,
			expected: int64(10),
		},

		"errnz/0": {
			code: `
		fn main () integer
			ipush_b false
			errnz "woah!"
			ipush_i 10
			ret
		end`,
			expected: int64(10),
		},
		"errnz/1": {
			code: `
		fn main () integer
			ipush_b true
			errnz "woah!"
			ret
		end`,
			err: "woah!",
		},

		"pop_s": {
			code: `
		fn main () string
			ipush_s "foo"
			ipush_s "bar"
			pop_s
			ret
		end`,
			expected: "foo",
		},

		"pop_b": {
			code: `
		fn main () bool
			ipush_b true
			ipush_b false
			pop_b
			ret
		end`,
			expected: true,
		},

		"pop_i": {
			code: `
		fn main () integer
			ipush_i 49
			ipush_i 52
			pop_i
			ret
		end`,
			expected: int64(49),
		},

		"pop_d": {
			code: `
		fn main () double
			ipush_d 49.3
			ipush_d 52.7
			pop_d
			ret
		end`,
			expected: float64(49.3),
		},

		"dup_s": {
			code: `
		fn main () bool
			ipush_s "foo"
			dup_s
			eq_s
			ret
		end`,
			expected: true,
		},
		"dup_b": {
			code: `
		fn main () bool
			ipush_b false
			dup_b
			eq_b
			ret
		end`,
			expected: true,
		},
		"dup_i": {
			code: `
		fn main () bool
			ipush_i 42
			dup_i
			eq_i
			ret
		end`,
			expected: true,
		},
		"dup_d": {
			code: `
		fn main () bool
			ipush_d 123.987
			dup_d
			eq_d
			ret
		end`,
			expected: true,
		},

		"rload_s": {
			code: `
		fn main () string
			ipush_s "abc"
			rload_s r1
			ipush_s "def"
			rpush_s r1
			ret
		end`,
			expected: "abc",
		},
		"rload_b": {
			code: `
		fn main () bool
			ipush_b false
			rload_b r2
			ipush_b true
			rpush_b r2
			ret
		end`,
			expected: false,
		},
		"rload_i": {
			code: `
		fn main () integer
			ipush_i 42
			rload_i r2
			ipush_i 54
			rpush_i r2
			ret
		end`,
			expected: int64(42),
		},
		"rload_d": {
			code: `
		fn main () double
			ipush_d 42.4
			rload_d r2
			ipush_d 54.6
			rpush_d r2
			ret
		end`,
			expected: float64(42.4),
		},

		"iload_s": {
			code: `
		fn main () string
			iload_s r1 "abc"
			ipush_s "def"
			rpush_s r1
			ret
		end`,
			expected: "abc",
		},
		"iload_b": {
			code: `
		fn main () bool
			iload_b r2 false
			ipush_b true
			rpush_b r2
			ret
		end`,
			expected: false,
		},
		"iload_i": {
			code: `
		fn main () integer
			iload_i r2 42
			ipush_i 54
			rpush_i r2
			ret
		end`,
			expected: int64(42),
		},
		"iload_d": {
			code: `
		fn main () double
			iload_d r2 42.4
			ipush_d 54.6
			rpush_d r2
			ret
		end`,
			expected: float64(42.4),
		},

		"ipush_s": {
			code: `
		fn main () string
			ipush_s "aaa"
			ret
		end`,
			expected: "aaa",
		},
		"ipush_b/true": {
			code: `
		fn main () bool
			ipush_b true
			ret
		end`,
			expected: true,
		},
		"ipush_b/false": {
			code: `
		fn main () bool
			ipush_b false
			ret
		end`,
			expected: false,
		},
		"ipush_i": {
			code: `
		fn main () integer
			ipush_i 20
			ret
		end`,
			expected: int64(20),
		},
		"ipush_d": {
			code: `
			fn main () double
				ipush_d 43.34
				ret
			end`,
			expected: float64(43.34),
		},

		"eq_s/false": {
			code: `
		fn main () bool
			ipush_s "aaa"
			ipush_s "bbb"
			eq_s
			ret
		end`,
			expected: false,
		},
		"eq_s/true": {
			code: `
		fn main () bool
			ipush_s "aaa"
			ipush_s "aaa"
			eq_s
			ret
		end`,
			expected: true,
		},
		"eq_b/false": {
			code: `
		fn main () bool
			ipush_b false
			ipush_b true
			eq_b
			ret
		end`,
			expected: false,
		},
		"eq_b/true": {
			code: `
		fn main () bool
			ipush_b false
			ipush_b false
			eq_b
			ret
		end`,
			expected: true,
		},
		"eq_i/false": {
			code: `
		fn main () bool
			ipush_i 42
			ipush_i 24
			eq_i
			ret
		end`,
			expected: false,
		},
		"eq_i/true": {
			code: `
		fn main () bool
			ipush_i 23232
			ipush_i 23232
			eq_i
			ret
		end`,
			expected: true,
		},
		"eq_d/false": {
			code: `
		fn main () bool
			ipush_d 42.45
			ipush_d 24.87
			eq_d
			ret
		end`,
			expected: false,
		},
		"eq_d/true": {
			code: `
		fn main () bool
			ipush_d 23232.2323
			ipush_d 23232.2323
			eq_d
			ret
		end`,
			expected: true,
		},

		"ieq_s/false": {
			code: `
		fn main () bool
			ipush_s "aaa"
			ieq_s "bbb"
			ret
		end`,
			expected: false,
		},
		"ieq_s/true": {
			code: `
		fn main () bool
			ipush_s "aaa"
			ieq_s "aaa"
			ret
		end`,
			expected: true,
		},
		"ieq_b/false": {
			code: `
		fn main () bool
			ipush_b false
			ieq_b true
			ret
		end`,
			expected: false,
		},
		"ieq_b/true": {
			code: `
		fn main () bool
			ipush_b false
			ieq_b false
			ret
		end`,
			expected: true,
		},
		"ieq_i/false": {
			code: `
		fn main () bool
			ipush_i 42
			ieq_i 24
			ret
		end`,
			expected: false,
		},
		"ieq_i/true": {
			code: `
		fn main () bool
			ipush_i 23232
			ieq_i 23232
			ret
		end`,
			expected: true,
		},
		"ieq_d/false": {
			code: `
		fn main () bool
			ipush_d 42.45
			ieq_d 24.87
			ret
		end`,
			expected: false,
		},
		"ieq_d/true": {
			code: `
		fn main () bool
			ipush_d 23232.2323
			ieq_d 23232.2323
			ret
		end`,
			expected: true,
		},

		"xor/f/f": {
			code: `
		fn main () bool
			ipush_b false
			ipush_b false
			xor
			ret
		end`,
			expected: false,
		},
		"xor/t/f": {
			code: `
		fn main () bool
			ipush_b true
			ipush_b false
			xor
			ret
		end`,
			expected: true,
		},
		"xor/f/t": {
			code: `
		fn main () bool
			ipush_b false
			ipush_b true
			xor
			ret
		end`,
			expected: true,
		},
		"xor/t/t": {
			code: `
		fn main () bool
			ipush_b true
			ipush_b true
			xor
			ret
		end`,
			expected: false,
		},

		"or/f/f": {
			code: `
		fn main () bool
			ipush_b false
			ipush_b false
			or
			ret
		end`,
			expected: false,
		},
		"or/f/t": {
			code: `
		fn main () bool
			ipush_b false
			ipush_b true
			or
			ret
		end`,
			expected: true,
		},
		"or/t/f": {
			code: `
		fn main () bool
			ipush_b true
			ipush_b false
			or
			ret
		end`,
			expected: true,
		},
		"or/t/t": {
			code: `
		fn main () bool
			ipush_b true
			ipush_b true
			or
			ret
		end`,
			expected: true,
		},

		"and/f/f": {
			code: `
		fn main () bool
			ipush_b false
			ipush_b false
			and
			ret
		end`,
			expected: false,
		},
		"and/t/f": {
			code: `
		fn main () bool
			ipush_b true
			ipush_b false
			and
			ret
		end`,
			expected: false,
		},
		"and/f/t": {
			code: `
		fn main () bool
			ipush_b false
			ipush_b true
			and
			ret
		end`,
			expected: false,
		},
		"and/t/t": {
			code: `
		fn main () bool
			ipush_b true
			ipush_b true
			and
			ret
		end`,
			expected: true,
		},

		"ixor/f/f": {
			code: `
		fn main () bool
			ipush_b false
			ixor false
			ret
		end`,
			expected: false,
		},
		"ixor/t/f": {
			code: `
		fn main () bool
			ipush_b true
			ixor false
			ret
		end`,
			expected: true,
		},
		"ixor/f/t": {
			code: `
		fn main () bool
			ipush_b false
			ixor true
			ret
		end`,
			expected: true,
		},
		"ixor/t/t": {
			code: `
		fn main () bool
			ipush_b true
			ixor true
			ret
		end`,
			expected: false,
		},

		"ior/f/f": {
			code: `
		fn main () bool
			ipush_b false
			ior false
			ret
		end`,
			expected: false,
		},
		"ior/f/t": {
			code: `
		fn main () bool
			ipush_b false
			ior true
			ret
		end`,
			expected: true,
		},
		"ior/t/f": {
			code: `
		fn main () bool
			ipush_b true
			ior false
			ret
		end`,
			expected: true,
		},
		"ior/t/t": {
			code: `
		fn main () bool
			ipush_b true
			ior true
			ret
		end`,
			expected: true,
		},

		"iand/f/f": {
			code: `
		fn main () bool
			ipush_b false
			iand false
			ret
		end`,
			expected: false,
		},
		"iand/t/f": {
			code: `
		fn main () bool
			ipush_b true
			iand false
			ret
		end`,
			expected: false,
		},
		"iand/f/t": {
			code: `
		fn main () bool
			ipush_b false
			iand true
			ret
		end`,
			expected: false,
		},
		"iand/t/t": {
			code: `
		fn main () bool
			ipush_b true
			iand true
			ret
		end`,
			expected: true,
		},

		"not/f": {
			code: `
		fn main () bool
			ipush_b false
			not
			ret
		end`,
			expected: true,
		},
		"not/t": {
			code: `
		fn main () bool
			ipush_b true
			not
			ret
		end`,
			expected: false,
		},

		"resolve_s/success": {
			code: `
		fn main () string
			resolve_s "a"
			ret
		end`,
			input: map[string]interface{}{
				"a": "z",
			},
			expected: "z",
		},
		"resolve_s/not found": {
			code: `
		fn main () string
			resolve_s "q"
			ret
		end`,
			err: "lookup failed: q",
		},
		"resolve_s/type mismatch": {
			code: `
		fn main () string
			resolve_s "a"
			ret
		end`,
			input: map[string]interface{}{
				"a": true,
			},
			err: "error converting value to string: true",
		},
		"resolve_b/true": {
			code: `
		fn main () bool
			resolve_b "a"
			ret
		end`,
			input: map[string]interface{}{
				"a": true,
			},
			expected: true,
		},
		"resolve_b/false": {
			code: `
		fn main () bool
			resolve_b "a"
			ret
		end`,
			input: map[string]interface{}{
				"a": false,
			},
			expected: false,
		},
		"resolve_b/not found": {
			code: `
		fn main () bool
			resolve_b "q"
			ret
		end`,
			err: "lookup failed: q",
		},
		"resolve_b/type mismatch": {
			code: `
		fn main () bool
			resolve_b "a"
			ret
		end`,
			input: map[string]interface{}{
				"a": "A",
			},
			err: "error converting value to bool: A",
		},
		"resolve_i/success": {
			code: `
		fn main () integer
			resolve_i "a"
			ret
		end`,
			input: map[string]interface{}{
				"a": int64(123456),
			},
			expected: int64(123456),
		},
		"resolve_i/not found": {
			code: `
		fn main () integer
			resolve_i "q"
			ret
		end`,
			err: "lookup failed: q",
		},
		"resolve_i/type mismatch": {
			code: `
		fn main () integer
			resolve_i "a"
			ret
		end`,
			input: map[string]interface{}{
				"a": "B",
			},
			err: "error converting value to integer: B",
		},
		"resolve_d/success": {
			code: `
		fn main () double
			resolve_d "a"
			ret
		end`,
			input: map[string]interface{}{
				"a": float64(123.456),
			},
			expected: float64(123.456),
		},
		"resolve_d/not found": {
			code: `
		fn main () double
			resolve_d "q"
			ret
		end`,
			err: "lookup failed: q",
		},
		"resolve_d/type mismatch": {
			code: `
		fn main () double
			resolve_d "a"
			ret
		end`,
			input: map[string]interface{}{
				"a": true,
			},
			err: "error converting value to double: true",
		},
		"resolve_r/success": {
			code: `
		fn main () string
			resolve_r "a"
			ilookup "b"
			ret
		end`,
			input: map[string]interface{}{
				"a": map[string]string{"b": "c"},
			},
			expected: "c",
		},
		"resolve_r/not found": {
			code: `
		fn main () string
			resolve_r "q"
			ret
		end`,
			err: "lookup failed: q",
		},
		"resolve_r/type mismatch": {
			code: `
		fn main () string
			resolve_r "a"
			ret
		end`,
			input: map[string]interface{}{
				"a": true,
			},
			err: "error converting value to record: true",
		},

		"tresolve_s/success": {
			code: `
		fn main () string
			tresolve_s "a"
			errz "not found!"
			ret
		end`,
			input: map[string]interface{}{
				"a": "z",
			},
			expected: "z",
		},
		"tresolve_s/not found": {
			code: `
		fn main () string
			tresolve_s "q"
			errz "not found!"
			ret
		end`,
			err: "not found!",
		},
		"tresolve_s/type mismatch": {
			code: `
		fn main () string
			tresolve_s "a"
			errz "not found!"
			ret
		end`,
			input: map[string]interface{}{
				"a": true,
			},
			err: "error converting value to string: true",
		},
		"tresolve_b/true": {
			code: `
		fn main () bool
			tresolve_b "a"
			errz "not found!"
			ret
		end`,
			input: map[string]interface{}{
				"a": true,
			},
			expected: true,
		},
		"tresolve_b/false": {
			code: `
		fn main () bool
			tresolve_b "a"
			errz "not found!"
			ret
		end`,
			input: map[string]interface{}{
				"a": false,
			},
			expected: false,
		},
		"tresolve_b/not found": {
			code: `
		fn main () bool
			tresolve_b "q"
			errz "not found!"
			ret
		end`,
			err: "not found!",
		},
		"tresolve_b/type mismatch": {
			code: `
		fn main () bool
			tresolve_b "a"
			errz "not found!"
			ret
		end`,
			input: map[string]interface{}{
				"a": "A",
			},
			err: "error converting value to bool: A",
		},
		"tresolve_i/success": {
			code: `
		fn main () integer
			tresolve_i "a"
			errz "not found!"
			ret
		end`,
			input: map[string]interface{}{
				"a": int64(123456),
			},
			expected: int64(123456),
		},
		"tresolve_i/not found": {
			code: `
		fn main () integer
			tresolve_i "q"
			errz "not found!"
			ret
		end`,
			err: "not found!",
		},
		"tresolve_i/type mismatch": {
			code: `
		fn main () integer
			tresolve_i "a"
			errz "not found!"
			ret
		end`,
			input: map[string]interface{}{
				"a": "B",
			},
			err: "error converting value to integer: B",
		},
		"tresolve_d/success": {
			code: `
		fn main () double
			tresolve_d "a"
			errz "not found!"
			ret
		end`,
			input: map[string]interface{}{
				"a": float64(123.456),
			},
			expected: float64(123.456),
		},
		"tresolve_d/not found": {
			code: `
		fn main () double
			tresolve_d "q"
			errz "not found!"
			ret
		end`,
			err: "not found!",
		},
		"tresolve_d/type mismatch": {
			code: `
		fn main () integer
			tresolve_d "a"
			errz "not found!"
			ret
		end`,
			input: map[string]interface{}{
				"a": "B",
			},
			err: "error converting value to double: B",
		},
		"tresolve_r/success": {
			code: `
		fn main () string
			tresolve_r "a"
			errz "not found!"
			ilookup "b"
			ret
		end`,
			input: map[string]interface{}{
				"a": map[string]string{
					"b": "c",
				},
			},
			expected: "c",
		},
		"tresolve_r/not found": {
			code: `
		fn main () bool
			tresolve_r "q"
			errz "not found!"
		end`,
			err: "not found!",
		},
		"tresolve_r/type mismatch": {
			code: `
		fn main () bool
			tresolve_r "a"
			ret
		end`,
			input: map[string]interface{}{
				"a": "B",
			},
			err: "error converting value to record: B",
		},

		"lookup/success": {
			code: `
		fn main () string
			resolve_r "a"
			ipush_s "b"
			lookup
			ret
		end`,
			input: map[string]interface{}{
				"a": map[string]string{"b": "c"},
			},
			expected: "c",
		},
		"lookup/failure": {
			code: `
		fn main () string
			resolve_r "a"
			ipush_s "q"
			lookup
			ret
		end`,
			input: map[string]interface{}{
				"a": map[string]string{"b": "c"},
			},
			err: "member lookup failed: 'q'",
		},
		"ilookup/success": {
			code: `
		fn main () string
			resolve_r "a"
			ilookup "b"
			ret
		end`,
			input: map[string]interface{}{
				"a": map[string]string{"b": "c"},
			},
			expected: "c",
		},
		"ilookup/failure": {
			code: `
		fn main () string
			resolve_r "a"
			ilookup "q"
			ret
		end`,
			input: map[string]interface{}{
				"a": map[string]string{"b": "c"},
			},
			err: "member lookup failed: 'q'",
		},
		"tlookup/success": {
			code: `
		fn main () string
			resolve_r "a"
			ipush_s "b"
			tlookup
			errz "not found!"
			ret
		end`,
			input: map[string]interface{}{
				"a": map[string]string{"b": "c"},
			},
			expected: "c",
		},
		"tlookup/failure": {
			code: `
		fn main () string
			resolve_r "a"
			ipush_s "q"
			tlookup
			errz "not found!"
			ret
		end`,
			input: map[string]interface{}{
				"a": map[string]string{"b": "c"},
			},
			err: "not found!",
		},

		"add_i": {
			code: `
			fn main() integer
			  ipush_i 123
			  ipush_i 456
			  add_i
			  ret
		  	end`,
			expected: int64(579),
		},
		"add_i/neg/pos": {
			code: `
			fn main() integer
			  ipush_i -123
			  ipush_i 456
			  add_i
			  ret
		  	end`,
			expected: int64(333),
		},
		"sub_i": {
			code: `
			fn main() integer
			  ipush_i 456
			  ipush_i 123
			  sub_i
			  ret
		  	end`,
			expected: int64(333),
		},
		"sub_i/pos/neg": {
			code: `
			fn main() integer
			  ipush_i 456
			  ipush_i -123
			  sub_i
			  ret
		  	end`,
			expected: int64(579),
		},
		"iadd_i": {
			code: `
			fn main() integer
			  ipush_i 456
			  iadd_i 123
			  ret
		  	end`,
			expected: int64(579),
		},
		"iadd_i/pos/neg": {
			code: `
			fn main() integer
			  ipush_i 456
			  iadd_i -123
			  ret
		  	end`,
			expected: int64(333),
		},
		"isub_i": {
			code: `
			fn main() integer
			  ipush_i 456
			  isub_i 123
			  ret
		  	end`,
			expected: int64(333),
		},
		"isub_i/pos/neg": {
			code: `
			fn main() integer
			  ipush_i 456
			  isub_i -123
			  ret
		  	end`,
			expected: int64(579),
		},

		"add_d": {
			code: `
			fn main() double
			  ipush_d 123.123
			  ipush_d 456.456
			  add_d
			  ret
		  	end`,
			expected: float64(123.123) + float64(456.456),
		},
		"add_d/neg/pos": {
			code: `
			fn main() double
			  ipush_d -123.123
			  ipush_d 456.456
			  add_d
			  ret
		  	end`,
			expected: float64(456.456) + float64(-123.123),
		},
		"sub_d": {
			code: `
			fn main() double
			  ipush_d 456.456
			  ipush_d 123.123
			  sub_d
			  ret
		  	end`,
			expected: float64(456.456) - float64(123.123),
		},
		"sub_d/pos/neg": {
			code: `
			fn main() double
			  ipush_d 456.456
			  ipush_d -123.123
			  sub_d
			  ret
		  	end`,
			expected: float64(456.456) - float64(-123.123),
		},
		"iadd_d": {
			code: `
			fn main() double
			  ipush_d 456.456
			  iadd_d 123.123
			  ret
		  	end`,
			expected: float64(456.456) + float64(123.123),
		},
		"iadd_d/pos/neg": {
			code: `
			fn main() double
			  ipush_d 456.456
			  iadd_d -123.123
			  ret
		  	end`,
			expected: float64(456.456) + float64(-123.123),
		},
		"isub_d": {
			code: `
			fn main() double
			  ipush_d 456.456
			  isub_d 123.123
			  ret
		  	end`,
			expected: float64(456.456) - float64(123.123),
		},
		"isub_d/pos/neg": {
			code: `
			fn main() double
			  ipush_d 456.456
			  isub_d -123.123
			  ret
		  	end`,
			expected: float64(456.456) - float64(-123.123),
		},

		"jmp": {
			code: `
		fn main () bool
			ipush_b true
			jmp L1
			err "grasshopper is dead!"
		L1:
			ret
		end`,
			expected: true,
		},

		"jz/yes": {
			code: `
		fn main () bool
			ipush_b false
			jz L1
			err "grasshopper is dead!"
			ret
		L1:
			ipush_b true
			ret
		end`,
			expected: true,
		},
		"jz/no": {
			code: `
		fn main () bool
			ipush_b true
			jz L1
			ipush_b true
			ret
		L1:
			err "jumping and skateboarding not allowed!"
			ret
		end`,
			expected: true,
		},

		"jnz/yes": {
			code: `
		fn main () bool
			ipush_b true
			jnz L1
			err "grasshopper is dead!"
			ret
		L1:
			ipush_b true
			ret
		end`,
			expected: true,
		},
		"jnz/no": {
			code: `
		fn main () bool
			ipush_b false
			jnz L1
			ipush_b true
			ret
		L1:
			err "jumping and skateboarding not allowed!"
			ret
		end`,
			expected: true,
		},

		"eval/return/void": {
			code: `
		fn main () void
			ret
		end`,
			expected: nil,
		},
		"eval/return/bool": {
			code: `
		fn main () bool
			ipush_b true
			ret
		end`,
			expected: true,
		},
		"eval/return/string": {
			code: `
		fn main () string
			ipush_s "abc"
			ret
		end`,
			expected: "abc",
		},
		"eval/return/integer42": {
			code: `
		fn main () integer
			ipush_i 42
			ret
		end`,
			expected: int64(42),
		},
		"eval/return/integer0xF00000000": {
			code: `
		fn main () integer
			ipush_i 0xF00000000
			ret
		end`,
			expected: int64(0xF00000000),
		},
		"call/return/void": {
			code: `
		fn main() integer
			ipush_i 11
			call foo
			ipush_i 12
			ret
		end

		fn foo() void
			ipush_i 15
			ipush_i 18
			ret
		end
		`,
			expected: int64(12),
		},
		"call/return/string": {
			code: `
		fn main() string
			call foo
			ret
		end

		fn foo() string
			ipush_s "boo"
			ret
		end
		`,
			expected: "boo",
		},
		"call/return/integer1": {
			code: `
		fn main() integer
			call foo
			ret
		end

		fn foo() integer
			ipush_i 0x101
			ret
		end
		`,
			expected: int64(257),
		},
		"call/return/stackcleanup1": {
			code: `
		fn main() string
			call foo
			ret
		end

		fn foo() string
			ipush_s "boo"
			ipush_s "bar"
			ipush_s "baz"
			ret
		end
		`,
			expected: "baz",
		},
		"call/return/stackcleanup2": {
			code: `
		fn main() string
			ipush_s "zoo"
			call foo
			pop_s
			ret
		end

		fn foo() string
			ipush_s "boo"
			ipush_s "bar"
			ipush_s "baz"
			ret
		end
		`,
			expected: "zoo",
		},
		"extern/ret/string": {
			code: `
		fn main() string
			call ext
			ret
		end
		`,
			expected: "foo",
			externs: map[string]Extern{
				"ext": ExternFromFn("ext", func() string {
					return "foo"
				}),
			},
		},

		"extern/ret/int": {
			code: `
		fn main() integer
			call ext
			ret
		end
		`,
			expected: int64(42),
			externs: map[string]Extern{
				"ext": ExternFromFn("ext", func() int64 {
					return int64(42)
				}),
			},
		},

		"extern/ret/bool": {
			code: `
		fn main() bool
			call ext
			ret
		end
		`,
			expected: true,
			externs: map[string]Extern{
				"ext": ExternFromFn("ext", func() bool {
					return true
				}),
			},
		},

		"extern/ret/double": {
			code: `
		fn main() double
			call ext
			ret
		end
		`,
			expected: float64(567.789),
			externs: map[string]Extern{
				"ext": ExternFromFn("ext", func() float64 {
					return float64(567.789)
				}),
			},
		},

		"extern/ret/record": {
			code: `
		fn main() string
			call ext
			ilookup "b"
			ret
		end
		`,
			expected: "c",
			externs: map[string]Extern{
				"ext": ExternFromFn("ext", func() map[string]string {
					return map[string]string{"b": "c"}
				}),
			},
		},

		"extern/ret/void": {
			code: `
		fn main() integer
			ipush_i 42
			call ext
			ret
		end
		`,
			expected: int64(42),
			externs: map[string]Extern{
				"ext": ExternFromFn("ext", func() {
					return
				}),
			},
		},

		"extern/par/string": {
			code: `
		fn main() string
			ipush_s "ABC"
			call ext
			ret
		end
		`,
			expected: "ABCDEF",
			externs: map[string]Extern{
				"ext": ExternFromFn("ext", func(s string) string {
					return s + "DEF"
				}),
			},
		},
		"extern/par/bool/true": {
			code: `
		fn main() bool
			ipush_b true
			call ext
			ret
		end
		`,
			expected: false,
			externs: map[string]Extern{
				"ext": ExternFromFn("ext", func(b bool) bool {
					return !b
				}),
			},
		},
		"extern/par/bool/false": {
			code: `
		fn main() bool
			ipush_b false
			call ext
			ret
		end
		`,
			expected: true,
			externs: map[string]Extern{
				"ext": ExternFromFn("ext", func(b bool) bool {
					return !b
				}),
			},
		},
		"extern/par/integer": {
			code: `
		fn main() integer
			ipush_i 28
			call ext
			ret
		end
		`,
			expected: int64(56),
			externs: map[string]Extern{
				"ext": ExternFromFn("ext", func(i int64) int64 {
					return i * 2
				}),
			},
		},
		"extern/par/double": {
			code: `
		fn main() double
			ipush_d 5.612
			call ext
			ret
		end
		`,
			expected: float64(11.224),
			externs: map[string]Extern{
				"ext": ExternFromFn("ext", func(d float64) float64 {
					return d * 2
				}),
			},
		},
		"extern/par/record": {
			code: `
		fn main() string
			resolve_r "a"
			call ext
			ret
		end
		`,
			expected: "c",
			input: map[string]interface{}{
				"a": map[string]string{
					"b": "c",
				},
			},
			externs: map[string]Extern{
				"ext": ExternFromFn("ext", func(r map[string]string) string {
					return r["b"]
				}),
			},
		},
		"extern/err": {
			code: `
		fn main() string
			call ext
			ipush_s "foo"
			ret
		end
		`,
			err: "extern failure",
			externs: map[string]Extern{
				"ext": ExternFromFn("ext", func() error {
					return fmt.Errorf("extern failure")
				}),
			},
		},
		"extern/string/err": {
			code: `
		fn main() string
			call ext
			ret
		end
		`,
			err: "extern failure",
			externs: map[string]Extern{
				"ext": ExternFromFn("ext", func() (string, error) {
					return "", fmt.Errorf("extern failure")
				}),
			},
		},
		"extern/string/err/success": {
			code: `
		fn main() string
			call ext
			ret
		end
		`,
			expected: "aaa",
			externs: map[string]Extern{
				"ext": ExternFromFn("ext", func() (string, error) {
					return "aaa", nil
				}),
			},
		},
		"extern/not_found": {
			code: `
		fn main() string
			call ext
			ret
		end
		`,
			err: "function not found: 'ext'",
		},
		"benchmark": {
			code: `
		fn main () bool
			resolve_i "a"
			ieq_i 20
			jz LeftFalse
			ipush_b true
			ret
		LeftFalse:
			resolve_r "request.header"
			ilookup "host"
			ieq_s "abc"
			ret
		end
		`,
			input: map[string]interface{}{
				"a": int64(19),
				"request.header": map[string]string{
					"host": "abcd",
				},
			},
			expected: false,
		},
		"benchmark/success_at_A": {
			input: map[string]interface{}{
				"a": int64(20),
				"request.header": map[string]string{
					"host": "abcd",
				},
			},
			expected: true,
		},
		"benchmark/success_at_request_header": {
			input: map[string]interface{}{
				"a": int64(19),
				"request.header": map[string]string{
					"host": "abcd",
				},
			},
			expected: false,
		},
		"non-zero main params": {
			code: `
fn main (bool) void
  ipush_b true
  ret
end`,
			err: "init function must have 0 args",
		},

		// This test is to make sure that the overflow tests work correctly.
		"overflowtest": {
			code: `fn main() void
   ipush_i 2
L0:
   dup_i
   iadd_i 2
   dup_i
   ieq_i 62
   jz L0
   ipush_i 64
   // %s
   ret
end`,
			expected: nil,
		},
	}

	for n, test := range tests {
		code := test.code
		if len(test.code) == 0 {
			// Find the code from another test that has the same prefix.
			idx := strings.LastIndex(n, "/")
			if idx == -1 {
				t.Fatalf("unable parse the test name when looking for a parent: %s", n)
			}
			pn := n[0:idx]
			ptest, found := tests[pn]
			if !found {
				t.Fatalf("unable to find parent test: %s", pn)
			}
			code = ptest.code
		}

		test.code = code
		t.Run(n, func(tt *testing.T) {
			runTestCode(tt, test)
		})
	}
}

func TestInterpreter_Eval_Underflow(t *testing.T) {
	var tests = map[string]test{
		"errz": {
			code: `
fn main () bool
  errz "BBB"
end`,
		},
		"errnz": {
			code: `
fn main () bool
  errnz "AAA"
end`,
		},
		"ipop_s": {
			code: `
fn main () bool
  pop_s
end`,
		},
		"ipop_b": {
			code: `
fn main () bool
  pop_b
end`,
		},
		"ipop_i": {
			code: `
fn main () bool
  pop_i
end`,
		},
		"ipop_d": {
			code: `
fn main () bool
  pop_d
end`,
		},
		"dup_s": {
			code: `
fn main () bool
  dup_s
end`,
		},
		"dup_b": {
			code: `
fn main () bool
  dup_b
end`,
		},
		"dup_i": {
			code: `
fn main () bool
  dup_i
end`,
		},
		"dup_d": {
			code: `
fn main () bool
  dup_d
end`,
		},
		"rload_s": {
			code: `
fn main () bool
  rload_s r0
end`,
		},
		"rload_b": {
			code: `
fn main () bool
  rload_b r0
end`,
		},
		"rload_i": {
			code: `
fn main () bool
  rload_i r0
end`,
		},
		"rload_d": {
			code: `
fn main () bool
  rload_d r0
end`,
		},
		"eq_s": {
			code: `
fn main () bool
  eq_s
end`,
		},
		"eq_b": {
			code: `
fn main () bool
  eq_b
end`,
		},
		"eq_i": {
			code: `
fn main () bool
  eq_i
end`,
		},
		"eq_d": {
			code: `
fn main () bool
  eq_d
end`,
		},
		"ieq_s": {
			code: `
fn main () bool
  ieq_s "s"
end`,
		},
		"ieq_b": {
			code: `
fn main () bool
  ieq_b true
end`,
		},
		"ieq_i": {
			code: `
fn main () bool
  ieq_i 232
end`,
		},
		"ieq_d": {
			code: `
fn main () bool
  ieq_d 1234.54
end`,
		},
		"xor": {
			code: `
fn main () bool
  xor
end`,
		},
		"and": {
			code: `
fn main () bool
  and
end`,
		},
		"or": {
			code: `
fn main () bool
  or
end`,
		},
		"ixor": {
			code: `
fn main () bool
  ixor true
end`,
		},
		"iand": {
			code: `
fn main () bool
  iand true
end`,
		},
		"ior": {
			code: `
fn main () bool
  ior true
end`,
		},
		"add_i": {
			code: `
fn main () integer
  add_i
end`,
		},
		"sub_i": {
			code: `
fn main () integer
  sub_i
end`,
		},
		"add_d": {
			code: `
fn main () double
  add_d
end`,
		},
		"sub_d": {
			code: `
fn main () double
  sub_d
end`,
		},
		"iadd_i": {
			code: `
fn main () integer
  iadd_i 1
end`,
		},
		"isub_i": {
			code: `
fn main () integer
  isub_i 1
end`,
		},
		"iadd_d": {
			code: `
fn main () double
  iadd_d 1
end`,
		},
		"isub_d": {
			code: `
fn main () double
  isub_d 1
end`,
		},
		"jz": {
			code: `
fn main () bool
L0:
  jz L0
end`,
		},
		"jnz": {
			code: `
fn main () bool
L0:
  jnz L0
end`,
		},
		"ret": {
			code: `
		fn main () bool
		  ret
		end`,
		},
	}

	for n, test := range tests {
		test.err = "stack underflow"
		t.Run(n, func(tt *testing.T) {
			runTestCode(tt, test)
		})
	}
}

func TestInterpreter_Eval_Overflow(t *testing.T) {
	var tests = map[string]test{
		"rpush_s": {
			code: `rpush_s r0`,
		},
		"rpush_b": {
			code: `rpush_b r0`,
		},
		"rpush_i": {
			code: `rpush_i r0`,
		},
		"rpush_d": {
			code: `rpush_d r0`,
		},
		"dup_s": {
			code: `dup_s`,
		},
		"dup_b": {
			code: `dup_b`,
		},
		"dup_i": {
			code: `dup_i`,
		},
		"dup_d": {
			code: `dup_d`,
		},
		"ipush_s": {
			code: `ipush_s "AAA"`,
		},
		"ipush_b": {
			code: `ipush_b true`,
		},
		"ipush_i": {
			code: `ipush_i 1234`,
		},
		"ipush_d": {
			code: `ipush_d 1234`,
		},
	}

	template := `
fn main() void
   ipush_i 2
L0:
   dup_i
   iadd_i 2
   dup_i
   ieq_i 62
   jz L0
   ipush_i 64
   %s
   ret
end
`
	for n, test := range tests {
		test.err = "stack overflow"
		test.code = fmt.Sprintf(template, test.code)

		t.Run(n, func(tt *testing.T) {
			runTestCode(tt, test)
		})
	}
}

func runTestCode(t *testing.T, test test) {
	p := il.NewProgram()
	err := il.MergeText(test.code, p)
	if err != nil {
		t.Fatalf("code compilation failed: %v", err)
	}
	runTestProgram(t, p, test)
}

func runTestProgram(t *testing.T, p il.Program, test test) {
	s := NewStepper(p, test.externs)

	bag := &bag{attrs: test.input}
	for err := s.Begin("main", bag); !s.Done(); s.Step() {
		if err != nil {
			t.Fatal(s.Error())
		}
		t.Log(s)
	}
	if s.Error() != nil {
		if len(test.err) == 0 {
			t.Fatal(s.Error())
		}

		if test.err != s.Error().Error() {
			t.Fatalf("errors do not match: A:'%+v' != E: '%+v'", s.Error(), test.err)
		}
	} else {
		if len(test.err) != 0 {
			t.Fatalf("expected error not found: '%+v'", test.err)
		}

		r := s.Result()
		actual := r.Interface()
		if actual != test.expected {
			t.Fatalf("(stepper) result is not as expected: A:'%+v' != E:'%+v'", actual, test.expected)
		}
	}

	// Do the same thing with the interpreter directly:
	intr := New(p, test.externs)
	r, err := intr.Eval("main", bag)
	if err != nil {
		if len(test.err) == 0 {
			t.Fatal(s.Error())
		}

		if test.err != err.Error() {
			t.Fatalf("errors do not match: A:'%+v' != E: '%+v'", err, test.err)
		}
	} else {
		if len(test.err) != 0 {
			t.Fatalf("expected error not found: '%+v'", test.err)
		}

		actual := r.Interface()
		if actual != test.expected {
			t.Fatalf("(stepper) result is not as expected: A:'%+v' != E:'%+v'", actual, test.expected)
		}
	}
}
