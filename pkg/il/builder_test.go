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
	"strings"
	"testing"
)

type builderTest struct {
	i func(*Builder)
	e string
}

var builderTests = []builderTest{
	{
		i: func(b *Builder) {
			b.And()
		},
		e: `
fn main() void
  and
end`,
	},
	{
		i: func(b *Builder) {
			b.Xor()
		},
		e: `
fn main() void
  xor
end`,
	},
	{
		i: func(b *Builder) {
			b.Not()
		},
		e: `
fn main() void
  not
end`,
	},
	{
		i: func(b *Builder) {
			b.Or()
		},
		e: `
fn main() void
  or
end`,
	},
	{
		i: func(b *Builder) {
			b.EQString()
		},
		e: `
fn main() void
  eq_s
end`,
	},
	{
		i: func(b *Builder) {
			b.IEQString("abv")
		},
		e: `
fn main() void
  aeq_s "abv"
end`,
	},
	{
		i: func(b *Builder) {
			b.EQInteger()
		},
		e: `
fn main() void
  eq_i
end`,
	},
	{
		i: func(b *Builder) {
			b.IEQInteger(345)
		},
		e: `
fn main() void
  aeq_i 345
end`,
	},
	{
		i: func(b *Builder) {
			b.EQBool()
		},
		e: `
fn main() void
  eq_b
end`,
	},
	{
		i: func(b *Builder) {
			b.IEQBool(true)
		},
		e: `
fn main() void
  aeq_b true
end`,
	},
	{
		i: func(b *Builder) {
			b.EQDouble()
		},
		e: `
fn main() void
  eq_d
end`,
	},
	{
		i: func(b *Builder) {
			b.IEQDouble(10.123)
		},
		e: `
fn main() void
  aeq_d 10.123000
end`,
	},
	{
		i: func(b *Builder) {
			b.Ret()
		},
		e: `
fn main() void
  ret
end`,
	},
	{
		i: func(b *Builder) {
			b.Call("foo")
		},
		e: `
fn main() void
  call foo
end`,
	},
	{
		i: func(b *Builder) {
			b.ResolveInt("foo")
		},
		e: `
fn main() void
  resolve_i "foo"
end`,
	},
	{
		i: func(b *Builder) {
			b.TResolveInt("foo")
		},
		e: `
fn main() void
  tresolve_i "foo"
end`,
	},
	{
		i: func(b *Builder) {
			b.ResolveBool("foo")
		},
		e: `
fn main() void
  resolve_b "foo"
end`,
	},
	{
		i: func(b *Builder) {
			b.TResolveBool("foo")
		},
		e: `
fn main() void
  tresolve_b "foo"
end`,
	},
	{
		i: func(b *Builder) {
			b.ResolveString("foo")
		},
		e: `
fn main() void
  resolve_s "foo"
end`,
	},
	{
		i: func(b *Builder) {
			b.TResolveString("foo")
		},
		e: `
fn main() void
  tresolve_s "foo"
end`,
	},
	{
		i: func(b *Builder) {
			b.ResolveDouble("foo")
		},
		e: `
fn main() void
  resolve_d "foo"
end`,
	},
	{
		i: func(b *Builder) {
			b.TResolveDouble("foo")
		},
		e: `
fn main() void
  tresolve_d "foo"
end`,
	},
	{
		i: func(b *Builder) {
			b.ResolveRecord("foo")
		},
		e: `
fn main() void
  resolve_m "foo"
end`,
	},
	{
		i: func(b *Builder) {
			b.TResolveRecord("foo")
		},
		e: `
fn main() void
  tresolve_m "foo"
end`,
	},
	{
		i: func(b *Builder) {
			b.IPushBool(true)
		},
		e: `
fn main() void
  apush_b true
end`,
	},
	{
		i: func(b *Builder) {
			b.IPushStr("ABV")
		},
		e: `
fn main() void
  apush_s "ABV"
end`,
	},
	{
		i: func(b *Builder) {
			b.IPushDouble(123.456)
		},
		e: `
fn main() void
  apush_d 123.456000
end`,
	},
	{
		i: func(b *Builder) {
			b.IPushInt(456)
		},
		e: `
fn main() void
  apush_i 456
end`,
	},
	{
		i: func(b *Builder) {
			b.Lookup()
		},
		e: `
fn main() void
  lookup
end`,
	},
	{
		i: func(b *Builder) {
			b.ALookup("abc")
		},
		e: `
fn main() void
  alookup "abc"
end`,
	},
	{
		i: func(b *Builder) {
			l := b.AllocateLabel()
			b.Nop()
			b.Jmp(l)
			b.Nop()
			b.Jnz(l)
			b.Nop()
			b.Jz(l)
			b.Nop()
			b.SetLabelPos(l)
			b.Nop()
			b.Jmp(l)
			b.Nop()
			b.Jnz(l)
			b.Nop()
			b.Jz(l)
			b.Nop()
		},
		e: `
fn main() void
  nop
  jmp L0
  nop
  jnz L0
  nop
  jz L0
  nop
L0:
  nop
  jmp L0
  nop
  jnz L0
  nop
  jz L0
  nop
end`,
	},
}

func Test(t *testing.T) {
	for _, test := range builderTests {
		t.Run("["+test.e+"]", func(tt *testing.T) {
			p := NewProgram()
			b := NewBuilder(p.strings)
			test.i(b)
			body := b.Build()
			e := p.AddFunction("main", []Type{}, Void, body)
			if e != nil {
				tt.Fatal(e)
			}
			txt := WriteText(p)

			if strings.TrimSpace(txt) != strings.TrimSpace(test.e) {
				tt.Logf("Expected:\n%s\n", test.e)
				tt.Logf("Actual:\n%s\n", txt)
				tt.Fatal()
			}

		})
	}
}
