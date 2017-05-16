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

package il

import (
	"fmt"
)

// Builder is used for building function bodies in a programmatic way.
type Builder struct {
	strings *StringsTable
	body    []uint32
	labels  map[string]uint32
	fixups  map[string][]uint32
}

// NewBuilder creates a new Builder instance. It uses the given StringsTable when it needs to
// allocate ids for strings.
func NewBuilder(s *StringsTable) *Builder {
	return &Builder{
		strings: s,
		body:    make([]uint32, 0),

		// labels maps the label name to the target address. If the position is not set yet, then
		// the label position be marked as 0, and any usages of the labels needs to be fixed up w
		labels: make(map[string]uint32),

		// fixups holds the bytecode locations that needs to be updated once a label position is set.
		fixups: make(map[string][]uint32),
	}
}

// Build completes building a function body and returns the accumulated byte code.
func (f *Builder) Build() []uint32 {
	return f.body
}

// Nop appends the "nop" instruction to the byte code.
func (f *Builder) Nop() {
	f.op0(Nop)
}

// Ret appends the "ret" instruction to the byte code.
func (f *Builder) Ret() {
	f.op0(Ret)
}

// Call appends the "call" instruction to the byte code.
func (f *Builder) Call(fnName string) {
	f.op1(Call, f.id(fnName))
}

// ResolveInt appends the "resolve_i" instruction to the byte code.
func (f *Builder) ResolveInt(n string) {
	f.op1(ResolveI, f.id(n))
}

// TResolveInt appends the "tresolve_i" instruction to the byte code.
func (f *Builder) TResolveInt(n string) {
	f.op1(TResolveI, f.id(n))
}

// ResolveString appends the "resolve_s" instruction to the byte code.
func (f *Builder) ResolveString(n string) {
	f.op1(ResolveS, f.id(n))
}

// TResolveString appends the "tresolve_s" instruction to the byte code.
func (f *Builder) TResolveString(n string) {
	f.op1(TResolveS, f.id(n))
}

// ResolveBool appends the "resolve_b" instruction to the byte code.
func (f *Builder) ResolveBool(n string) {
	f.op1(ResolveB, f.id(n))
}

// TResolveBool appends the "tresolve_b" instruction to the byte code.
func (f *Builder) TResolveBool(n string) {
	f.op1(TResolveB, f.id(n))
}

// ResolveDouble appends the "resolve_d" instruction to the byte code.
func (f *Builder) ResolveDouble(n string) {
	f.op1(ResolveD, f.id(n))
}

// TResolveDouble appends the "tresolve_d" instruction to the byte code.
func (f *Builder) TResolveDouble(n string) {
	f.op1(TResolveD, f.id(n))
}

// ResolveRecord appends the "resolve_r" instruction to the byte code.
func (f *Builder) ResolveRecord(n string) {
	f.op1(ResolveM, f.id(n))
}

// TResolveRecord appends the "tresolve_r" instruction to the byte code.
func (f *Builder) TResolveRecord(n string) {
	f.op1(TResolveM, f.id(n))
}

// IPushBool appends the "ipush_b" instruction to the byte code.
func (f *Builder) IPushBool(b bool) {
	f.op1(APushB, boolToOpcode(b))
}

// IPushStr appends the "ipush_s" instruction to the byte code.
func (f *Builder) IPushStr(s string) {
	f.op1(APushS, f.id(s))
}

// IPushInt appends the "ipush_i" instruction to the byte code.
func (f *Builder) IPushInt(i int64) {
	a1, a2 := integerToOpcode(i)
	f.op2(APushI, a1, a2)
}

// IPushDouble appends the "ipush_d" instruction to the byte code.
func (f *Builder) IPushDouble(n float64) {
	a1, a2 := doubleToOpcode(n)
	f.op2(APushD, a1, a2)
}

// Xor appends the "xor" instruction to the byte code.
func (f *Builder) Xor() {
	f.op0(Xor)
}

// EQString appends the "eq_s" instruction to the byte code.
func (f *Builder) EQString() {
	f.op0(EqS)
}

// IEQString appends the "ieq_s" instruction to the byte code.
func (f *Builder) IEQString(v string) {
	f.op1(AEqS, f.id(v))
}

// EQBool appends the "eq_b" instruction to the byte code.
func (f *Builder) EQBool() {
	f.op0(EqB)
}

// IEQBool appends the "ieq_b" instruction to the byte code.
func (f *Builder) IEQBool(v bool) {
	f.op1(AEqB, boolToOpcode(v))
}

// EQInteger appends the "eq_i" instruction to the byte code.
func (f *Builder) EQInteger() {
	f.op0(EqI)
}

// IEQInteger appends the "eq_i" instruction to the byte code.
func (f *Builder) IEQInteger(v int64) {
	a1, a2 := integerToOpcode(v)
	f.op2(AEqI, a1, a2)
}

// EQDouble appends the "eq_d" instruction to the byte code.
func (f *Builder) EQDouble() {
	f.op0(EqD)
}

// IEQDouble appends the "ieq_d" instruction to the byte code.
func (f *Builder) IEQDouble(v float64) {
	a1, a2 := doubleToOpcode(v)
	f.op2(AEqD, a1, a2)
}

// Not appends the "not" instruction to the byte code.
func (f *Builder) Not() {
	f.op0(Not)
}

// Or appends the "or" instruction to the byte code.
func (f *Builder) Or() {
	f.op0(Or)
}

// And appends the "and" instruction to the byte code.
func (f *Builder) And() {
	f.op0(And)
}

// Lookup appends the "lookup" instruction to the byte code.
func (f *Builder) Lookup() {
	f.op0(Lookup)
}

// ILookup appends the "ilookup" instruction to the byte code.
func (f *Builder) ILookup(v string) {
	f.op1(ALookup, f.id(v))
}

// AllocateLabel allocates a new label value for use within the code.
func (f *Builder) AllocateLabel() string {
	l := fmt.Sprintf("L%d", len(f.labels))
	f.labels[l] = 0
	return l
}

// SetLabelPos puts the label position at the current bytecode point that builder is pointing at.
// Panics if the label position was already set.
func (f *Builder) SetLabelPos(label string) {
	if f.labels[label] != 0 {
		panic("interpreter.Builder: setting the label position twice.")
	}
	adr := uint32(len(f.body))
	f.labels[label] = adr
	if fixups, found := f.fixups[label]; found {
		for _, fixup := range fixups {
			f.body[fixup] = adr
		}
	}
}

// Jz appends "jz" instruction to the byte code, against the given label.
func (f *Builder) Jz(label string) {
	adr := f.labels[label]
	f.op1(Jz, adr)
	if adr == 0 {
		fixup := uint32(len(f.body) - 1)
		f.fixups[label] = append(f.fixups[label], fixup)
	}
}

// Jnz appends "jnz" instruction to the byte code, against the given label.
func (f *Builder) Jnz(label string) {
	adr := f.labels[label]
	f.op1(Jnz, adr)
	if adr == 0 {
		fixup := uint32(len(f.body) - 1)
		f.fixups[label] = append(f.fixups[label], fixup)
	}
}

// Jmp appends "jmp" instruction to the byte code, against the given label.
func (f *Builder) Jmp(label string) {
	adr := f.labels[label]
	f.op1(Jmp, adr)
	if adr == 0 {
		fixup := uint32(len(f.body) - 1)
		f.fixups[label] = append(f.fixups[label], fixup)
	}
}

// op0 inserts a 0-arg opcode into the body.
func (f *Builder) op0(op Opcode) {
	f.body = append(f.body, uint32(op))
}

// op1 inserts a 1-arg opcode into the body.
func (f *Builder) op1(op Opcode, p1 uint32) {
	f.body = append(f.body, uint32(op))
	f.body = append(f.body, p1)
}

// op2 inserts a 2-arg opcode into the body.
func (f *Builder) op2(op Opcode, p1 uint32, p2 uint32) {
	f.body = append(f.body, uint32(op))
	f.body = append(f.body, p1)
	f.body = append(f.body, p2)
}

func (f *Builder) id(s string) uint32 {
	return f.strings.GetID(s)
}
