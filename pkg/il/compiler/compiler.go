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
	prg il.Program
	bld *il.Builder
	fnd expr.AttributeDescriptorFinder
	err error
}

// Compile compiles the given expression text, and return the corresponding program.
func Compile(text string, finder expr.AttributeDescriptorFinder) (il.Program, error) {
	p := il.NewProgram()

	ex, err := expr.Parse(text)
	if err != nil {
		return p, err
	}

	rvt, err := ex.EvalType(finder, expr.FuncMap())
	if err != nil {
		return p, err
	}

	g := generator{
		prg: p,
		bld: il.NewBuilder(p.Strings()),
		fnd: finder,
	}

	rt := g.toIlType(rvt)
	g.generate(ex, 0, false)

	g.bld.Ret()
	body := g.bld.Build()
	err = g.prg.AddFunction("eval", []il.Type{}, rt, body)
	if err != nil {
		g.internalError(err.Error())
	}

	return p, g.err
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
		return il.Record
	default:
		g.internalError("unhandled expression type: '%v'", t)
		return il.Unknown
	}
}

func (g *generator) evalType(e *expr.Expression) il.Type {
	dvt, _ := e.EvalType(g.fnd, expr.FuncMap())
	return g.toIlType(dvt)
}

func (g *generator) generate(e *expr.Expression, depth int, nullable bool) {
	switch {
	case e.Const != nil:
		g.generateConstant(e.Const)
	case e.Var != nil:
		g.generateVariable(e.Var, nullable)
	case e.Fn != nil:
		g.generateFunction(e.Fn, depth, nullable)
	default:
		g.internalError("unexpected expression type encountered.")
	}
}

func (g *generator) generateVariable(v *expr.Variable, nullable bool) {
	i := g.fnd.GetAttribute(v.Name)
	ilType := g.toIlType(i.ValueType)
	switch ilType {
	case il.Integer:
		if nullable {
			g.bld.TResolveInt(v.Name)
		} else {
			g.bld.ResolveInt(v.Name)
		}
	case il.String:
		if nullable {
			g.bld.TResolveString(v.Name)
		} else {
			g.bld.ResolveString(v.Name)
		}
	case il.Bool:
		if nullable {
			g.bld.TResolveBool(v.Name)
		} else {
			g.bld.ResolveBool(v.Name)
		}
	case il.Double:
		if nullable {
			g.bld.TResolveDouble(v.Name)

		} else {
			g.bld.ResolveDouble(v.Name)
		}
	case il.Record:
		if nullable {
			g.bld.TResolveRecord(v.Name)
		} else {
			g.bld.ResolveRecord(v.Name)
		}
	default:
		g.internalError("unrecognized variable type: '%v'", i.ValueType)
	}
}

func (g *generator) generateFunction(f *expr.Function, depth int, nullable bool) {

	switch f.Name {
	case "EQ":
		g.generateEq(f, depth, nullable)
	case "NEQ":
		g.generateNeq(f, depth)
	case "LOR":
		g.generateLor(f, depth)
	case "LAND":
		g.generateLand(f, depth)
	case "INDEX":
		g.generateIndex(f, depth)
	case "OR":
		g.generateOr(f, depth, nullable)
	default:
		g.internalError("function not yet implemented: %s", f.Name)
	}
}

func (g *generator) generateEq(f *expr.Function, depth int, nullable bool) {
	exprType := g.evalType(f.Args[0])
	g.generate(f.Args[0], depth+1, false)

	var constArg1 interface{}
	if f.Args[1].Const != nil {
		constArg1 = f.Args[1].Const.Value
	} else {
		g.generate(f.Args[1], depth+1, false)
	}

	switch exprType {
	case il.Bool:
		if constArg1 != nil {
			g.bld.IEQBool(constArg1.(bool))
		} else {
			g.bld.EQBool()
		}

	case il.String:
		if constArg1 != nil {
			g.bld.IEQString(constArg1.(string))
		} else {
			g.bld.EQString()
		}

	case il.Integer:
		if constArg1 != nil {
			g.bld.IEQInteger(constArg1.(int64))
		} else {
			g.bld.EQInteger()
		}

	case il.Double:
		if constArg1 != nil {
			g.bld.IEQDouble(constArg1.(float64))
		} else {
			g.bld.EQDouble()
		}

	default:
		g.internalError("equality for type not yet implemented: %v", exprType)
	}
}

func (g *generator) generateNeq(f *expr.Function, depth int) {
	g.generateEq(f, depth+1, false)
	g.bld.Not()
}

func (g *generator) generateLor(f *expr.Function, depth int) {
	g.generate(f.Args[0], depth+1, false)
	lr := g.bld.AllocateLabel()
	le := g.bld.AllocateLabel()
	g.bld.Jz(lr)
	g.bld.IPushBool(true)
	if depth == 0 {
		g.bld.Ret()
	} else {
		g.bld.Jmp(le)
	}
	g.bld.SetLabelPos(lr)
	g.generate(f.Args[1], depth+1, false)

	if depth != 0 {
		g.bld.SetLabelPos(le)
	}
}

func (g *generator) generateLand(f *expr.Function, depth int) {
	for _, a := range f.Args {
		g.generate(a, depth+1, false)
	}

	g.bld.And()
}

func (g *generator) generateIndex(f *expr.Function, depth int) {
	g.generate(f.Args[0], depth+1, false)

	if f.Args[1].Const != nil {
		str := f.Args[1].Const.Value.(string)
		g.bld.ILookup(str)

	} else {
		g.generate(f.Args[1], depth+1, false)
		g.bld.Lookup()

	}
}

func (g *generator) generateOr(f *expr.Function, depth int, nullable bool) {
	if nullable {
		l := g.bld.AllocateLabel()
		le := g.bld.AllocateLabel()
		g.generate(f.Args[0], depth+1, true)
		g.bld.Jz(l)
		g.bld.IPushBool(true)
		g.bld.Jmp(le)
		g.bld.SetLabelPos(l)
		g.generate(f.Args[1], depth+1, true)
		g.bld.SetLabelPos(le)
	} else {
		le := g.bld.AllocateLabel()
		g.generate(f.Args[0], depth+1, true)
		g.bld.Jnz(le)
		g.generate(f.Args[1], depth+1, false)
		g.bld.SetLabelPos(le)
	}
}

func (g *generator) generateConstant(c *expr.Constant) {
	switch c.Type {
	case dpb.STRING:
		s := c.Value.(string)
		g.bld.IPushStr(s)
	case dpb.BOOL:
		b := c.Value.(bool)
		g.bld.IPushBool(b)
	case dpb.INT64:
		i := c.Value.(int64)
		g.bld.IPushInt(i)
	case dpb.DOUBLE:
		d := c.Value.(float64)
		g.bld.IPushDouble(d)
	default:
		g.internalError("unhandled constant type: %v", c.Type)
	}
}

func (g *generator) internalError(format string, args ...interface{}) {
	if g.err == nil {
		g.err = fmt.Errorf("internal compiler error -- %s", fmt.Sprintf(format, args...))
	}
}
