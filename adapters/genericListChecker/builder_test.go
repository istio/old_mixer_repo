// Copyright 2016 Google Inc.
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

package genericListChecker

import (
	"testing"

	"istio.io/mixer/adapters"
	"istio.io/mixer/adapters/testutil"
)

func TestBuilderInvariants(t *testing.T) {
	b := NewBuilder()
	testutil.TestBuilderInvariants(b, t)
}

func runTests(t *testing.T, bc BuilderConfig, ac AdapterConfig, goodValues []string, badValues []string) {
	b := NewBuilder()
	b.Configure(&bc)

	aa, err := b.NewAdapter(&ac)
	if err != nil {
		t.Error("unable to create adapter " + err.Error())
	}
	a := aa.(adapters.ListChecker)

	for _, value := range goodValues {
		ok, err := a.CheckList(value)
		if !ok || err != nil {
			t.Error("Expecting check to pass for value " + value)
		}
	}

	for _, value := range badValues {
		ok, err := a.CheckList(value)
		if ok || err != nil {
			t.Error("Expecting check to fail for value " + value)
		}
	}
}

func TestWhitelistWithOverrides(t *testing.T) {
	builderEntries := []string{"Four", "Five"}
	adapterEntries := []string{"One", "Two", "Three"}
	goodValues := []string{"One", "Two", "Three"}
	badValues := []string{"O", "OneOne", "ne", "ree"}
	runTests(t, BuilderConfig{ListEntries: builderEntries, WhitelistMode: true}, AdapterConfig{ListEntries: adapterEntries}, goodValues, badValues)
}

func TestBlacklistWithOverrides(t *testing.T) {
	builderEntries := []string{"Four", "Five"}
	adapterEntries := []string{"One", "Two", "Three"}
	goodValues := []string{"O", "OneOne", "ne", "ree"}
	badValues := []string{"One", "Two", "Three"}
	runTests(t, BuilderConfig{ListEntries: builderEntries, WhitelistMode: false}, AdapterConfig{ListEntries: adapterEntries}, goodValues, badValues)
}

func TestWhitelistNoOverride(t *testing.T) {
	builderEntries := []string{"One", "Two", "Three"}
	goodValues := []string{"One", "Two", "Three"}
	badValues := []string{"O", "OneOne", "ne", "ree"}
	runTests(t, BuilderConfig{ListEntries: builderEntries, WhitelistMode: true}, AdapterConfig{}, goodValues, badValues)
}

func TestEmptyList(t *testing.T) {
	goodValues := []string{}
	badValues := []string{"Lasagna"}
	runTests(t, BuilderConfig{WhitelistMode: true}, AdapterConfig{}, goodValues, badValues)
}
