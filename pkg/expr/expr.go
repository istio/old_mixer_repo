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
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"

	"github.com/golang/glog"

	config "istio.io/api/mixer/v1/config/descriptor"
	"istio.io/mixer/pkg/attribute"
)

var tMap = map[token.Token]string{
	token.ILLEGAL: "ILLEGAL",

	token.EOF:     "EOF",
	token.COMMENT: "COMMENT",

	token.IDENT:  "IDENT",
	token.INT:    "INT",
	token.FLOAT:  "FLOAT",
	token.IMAG:   "IMAG",
	token.CHAR:   "CHAR",
	token.STRING: "STRING",

	token.ADD: "ADD",
	token.SUB: "SUB",
	token.MUL: "MUL",
	token.QUO: "QUO",
	token.REM: "REM",

	token.AND:  "AND",
	token.OR:   "OR",
	token.XOR:  "XOR",
	token.LAND: "LAND",
	token.LOR:  "LOR",

	token.EQL: "EQ",
	token.LSS: "LT",
	token.GTR: "GT",
	token.NOT: "NOT",

	token.NEQ: "NEQ",
	token.LEQ: "LEQ",
	token.GEQ: "GEQ",

	token.LBRACK: "INDEX",
}

var typeMap = map[token.Token]config.ValueType{
	token.INT:    config.INT64,
	token.FLOAT:  config.DOUBLE,
	token.CHAR:   config.STRING,
	token.STRING: config.STRING,
}

// Expression is a simplified expression AST
type Expression struct {
	// Oneof the following
	Const *Constant
	Var   *Variable
	Fn    *Function
}

// Eval evaluates the expression given an attribute bag.
func (e *Expression) Eval(attrs attribute.Bag, fMap map[string]Func) (interface{}, error) {
	if e.Const != nil {
		return e.Const.TypedValue, nil
	}
	if e.Var != nil {
		v, ok := attribute.Value(attrs, e.Var.Name)
		if !ok {
			return nil, fmt.Errorf("unresolved attribute %s", e.Var.Name)
		}
		return v, nil
	}
	// may panic
	return e.Fn.Eval(attrs, fMap)
}

// String produces postfix version with all operators converted to function names
func (e *Expression) String() string {
	if e.Const != nil {
		return e.Const.String()
	}
	if e.Var != nil {
		return e.Var.String()
	}
	if e.Fn != nil {
		return e.Fn.String()
	}
	return "<nil>"
}

// newConstant creates a new constant of given type.
// It also stores a typed form of the constant.
func newConstant(v string, vType config.ValueType) (*Constant, error) {
	var typedVal interface{}
	var err error
	switch vType {
	case config.INT64:
		if typedVal, err = strconv.ParseInt(v, 10, 64); err != nil {
			return nil, err
		}
	case config.DOUBLE:
		if typedVal, err = strconv.ParseFloat(v, 64); err != nil {
			return nil, err
		}
	default: // string
		if typedVal, err = strconv.Unquote(v); err != nil {
			return nil, err
		}
	}
	return &Constant{Value: v, Kind: vType, TypedValue: typedVal}, nil
}

// Constant models a typed constant.
type Constant struct {
	Value      string
	TypedValue interface{}
	Kind       config.ValueType
}

func (c *Constant) String() string {
	return fmt.Sprintf("%s", c.Value)
}

// Variable models a variable.
type Variable struct {
	Name string
}

func (v *Variable) String() string {
	return fmt.Sprintf("$%s", v.Name)
}

// Function models a function with multiple parameters
type Function struct {
	Name string
	Args []*Expression
}

func (f *Function) String() string {
	var w bytes.Buffer
	w.WriteString(f.Name + "(")
	for idx, arg := range f.Args {
		if idx != 0 {
			w.WriteString(", ")
		}
		w.WriteString(arg.String())
	}
	w.WriteString(")")
	return w.String()
}

// Eval evaluate function.
func (f *Function) Eval(attrs attribute.Bag, fMap map[string]Func) (interface{}, error) {
	fn := fMap[f.Name]
	if fn == nil {
		return nil, fmt.Errorf("unknown function: %s", f.Name)
	}
	// can throw NPE if config is not consistent.
	args := []interface{}{}
	for _, earg := range f.Args {
		arg, err := earg.Eval(attrs, fMap)
		if err != nil && !fn.NullArgs() {
			return nil, err
		}
		args = append(args, arg)
	}
	glog.V(2).Infof("calling %#v %#v", fn, args)
	return fn.Call(args), nil
}

func process(ex ast.Expr, tgt *Expression) (err error) {
	switch v := ex.(type) {
	case *ast.UnaryExpr:
		tgt.Fn = &Function{Name: tMap[v.Op]}
		if err = processFunc(tgt.Fn, []ast.Expr{v.X}); err != nil {
			return
		}
	case *ast.BinaryExpr:
		tgt.Fn = &Function{Name: tMap[v.Op]}
		if err = processFunc(tgt.Fn, []ast.Expr{v.X, v.Y}); err != nil {
			return
		}
	case *ast.CallExpr:
		vfunc, found := v.Fun.(*ast.Ident)
		if !found {
			return fmt.Errorf("unexpected expression: %#v", v.Fun)
		}
		tgt.Fn = &Function{Name: vfunc.Name}
		if err = processFunc(tgt.Fn, v.Args); err != nil {
			return
		}
	case *ast.ParenExpr:
		if err = process(v.X, tgt); err != nil {
			return
		}
	case *ast.BasicLit:
		tgt.Const, err = newConstant(v.Value, typeMap[v.Kind])
		if err != nil {
			return
		}
	case *ast.Ident:
		// true and false are treated as identifiers by parser
		// we need to convert them into constants here
		lv := strings.ToLower(v.Name)
		if lv == "true" || lv == "false" {
			typedVal := true
			if lv == "false" {
				typedVal = false
			}
			tgt.Const = &Constant{Value: lv, Kind: config.BOOL, TypedValue: typedVal}
		} else {
			tgt.Var = &Variable{Name: v.Name}
		}
	case *ast.SelectorExpr:
		// for selectorExpr length is guaranteed to be at least 2.
		var w []string
		if err = processSelectorExpr(v, &w); err != nil {
			return
		}
		var ww bytes.Buffer
		ww.WriteString(w[len(w)-1])
		for idx := len(w) - 2; idx >= 0; idx-- {
			ww.WriteString("." + w[idx])
		}
		tgt.Var = &Variable{Name: ww.String()}
	case *ast.IndexExpr:
		tgt.Fn = &Function{Name: tMap[token.LBRACK]}
		if err = processFunc(tgt.Fn, []ast.Expr{v.X, v.Index}); err != nil {
			return
		}
	default:
		return fmt.Errorf("unexpected expression: %#v", v)
	}

	return nil
}

func processSelectorExpr(exin *ast.SelectorExpr, w *[]string) (err error) {
	ex := exin
	for {
		*w = append(*w, ex.Sel.Name)
		switch v := ex.X.(type) {
		case *ast.SelectorExpr:
			ex = v
		case *ast.Ident:
			*w = append(*w, v.Name)
			return nil
		default:
			return fmt.Errorf("unexpected expression: %#v", v)
		}
	}
}

func processFunc(fn *Function, args []ast.Expr) (err error) {
	fAargs := []*Expression{}
	for _, ee := range args {
		aex := &Expression{}
		fAargs = append(fAargs, aex)
		if err = process(ee, aex); err != nil {
			return
		}
	}
	fn.Args = fAargs
	return nil
}

// Parse parses a given expression to ast.Expression.
func Parse(src string) (ex *Expression, err error) {
	a, err := parser.ParseExpr(src)
	if err != nil {
		return nil, fmt.Errorf("parse error: %s %s", src, err)
	}
	glog.V(2).Infof("\n\n%s : %#v\n", src, a)
	ex = &Expression{}
	if err = process(a, ex); err != nil {
		glog.Warningf("parser error: %s", ex)
		return nil, err
	}
	return ex, nil
}

// Evaluator interface
type cexl struct {
	//TODO add ast cache
	// function Map
	fMap map[string]Func
}

func (e *cexl) Eval(s string, attrs attribute.Bag) (ret interface{}, err error) {
	var ex *Expression
	if ex, err = Parse(s); err != nil {
		return
	}
	return ex.Eval(attrs, e.fMap)
}

// Eval evaluates given expression using the attribute bag to a string
func (e *cexl) EvalString(s string, attrs attribute.Bag) (ret string, err error) {
	var uret interface{}
	if uret, err = e.Eval(s, attrs); err != nil {
		return
	}
	return uret.(string), nil
}

func (e *cexl) EvalPredicate(s string, attrs attribute.Bag) (ret bool, err error) {
	var uret interface{}
	if uret, err = e.Eval(s, attrs); err != nil {
		return
	}
	return uret.(bool), nil
}

// Validate validates configuration
func (e *cexl) Validate(s string) (err error) {
	var ex *Expression
	if ex, err = Parse(s); err != nil {
		return
	}
	glog.V(2).Infof("%s --> %s", s, ex)
	return nil
}

// NewCexlEvaluator returns a new Evaluator of this type.
func NewCexlEvaluator() Evaluator {
	return &cexl{
		fMap: FuncMap(),
	}
}
