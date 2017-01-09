// Copyright 2017 Google Inc.
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

package adaptertesting

import (
	"testing"

	"istio.io/mixer/pkg/aspect"
	"istio.io/mixer/pkg/aspect/denyChecker"
	"istio.io/mixer/pkg/aspect/listChecker"
	"istio.io/mixer/pkg/registry"
)

// RegisterFunc is the function that registers adapters into the supplied registry
type RegisterFunc func(registry.Registrar) error

type fakeRegistrar struct {
	registrations int
}

func (r *fakeRegistrar) RegisterCheckList(listChecker.Adapter) error {
	r.registrations++
	return nil
}

func (r *fakeRegistrar) RegisterDeny(denyChecker.Adapter) error {
	r.registrations++
	return nil
}

// TestAdapterInvariants ensures that adapters implement expected semantics.
func TestAdapterInvariants(a aspect.Adapter, r RegisterFunc, t *testing.T) {
	if a.Name() == "" {
		t.Error("All adapters need names")
	}

	if a.Description() == "" {
		t.Error("All adapters need descriptions")
	}

	c := a.DefaultConfig()
	if err := a.ValidateConfig(c); err != nil {
		t.Errorf("Default config is expected to validate correctly: %v", err)
	}

	if err := a.Close(); err != nil {
		t.Errorf("Should not fail Close with default config: %v", err)
	}

	fr := &fakeRegistrar{}
	if err := r(fr); err != nil {
		t.Errorf("Registration failed")
	}

	if fr.registrations < 1 {
		t.Errorf("Expecting at least one adapter to be registered, got %d", fr.registrations)
	}
}
