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

package runtime

type protocol int

const (
	protocolHTTP protocol = iota << 1
	protocolTCP
)

type method int

const (
	methodCheck method = iota << 1
	methodReport
	methodPreprocess
)

// RuleType codifies types of rules.
// rules apply to certain protocols or methods.
type RuleType struct {
	protocol protocol
	method   method
}

// String return string presentation of const.
func (r RuleType) String() string {
	p := ""
	if r.protocol&protocolHTTP != 0 {
		p += "HTTP "
	}
	if r.protocol&protocolTCP != 0 {
		p += "TCP "
	}
	m := ""
	if r.method&methodCheck != 0 {
		m += "Check "
	}
	if r.method&methodReport != 0 {
		m += "Report "
	}
	if r.method&methodPreprocess != 0 {
		m += "Preprocess"
	}
	return "RuleType:{" + p + "/" + m + "}"
}

// IsTCP returns true if rule is ok for TCP
func (r RuleType) IsTCP() bool {
	return r.protocol&protocolTCP != 0
}

// defaultRuletype defines the rule type if nothing is specified.
func defaultRuletype() RuleType {
	return RuleType{
		protocol: protocolHTTP,
		method:   methodCheck | methodReport | methodPreprocess,
	}
}
