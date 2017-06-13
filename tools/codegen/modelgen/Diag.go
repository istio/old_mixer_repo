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

package modelgen

import (
	"bytes"
	"fmt"
	"strings"
)

const (
	ERROR_DIAG DiagKind = iota
	WARNING_DIAG
)
const UNKNOWN_LINE = ""
const UNKNOWN_FILE = ""

type (
	DiagKind uint8

	// Diag represents diagnostic information.
	Diag struct {
		kind     DiagKind
		Location location
		Message  string
	}
	// location represents the location of the Diag
	location struct {
		File string
		// TODO: Currently Line is always set as UNKNOWN_LINE. Consider using proto's
		// SourceCodeInfo to exactly point to the line number.
		Line string
	}
)

func stringifyDiags(diags []Diag) string {
	var result bytes.Buffer
	for _, diag := range diags {
		var kind string
		if diag.kind == ERROR_DIAG {
			kind = "Error"
		} else {
			kind = "Warning"
		}

		var msg string
		msg = strings.TrimSpace(diag.Message)
		if !strings.HasSuffix(msg, ".") {
			msg = msg + "."
		}

		if diag.Location.Line != "" {
			result.WriteString(fmt.Sprintf("%s: %s:%s: %s\n", kind, diag.Location.File, diag.Location.Line, msg))
		} else if diag.Location.File != "" {
			result.WriteString(fmt.Sprintf("%s: %s: %s\n", kind, diag.Location.File, msg))
		} else {
			result.WriteString(fmt.Sprintf("%s: %s\n", kind, msg))
		}
	}
	return result.String()
}

func (m *Model) addError(file string, line string, format string, a ...interface{}) {
	m.addDiag(ERROR_DIAG, file, line, format, a)
}

func (m *Model) addWarning(file string, line string, format string, a ...interface{}) {
	m.addDiag(WARNING_DIAG, file, line, format, a)
}

func (m *Model) addDiag(kind DiagKind, file string, line string, format string, a []interface{}) {
	m.Diags = append(m.Diags, createDiag(kind, file, line, format, a))
}

func createDiag(kind DiagKind, file string, line string, format string, a []interface{}) Diag {
	if len(a) == 0 {
		return Diag{kind: kind, Location: location{File: file, Line: line}, Message: format}
	} else {
		return Diag{kind: kind, Location: location{File: file, Line: line}, Message: fmt.Sprintf(format, a...)}
	}
}
