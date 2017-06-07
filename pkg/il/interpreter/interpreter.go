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

// Package interpreter implements an interpreter based runtime for the Mixer IL. Typically a user
// creates a program, in IL form, and creates an Interpreter, by calling interpeter.New, which takes
// a program, and its external, native bindings as input.
//
// Once an interpreter with a program is created, it can be used for multiple evaluation sessions.
// The evaluation is invoked by calling Interpreter.Eval or Interpreter.EvalFnID, which takes
// the name, or the id of the string of the name, as well as an attribute bag as input.
//
// The return type is a result, which is optimized for returning values directly from the Interpreter's
// internal data model.
//
// To help with debugging, the user can use the Stepper, which performs the same operations as
// Interpreter.Run, but stops and captures the full state of execution between instruction executions
// and allow the user to introspec them.
//
package interpreter

import (
	"fmt"

	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/il"
)

//go:generate go tool clang -E -P templates/interpreterRun.c >interpreterRunGenerated.go

const (
	callStackSize = 64
	opStackSize   = 64
	registerCount = 4
	heapSize      = 64
)

// Interpreter is an interpreted execution engine for the Mixer IL.
type Interpreter struct {
	program il.Program
	code    []uint32
	externs map[string]Extern
	stepper *Stepper
}

// New returns a new Interpreter instance, that can execute the provided program.
func New(p il.Program, es map[string]Extern) *Interpreter {

	return newIntr(p, es, nil)
}

// Eval evaluates the function with the given name, against the bag.
func (i *Interpreter) Eval(fnName string, bag attribute.Bag) (Result, error) {
	fn := i.program.Functions.Get(fnName)
	if fn == nil {
		return Result{}, fmt.Errorf("function not found: '%s'", fnName)
	}

	return i.run(fn, bag, false)
}

// EvalFnID evaluates the function with the given id, against the bag.
func (i *Interpreter) EvalFnID(fnID uint32, bag attribute.Bag) (Result, error) {
	fn := i.program.Functions.GetByID(fnID)

	return i.run(fn, bag, false)
}

func newIntr(p il.Program, es map[string]Extern, s *Stepper) *Interpreter {
	i := Interpreter{
		program: p,
		code:    p.ByteCode(),
		externs: es,
		stepper: s,
	}

	// TODO(ozben): This requires rationalization.
	for _, e := range es {
		p.AddExternDef(e.name, e.pTypes, e.rType)
	}

	return &i
}

func typeStackAllocSize(t il.Type) uint32 {
	switch t {
	case il.Void:
		return 0
	case il.Integer:
		fallthrough
	case il.Double:
		return 2
	case il.String:
		fallthrough
	case il.Bool:
		fallthrough
	case il.Record:
		return 1
	default:
		panic(fmt.Errorf("unknown type: '%v'", t))
	}
}

func typesStackAllocSize(ts []il.Type) uint32 {
	var result uint32
	for _, t := range ts {
		result += typeStackAllocSize(t)
	}
	return result
}
