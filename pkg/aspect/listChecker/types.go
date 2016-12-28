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

package listChecker

import "istio.io/mixer/pkg/aspect"

type (
	// AdapterConfig -- Adapter Author visible typed config for
	// creating new listChecker aspect
	// This may or may not have listcheckerpb.Config
	// Aspect manager handles as much of this config
	// as it makes sense. Rest of the struct is packaged
	// and sent down to NewAspect()
	// It will always have ImplConfig if one is present
	AdapterConfig struct {
		aspect.ImplConfig
	}

	// Aspect listChecker checks given symbol against a list
	Aspect interface {
		aspect.Aspect
		// CheckList verifies whether the given symbol is on the list.
		CheckList(Symbol string) (bool, error)
	}

	// Adapter builds the ListChecker Aspect
	Adapter interface {
		aspect.Adapter
		// NewAspect returns a new ListChecker
		NewAspect(cfg *AdapterConfig) (Aspect, error)
	}
)
