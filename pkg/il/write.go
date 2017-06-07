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
	"bytes"
	"fmt"
	"math"
	"sort"
)

// WriteText writes the program in the textual assembly form and returns it as string.
func WriteText(p Program) string {
	var b bytes.Buffer

	names := p.Functions.Names()
	sort.Strings(names)

	for _, name := range names {
		f := p.Functions.Get(name)
		WriteFn(&b, p.ByteCode(), f, p.Strings(), 0)
		b.WriteString("\n")
	}

	return b.String()
}

// WriteFn writes the given function to the given byte-buffer, as string.
// If the tag parameter is non-zero, a comment-based tag is also printed next to the op-code at
// the index indicated by tag.
func WriteFn(b *bytes.Buffer, code []uint32, f *Function, strings *StringsTable, tag uint32) {
	var labels = make(map[uint32]int)
	id := 0
	for i := f.Address; i < f.Address+f.Length; i++ {
		op := Opcode(code[i])
		for _, arg := range op.Args() {
			i++
			adr := code[i]
			if arg == OpcodeArgAddress {
				_, e := labels[adr]
				if !e {
					labels[adr] = id
					id++
				}
			}
		}
	}

	b.WriteString("fn ")
	b.WriteString(f.Name)
	b.WriteString("(")
	for i, p := range f.Parameters {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(fmt.Sprintf("%v", p))
	}
	b.WriteString(") ")
	b.WriteString(f.ReturnType.String())
	b.WriteString("\n")
	for i := f.Address; i < f.Address+f.Length; i++ {
		labelID, exists := labels[i]
		if exists {
			b.WriteString(fmt.Sprintf("L%d:\n", labelID))
		}

		opIndex := i
		op := Opcode(code[i])
		b.WriteString("  ")
		b.WriteString(op.Keyword())
		for _, arg := range op.Args() {
			b.WriteString(" ")
			i++
			val := code[i]
			switch arg {
			case OpcodeArgString:
				str := strings.GetString(val)
				str = escape(str)
				b.WriteString("\"")
				b.WriteString(str)
				b.WriteString("\"")

			case OpcodeArgAddress:
				b.WriteString(fmt.Sprintf("L%d", labels[val]))

			case OpcodeArgFunction:
				str := strings.GetString(val)
				b.WriteString(str)

			case OpcodeArgRegister:
				b.WriteString(fmt.Sprintf("r%d", val))

			case OpcodeArgInt:
				i++
				val2 := code[i]
				b.WriteString(fmt.Sprintf("%d", int64(val)+(int64(val2)<<32)))

			case OpcodeArgDouble:
				i++
				val2 := code[i]
				fl := math.Float64frombits(uint64(val) + (uint64(val2) << 32))
				b.WriteString(fmt.Sprintf("%f", fl))

			case OpcodeArgBool:
				if val == 0 {
					b.WriteString("false")
				} else {
					b.WriteString("true")
				}

			default:
				panic(fmt.Errorf("unknown arg type: %v", arg))
			}
		}
		if tag != 0 && tag == opIndex {
			b.WriteString("  // <<<<")
		}
		b.WriteString("\n")
	}
	b.WriteString("end\n")
}
