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

import (
	"istio.io/mixer/pkg/aspect"

	listcheckerpb "istio.io/api/istio/config/v1/aspect/listChecker"
)

type (
	// AdapterConfig -- Adapter Author visible typed config for
	// creating new listChecker aspect
	AdapterConfig struct {
		Aspect *listcheckerpb.Config

		aspect.ImplConfig
	}

	// Arg -- Developer visible input passed into the perform fn
	// of the aspect
	Arg struct {
		CfgInput *listcheckerpb.Input
	}

	// Aspect listChecker checks given symbol against a list
	Aspect interface {
		aspect.Aspect
		// CheckList verifies whether the given symbol is on the list.
		CheckList(Arg *Arg) (bool, error)
	}

	// Adapter builds the ListChecker Aspect
	Adapter interface {
		aspect.Adapter
		// NewAspect returns a new ListChecker
		NewAspect(cfg *AdapterConfig) (Aspect, error)
	}
)
