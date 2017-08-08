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

// Package compiler implements a compiler that converts Mixer's expression language into a
// Mixer IL-based program that can be executed via an interpreter.
package compiler

import (
	"fmt"

	dpb "istio.io/api/mixer/v1/config/descriptor"
	"istio.io/mixer/pkg/expr"
	"istio.io/mixer/pkg/il"
)

type generator struct {
	program *il.Program
	builder *il.Builder
	finder  expr.AttributeDescriptorFinder
	err     error
}

type nilHandling int

// Nil handling modes
const (
	// No nil handling
	nhNone nilHandling = iota

	// Jump to the given label if the value is present.
	nhJmpOnValue

	// Push a boolean indicating whether the value is nil or not as the last item.
	nhPushBool
)

// Result is returned as the result of compilation.
type Result struct {
	Program    *il.Program
	Expression *expr.Expression
}

// Compile converts the given expression text, into an IL based program.
func Compile(text string, finder expr.AttributeDescriptorFinder) (Result, error) {
	p := il.NewProgram()

	expression, err := expr.Parse(text)
	if err != nil {
		return Result{}, err
	}

	exprType, err := expression.EvalType(finder, expr.FuncMap())
	if err != nil {
		return Result{}, err
	}

	g := generator{
		program: p,
		builder: il.NewBuilder(p.Strings()),
		finder:  finder,
	}

	returnType := g.toIlType(exprType)
	g.generate(expression, 0, nhNone, "")
	if g.err != nil {
		return Result{}, err
	}

	g.builder.Ret()
	body := g.builder.Build()
	if err = g.program.AddFunction("eval", []il.Type{}, returnType, body); err != nil {
		g.internalError(err.Error())
		return Result{}, err
	}

	return Result{
		Program:    p,
		Expression: expression,
	}, nil
}

func (g *generator) toIlType(t dpb.ValueType) il.Type {
	switch t {
	case dpb.STRING:
		return il.String
	case dpb.BOOL:
		return il.Bool
	case dpb.INT64:
		return il.Integer
	case dpb.DOUBLE:
		return il.Double
	case dpb.STRING_MAP:
		return il.StringMap
	default:
		g.internalError("unhandled expression type: '%v'", t)
		return il.Unknown
	}
}

func (g *generator) evalType(e *expr.Expression) il.Type {
	dvt, _ := e.EvalType(g.finder, expr.FuncMap())
	return g.toIlType(dvt)
}

func (g *generator) generate(e *expr.Expression, depth int, nl nilHandling, label string) {
	switch {
	case e.Const != nil:
		g.generateConstant(e.Const)
	case e.Var != nil:
		g.generateVariable(e.Var, nl, label)
	case e.Fn != nil:
		g.generateFunction(e.Fn, depth, nl, label)
	default:
		g.internalError("unexpected expression type encountered.")
	}
}

func (g *generator) generateVariable(v *expr.Variable, nl nilHandling, label string) {
	i := g.finder.GetAttribute(v.Name)
	ilType := g.toIlType(i.ValueType)
	switch ilType {
	case il.Integer:
		switch nl {
		case nhNone:
			g.builder.ResolveInt(v.Name)
		case nhPushBool, nhJmpOnValue:
			g.builder.TResolveInt(v.Name)
		}

	case il.String:
		switch nl {
		case nhNone:
			g.builder.ResolveString(v.Name)
		case nhPushBool, nhJmpOnValue:
			g.builder.TResolveString(v.Name)
		}

	case il.Bool:
		switch nl {
		case nhNone:
			g.builder.ResolveBool(v.Name)
		case nhPushBool, nhJmpOnValue:
			g.builder.TResolveBool(v.Name)
		}

	case il.Double:
		switch nl {
		case nhNone:
			g.builder.ResolveDouble(v.Name)
		case nhPushBool, nhJmpOnValue:
			g.builder.TResolveDouble(v.Name)
		}

	case il.StringMap:
		switch nl {
		case nhNone:
			g.builder.ResolveMap(v.Name)
		case nhPushBool, nhJmpOnValue:
			g.builder.TResolveMap(v.Name)
		}

	default:
		g.internalError("unrecognized variable type: '%v'", i.ValueType)
	}

	if nl == nhJmpOnValue {
		g.builder.Jnz(label)
	}
}

func (g *generator) generateFunction(f *expr.Function, depth int, nl nilHandling, label string) {

	switch f.Name {
	case "EQ":
		g.generateEq(f, depth)
	case "NEQ":
		g.generateNeq(f, depth)
	case "LOR":
		g.generateLor(f, depth)
	case "LAND":
		g.generateLand(f, depth)
	case "INDEX":
		g.generateIndex(f, depth, nl, label)
	case "OR":
		g.generateOr(f, depth, nl, label)
	default:
		g.internalError("function not yet implemented: %s", f.Name)
	}
}

func (g *generator) generateEq(f *expr.Function, depth int) {
	exprType := g.evalType(f.Args[0])
	g.generate(f.Args[0], depth+1, nhNone, "")

	var constArg1 interface{}
	if f.Args[1].Const != nil {
		constArg1 = f.Args[1].Const.Value
	} else {
		g.generate(f.Args[1], depth+1, nhNone, "")
	}

	switch exprType {
	case il.Bool:
		if constArg1 != nil {
			g.builder.AEQBool(constArg1.(bool))
		} else {
			g.builder.EQBool()
		}

	case il.String:
		if constArg1 != nil {
			g.builder.AEQString(constArg1.(string))
		} else {
			g.builder.EQString()
		}

	case il.Integer:
		if constArg1 != nil {
			g.builder.AEQInteger(constArg1.(int64))
		} else {
			g.builder.EQInteger()
		}

	case il.Double:
		if constArg1 != nil {
			g.builder.AEQDouble(constArg1.(float64))
		} else {
			g.builder.EQDouble()
		}

	default:
		g.internalError("equality for type not yet implemented: %v", exprType)
	}
}

func (g *generator) generateNeq(f *expr.Function, depth int) {
	g.generateEq(f, depth+1)
	g.builder.Not()
}

func (g *generator) generateLor(f *expr.Function, depth int) {
	g.generate(f.Args[0], depth+1, nhNone, "")
	lr := g.builder.AllocateLabel()
	le := g.builder.AllocateLabel()
	g.builder.Jz(lr)
	g.builder.APushBool(true)
	if depth == 0 {
		g.builder.Ret()
	} else {
		g.builder.Jmp(le)
	}
	g.builder.SetLabelPos(lr)
	g.generate(f.Args[1], depth+1, nhNone, "")

	if depth != 0 {
		g.builder.SetLabelPos(le)
	}
}

func (g *generator) generateLand(f *expr.Function, depth int) {
	for _, a := range f.Args {
		g.generate(a, depth+1, nhNone, "")
	}

	g.builder.And()
}

func (g *generator) generateIndex(f *expr.Function, depth int, nl nilHandling, label string) {

	switch nl {
	case nhNone:
		g.generate(f.Args[0], depth+1, nhNone, "")

		if f.Args[1].Const != nil {
			str := f.Args[1].Const.Value.(string)
			g.builder.ALookup(str)
		} else {
			g.generate(f.Args[1], depth+1, nhNone, "")
			g.builder.Lookup()
		}

	case nhPushBool:
		lEnd := g.builder.AllocateLabel()
		lTargetResolved := g.builder.AllocateLabel()
		g.generate(f.Args[0], depth+1, nhJmpOnValue, lTargetResolved)
		g.builder.APushBool(false)
		g.builder.Jmp(lEnd)
		g.builder.SetLabelPos(lTargetResolved)

		if f.Args[1].Const != nil {
			str := f.Args[1].Const.Value.(string)
			g.builder.APushStr(str)
			g.builder.TLookup()
		} else {
			lArgumentResolved := g.builder.AllocateLabel()
			g.generate(f.Args[1], depth+1, nhJmpOnValue, lArgumentResolved)
			g.builder.APushBool(false)
			g.builder.Jmp(lEnd)
			g.builder.SetLabelPos(lArgumentResolved)
			g.builder.TLookup()
		}
		g.builder.SetLabelPos(lEnd)

	case nhJmpOnValue:
		lEnd := g.builder.AllocateLabel()
		lTargetResolved := g.builder.AllocateLabel()
		g.generate(f.Args[0], depth+1, nhJmpOnValue, lTargetResolved)
		g.builder.Jmp(lEnd)
		g.builder.SetLabelPos(lTargetResolved)

		if f.Args[1].Const != nil {
			str := f.Args[1].Const.Value.(string)
			g.builder.APushStr(str)
		} else {
			lArgumentResolved := g.builder.AllocateLabel()
			g.generate(f.Args[1], depth+1, nhJmpOnValue, lArgumentResolved)
			g.builder.Jmp(lEnd)
			g.builder.SetLabelPos(lArgumentResolved)
		}
		g.builder.TLookup()
		g.builder.Jnz(label)
		g.builder.SetLabelPos(lEnd)
	}
}

func (g *generator) generateOr(f *expr.Function, depth int, nl nilHandling, label string) {
	switch nl {
	case nhNone:
		lEnd := g.builder.AllocateLabel()
		g.generate(f.Args[0], depth+1, nhJmpOnValue, lEnd)

		if f.Args[1].Fn != nil && f.Args[1].Fn.Name == "OR" {
			g.generate(f.Args[1], depth+1, nhJmpOnValue, lEnd)
		} else {
			g.generate(f.Args[1], depth+1, nhNone, "")
		}
		g.builder.SetLabelPos(lEnd)

	case nhPushBool:
		lEnd := g.builder.AllocateLabel()
		lTrue := g.builder.AllocateLabel()
		g.generate(f.Args[0], depth+1, nhJmpOnValue, lTrue)
		g.generate(f.Args[1], depth+1, nhJmpOnValue, lTrue)
		g.builder.APushBool(false)
		g.builder.Jmp(lEnd)
		g.builder.SetLabelPos(lTrue)
		g.builder.APushBool(true)
		g.builder.SetLabelPos(lEnd)

	case nhJmpOnValue:
		g.generate(f.Args[0], depth+1, nhJmpOnValue, label)
		g.generate(f.Args[1], depth+1, nhJmpOnValue, label)
	}
}

func (g *generator) generateConstant(c *expr.Constant) {
	switch c.Type {
	case dpb.STRING:
		s := c.Value.(string)
		g.builder.APushStr(s)
	case dpb.BOOL:
		b := c.Value.(bool)
		g.builder.APushBool(b)
	case dpb.INT64:
		i := c.Value.(int64)
		g.builder.APushInt(i)
	case dpb.DOUBLE:
		d := c.Value.(float64)
		g.builder.APushDouble(d)
	default:
		g.internalError("unhandled constant type: %v", c.Type)
	}
}

func (g *generator) internalError(format string, args ...interface{}) {
	if g.err == nil {
		g.err = fmt.Errorf("internal compiler error -- %s", fmt.Sprintf(format, args...))
	}
}
