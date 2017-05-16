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

package expr

import (
	"errors"
	"strings"
	"testing"

	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/il"
	"istio.io/mixer/pkg/il/interpreter"
)

// run micro benchmark using expression evaluator.
// Results vs hand coded expression eval function
// 2017-03-16
/*
$ go test -run=^$  -bench=.  -benchmem
LOR(EQ($a, 20), EQ(INDEX($request.header, "host"), "abc"))
BenchmarkExpressionAST/ok_1stAST-8         	10000000	       188 ns/op	      16 B/op	       3 allocs/op
BenchmarkExpressionAST/ok_2ndAST-8         	 3000000	       446 ns/op	      32 B/op	       5 allocs/op
BenchmarkExpressionAST/ok_notfoundAST-8    	 3000000	       435 ns/op	      32 B/op	       5 allocs/op
LOR(EQ($a, 20), EQ(INDEX($request.header, "host"), "abc"))
BenchmarkExpressionDirect/ok_1stDirect-8   	100000000	        12.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkExpressionDirect/ok_2ndDirect-8   	50000000	        31.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkExpressionDirect/ok_notfoundDirect-8         	50000000	        29.5 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	istio.io/mixer/pkg/expr	10.049s
*/

type exprFunc func(a attribute.Bag) (bool, error)

// dff implements `a == 20 || request.header["host"] == "abc"`
// this is done so we can compare Expr processed expression with
// raw direct golang performance.
func dff(a attribute.Bag) (bool, error) {
	var v interface{}
	var b bool
	if v, b = a.Get("a"); !b {
		return false, errors.New("a not found")
	}
	aa := v.(int64)
	if aa == 20 {
		return true, nil
	}

	if v, b = a.Get("request.header"); !b {
		return false, errors.New("a not found")
	}
	ss := v.(map[string]string)["host"]
	if ss == "abc" {
		return true, nil
	}
	return false, nil
}

var ilCode = `
fn eval () bool
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
`
var program il.Program = buildProgram()

func buildProgram() il.Program {
	p, err := il.ReadText(ilCode)
	if err != nil {
		panic(err)
	}
	return p
}

var i *interpreter.Interpreter = buildVM()

var fnID = program.Functions.IDOf("eval")

func buildVM() *interpreter.Interpreter {
	return interpreter.New(program, map[string]interpreter.Extern{})
}

func ilf(a attribute.Bag) (interpreter.Result, error) {
	return i.EvalFnID(fnID, a)
}

func BenchmarkExpressionAST(b *testing.B) {
	benchmarkExpression(b, "AST")
}

func BenchmarkExpressionDirect(b *testing.B) {
	benchmarkExpression(b, "Direct")
}

func BenchmarkExpressionVM(b *testing.B) {
	benchmarkExpression(b, "VM")
}

func benchmarkExpression(b *testing.B, stype string) {
	success := "_SUCCESS_"
	exprStr := `a == 20 || request.header["host"] == "abc"`
	exf, _ := Parse(exprStr)

	b.Logf("%s\n", exf.String())
	fm := FuncMap()
	tests := []struct {
		name   string
		tmap   map[string]interface{}
		result bool
		err    string
		df     exprFunc
		ex     *Expression
		il     func(attribute.Bag) (interpreter.Result, error)
	}{
		{"ok_1st", map[string]interface{}{
			"a": int64(20),
			"request.header": map[string]string{
				"host": "abc",
			},
		}, true, success, dff, exf, ilf,
		},
		{"ok_2nd", map[string]interface{}{
			"a": int64(2),
			"request.header": map[string]string{
				"host": "abc",
			},
		}, true, success, dff, exf, ilf,
		},
		{"ok_notfound", map[string]interface{}{
			"a": int64(2),
			"request.header": map[string]string{
				"host": "abcd",
			},
		}, false, success, dff, exf, ilf,
		},
	}

	//assertNoError
	assertNoError := func(err error, errStr string, ret interface{}, want bool, t *testing.B) {
		if (err == nil) != (errStr == success) {
			t.Errorf("got %s, want %s", err, errStr)
		}
		// check if error is of the correct type
		if err != nil {
			if !strings.Contains(err.Error(), errStr) {
				t.Errorf("got %s, want %s", err, errStr)
			}
			return
		}
		// check result
		if ret != want {
			t.Errorf("got %v, want %v", ret, want)
		}
	}

	for _, tst := range tests {

		attrs := &bag{attrs: tst.tmap}

		if stype == "AST" {
			ii, err := tst.ex.Eval(attrs, fm)
			assertNoError(err, tst.err, ii, tst.result, b)
			b.Run(tst.name+"AST", func(bb *testing.B) {
				for n := 0; n < bb.N; n++ {
					_, _ = tst.ex.Eval(attrs, fm)
				}
			})
		} else if stype == "Direct" {
			ii, err := tst.df(attrs)
			assertNoError(err, tst.err, ii, tst.result, b)
			b.Run(tst.name+"Direct", func(bb *testing.B) {
				for n := 0; n < bb.N; n++ {
					_, _ = tst.df(attrs)
				}
			})
		} else {
			ii, err := tst.il(attrs)
			var iii bool
			if err == nil {
				iii = ii.Bool()
			}
			assertNoError(err, tst.err, iii, tst.result, b)
			b.Run(tst.name+"VM", func(bb *testing.B) {
				for n := 0; n < bb.N; n++ {
					_, _ = tst.il(attrs)
				}
			})
		}
	}
}
