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
	"math"
	"strings"
	"testing"
)

func checkWrites(t *testing.T, p Program, expected string) {
	actual := WriteText(p)

	if strings.TrimSpace(actual) != strings.TrimSpace(expected) {
		t.Logf("Expected:\n%s\n\n", expected)
		t.Logf("Actual:\n%s\n\n", actual)
		t.Fail()
		return
	}
}

func TestWriteEmptyProgram(t *testing.T) {
	p := NewProgram()

	checkWrites(t, p, ``)
}

func TestWriteEmptyFunction(t *testing.T) {
	p := NewProgram()
	err := p.AddFunction("main", []Type{}, Bool, []uint32{})
	if err != nil {
		t.Error(err)
		return
	}
	checkWrites(t, p, `
fn main() bool
end
	`)
}

func TestWriteEmptyFunctionWithParameters(t *testing.T) {
	p := NewProgram()
	err := p.AddFunction("main", []Type{StringMap}, Bool, []uint32{})
	if err != nil {
		t.Error(err)
		return
	}
	checkWrites(t, p, `
fn main(stringmap) bool
end
	`)
}

func TestWriteTwoFunctionsWithParameters(t *testing.T) {
	p := NewProgram()
	err := p.AddFunction("foo", []Type{Bool}, Bool, []uint32{})
	if err != nil {
		t.Error(err)
		return
	}
	err = p.AddFunction("bar", []Type{Integer, String}, Bool, []uint32{})
	if err != nil {
		t.Error(err)
		return
	}
	checkWrites(t, p, `
fn bar(integer string) bool
end

fn foo(bool) bool
end
	`)
}

func TestWriteBasicFunction(t *testing.T) {
	p := NewProgram()
	err := p.AddFunction("main", []Type{}, Bool, []uint32{
		uint32(Ret),
	})
	if err != nil {
		t.Error(err)
		return
	}

	checkWrites(t, p, `
fn main() bool
  ret
end
	`)
}

func TestWriteStringArg(t *testing.T) {
	p := NewProgram()
	id := p.Strings().GetID("foo")
	err := p.AddFunction("main", []Type{}, Bool, []uint32{
		uint32(ResolveS),
		uint32(id),
	})
	if err != nil {
		t.Error(err)
		return
	}
	checkWrites(t, p, `
fn main() bool
  resolve_s "foo"
end
	`)
}

func TestWriteLabelArg(t *testing.T) {
	p := NewProgram()
	err := p.AddFunction("main", []Type{}, Bool, []uint32{
		uint32(Jnz),
		uint32(0),
	})
	if err != nil {
		t.Error(err)
		return
	}
	checkWrites(t, p, `
fn main() bool
L0:
  jnz L0
end
	`)
}

func TestWriteLabelArg2(t *testing.T) {
	p := NewProgram()
	err := p.AddFunction("main", []Type{}, Bool, []uint32{
		uint32(Halt),
		uint32(Jnz),
		uint32(1),
	})
	if err != nil {
		t.Error(err)
		return
	}
	checkWrites(t, p, `
fn main() bool
  halt
L0:
  jnz L0
end
	`)
}

func TestWriteFunctionArg(t *testing.T) {
	p := NewProgram()
	err := p.AddFunction("callee", []Type{}, String, []uint32{
		uint32(Ret),
	})
	if err != nil {
		t.Error(err)
		return
	}
	err = p.AddFunction("caller", []Type{}, Bool, []uint32{
		uint32(Call),
		p.Strings().GetID("callee"),
		uint32(Ret),
	})
	if err != nil {
		t.Error(err)
		return
	}
	checkWrites(t, p, `
fn callee() string
  ret
end

fn caller() bool
  call callee
  ret
end
	`)
}

func TestWriteRegisterArg(t *testing.T) {
	p := NewProgram()
	err := p.AddFunction("main", []Type{}, Void, []uint32{
		uint32(RLoadB),
		uint32(1),
	})
	if err != nil {
		t.Error(err)
		return
	}
	checkWrites(t, p, `
fn main() void
  rload_b r1
end
	`)
}

func TestWriteIntArg(t *testing.T) {
	p := NewProgram()
	err := p.AddFunction("main", []Type{}, Bool, []uint32{
		uint32(APushI),
		uint32(1),
		uint32(0),
		uint32(APushI),
		uint32(2),
		uint32(0),
	})
	if err != nil {
		t.Error(err)
		return
	}
	checkWrites(t, p, `
fn main() bool
  apush_i 1
  apush_i 2
end
	`)
}

func TestWriteFloatArg(t *testing.T) {
	fl := float64(123.456)
	ui := math.Float64bits(fl)
	p := NewProgram()
	err := p.AddFunction("main", []Type{}, Bool, []uint32{
		uint32(APushD),
		uint32(ui & 0xFFFFFFFF),
		uint32(ui >> 32),
	})
	if err != nil {
		t.Error(err)
		return
	}
	checkWrites(t, p, `
fn main() bool
  apush_d 123.456000
end
	`)
}

func TestWriteBoolArg(t *testing.T) {
	p := NewProgram()
	err := p.AddFunction("main", []Type{}, Integer, []uint32{
		uint32(APushB),
		uint32(1),
		uint32(APushB),
		uint32(0),
	})
	if err != nil {
		t.Error(err)
		return
	}
	checkWrites(t, p, `
fn main() integer
  apush_b true
  apush_b false
end
	`)
}
