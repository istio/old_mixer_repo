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
	"strconv"
	"strings"
)

// ReadText parses the given il assembly text and converts it into a Program.
func ReadText(text string) (Program, error) {
	p := NewProgram()
	err := MergeText(text, p)
	if err != nil {
		return Program{}, err
	}
	return p, nil
}

// MergeText parses the given il assembly text and merges its contents into the Program.
func MergeText(text string, program Program) error {
	return parse(text, program)
}

// parser contains the parsing state of il assemly text.
type parser struct {
	s     *scanner
	error error

	program Program
}

// parse the given text as il assembly and merge its contents into the given program.
func parse(text string, program Program) error {
	p := &parser{
		s:       newScanner(text),
		error:   nil,
		program: program,
	}

	for !p.s.end() {
		if !p.s.next() {
			break
		}
		if p.s.token == tkError {
			p.fail("Parse error.")
			break
		}
		if !p.parseFunctionDef() {
			break
		}
	}

	if p.failed() {
		return p.error
	}

	return nil
}

func (p *parser) skipNewlines() {
	for p.s.token == tkNewLine && p.s.next() {
	}
}

func (p *parser) parseFunctionDef() bool {
	p.skipNewlines()

	if p.s.end() {
		return false
	}

	var f bool
	var i string
	if i, f = p.identifierOrFail(); !f {
		return false
	}
	if i != "fn" {
		p.fail("Expected 'fn'.")
		return false
	}

	if !p.nextToken(tkIdentifier) {
		return false
	}

	name, _ := p.s.asIdentifier()

	if !p.nextToken(tkOpenParen) {
		return false
	}

	var pTypes = []Type{}
	for {
		if !p.nextOrFail() {
			return false
		}
		if p.s.token != tkIdentifier {
			break
		}

		n, _ := p.s.asIdentifier()
		var pType Type
		if pType, f = GetType(n); !f {
			p.fail("Unrecognized parameter type: '%s'", n)
			return false
		}

		pTypes = append(pTypes, pType)
	}

	if !p.currentToken(tkCloseParen) {
		return false
	}

	if !p.nextToken(tkIdentifier) {
		return false
	}

	i, _ = p.s.asIdentifier()
	var rType Type
	if rType, f = GetType(i); !f {
		p.fail("Unrecognized return type: '%s'", i)
		return false
	}

	if !p.nextToken(tkNewLine) {
		return false
	}

	var body []uint32
	if body, f = p.parseFunctionBody(); !f {
		return false
	}

	err := p.program.AddFunction(name, pTypes, rType, body)
	if err != nil {
		p.failErr(err)
		return false
	}
	return true
}

func (p *parser) parseFunctionBody() ([]uint32, bool) {
	labels := make(map[string]uint32)
	labelRefLocs := make(map[int]location)
	fixups := make(map[int]string)

	body := make([]uint32, 0)

Loop:
	for {
		p.skipNewlines()

		var i string
		switch p.s.token {
		case tkLabel:
			l, _ := p.s.asLabel()
			labels[l] = uint32(len(body))
			if !p.nextOrFail() {
				goto FAILED
			}
			continue Loop

		case tkIdentifier:
			i, _ = p.s.asIdentifier()
			if i == "end" {
				break Loop
			}

		default:
			p.fail("unexpected input encountered: '%s'", p.s.rawText())
			break Loop
		}

		op, f := GetOpcode(i)
		if !f {
			p.fail("unrecognized opcode: '%s'", i)
			break Loop
		}
		body = append(body, uint32(op))

		for _, arg := range op.Args() {

			if !p.nextOrFail() {
				goto FAILED
			}

			switch arg {
			case OpcodeArgString:
				if i, f = p.s.asStringLiteral(); !f {
					p.fail("unexpected input: '%s'", p.s.rawText())
					goto FAILED
				}
				id := p.program.Strings().GetID(i)
				body = append(body, id)

			case OpcodeArgFunction:

				if i, f = p.s.asIdentifier(); !f {
					p.fail("unexpected input: '%s'", p.s.rawText())
					goto FAILED
				}
				id := p.program.Strings().GetID(i)
				body = append(body, id)

			case OpcodeArgInt:
				var i64 int64
				if i64, f = p.s.asIntegerLiteral(); !f {
					p.fail("unexpected input: '%s'", p.s.rawText())
					goto FAILED
				}
				t1, t2 := integerToOpcode(i64)
				body = append(body, t1)
				body = append(body, t2)

			case OpcodeArgDouble:
				var i1, i2 uint32
				var f64 float64
				var i64 int64
				if f64, f = p.s.asFloatLiteral(); f {
					i1, i2 = doubleToOpcode(f64)
				} else if i64, f = p.s.asIntegerLiteral(); f {
					i1, i2 = doubleToOpcode(float64(i64))
				} else {
					p.fail("unexpected input: '%s'", p.s.rawText())
					goto FAILED
				}
				body = append(body, i1)
				body = append(body, i2)

			case OpcodeArgBool:
				if i, f = p.s.asIdentifier(); !f {
					p.fail("unexpected input: '%s'", p.s.rawText())
					goto FAILED
				}
				switch i {
				case "true":
					body = append(body, uint32(1))
				case "false":
					body = append(body, uint32(0))
				default:
					p.fail("unexpected input: '%s'", p.s.rawText())
					goto FAILED
				}

			case OpcodeArgAddress:
				if i, f = p.s.asIdentifier(); !f {
					p.fail("unexpected input: '%s'", p.s.rawText())
					goto FAILED
				}
				fixups[len(body)] = i
				labelRefLocs[len(body)] = p.s.lBegin
				body = append(body, 0)

			case OpcodeArgRegister:
				if i, f = p.s.asIdentifier(); !f {
					p.fail("unexpected input: '%s'", p.s.rawText())
					goto FAILED
				}
				if !strings.HasPrefix(i, "r") {
					p.fail("Invalid register name: '%s'", i)
					goto FAILED
				}
				id, err := strconv.Atoi(i[1:])
				if err != nil {
					p.fail("Invalid register name: '%s'", i)
					goto FAILED
				}
				body = append(body, uint32(id))

			default:
				p.fail("Unknown op code arg type.")
				goto FAILED
			}
		}

		if !p.nextToken(tkNewLine) {
			goto FAILED
		}
	}

	if p.failed() {
		goto FAILED
	}

	for index, label := range fixups {
		ival, exists := labels[label]
		if !exists {
			p.failLoc(labelRefLocs[index], "Label not found: %s", label)
			return body, false
		}
		body[index] = ival
	}

	return body, true

FAILED:
	return body, false
}

func (p *parser) fail(format string, args ...interface{}) {
	p.failLoc(p.s.lBegin, format, args...)
}

func (p *parser) failLoc(l location, format string, args ...interface{}) {
	p.failErr(fmt.Errorf("%s @%v", fmt.Sprintf(format, args...), l))
}

func (p *parser) failErr(err error) {
	p.error = err
}

func (p *parser) failed() bool {
	return p.error != nil
}

func (p *parser) nextOrFail() bool {
	if !p.s.next() || p.s.token == tkNone {
		p.fail("unexpected end of file.")
		return false
	}
	if p.s.token == tkError {
		p.fail("Parse error.")
		return false
	}
	return true
}

func (p *parser) nextToken(t token) bool {
	if !p.nextOrFail() {
		return false
	}
	return p.currentToken(t)
}

func (p *parser) currentToken(t token) bool {
	if p.s.end() {
		p.fail("unexpected end of file encountered")
		return false
	}
	if p.s.token != t {
		p.fail("unexpected input: '%v'", p.s.rawText())
		return false
	}
	return true
}

func (p *parser) identifierOrFail() (string, bool) {
	if p.s.token != tkIdentifier {
		p.fail("unexpected input: '%v'", p.s.rawText())
		return "", false
	}

	return p.s.asIdentifier()
}
