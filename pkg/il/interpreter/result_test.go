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

package interpreter

import (
	"testing"

	"istio.io/mixer/pkg/il"
)

func Test_BoolPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("The code did not panic")
		}
	}()

	r := Result{
		t: il.String,
	}

	_ = r.Bool()
}

func Test_IntegerPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("The code did not panic")
		}
	}()

	r := Result{
		t: il.String,
	}

	_ = r.Integer()
}

func Test_DoublePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("The code did not panic")
		}
	}()

	r := Result{
		t: il.String,
	}

	_ = r.Double()
}

func Test_DurationPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("The code did not panic")
		}
	}()

	r := Result{
		t: il.String,
	}

	_ = r.Duration()
}

func Test_InterfacePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("The code did not panic")
		}
	}()

	r := Result{
		t: il.Unknown,
	}

	_ = r.Interface()
}

func Test_String_WithNonString(t *testing.T) {
	r := Result{
		t: il.Integer,
	}

	if r.String() != "0" {
		t.Fatalf("Unexpected string serialization: %v", r.String())
	}
}

func Test_Interface_EmptyStringReturnsNull(t *testing.T) {
	r := Result{
		t:  il.String,
		vs: "",
	}

	if r.Interface() != nil {
		t.Fatalf("Expected empty string to be converted to nil.")
	}
}

func Test_Interface(t *testing.T) {
	r := Result{
		t:  il.Interface,
		vi: "foobarbaz",
	}

	if r.Interface() != "foobarbaz" {
		t.Fatalf("Expected interface value not found.")
	}
}

func Test_Type(t *testing.T) {
	r := Result{
		t: il.Integer,
	}

	if r.Type() != il.Integer {
		t.Fatalf("Unexpected type: %v", r.Type())
	}
}
