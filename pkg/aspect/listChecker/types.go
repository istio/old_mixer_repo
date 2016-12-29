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
	"github.com/golang/protobuf/proto"
)

type (
	// Config -- Adapter Author visible typed config for
	// creating new listChecker aspect
	// Every aspect defines it's own  aspect/XXXX.Config struct.
	// It may or may not contain the underlying
	// protobuf config specified by the user (for listChecker: listcheckerpb.Config)
	// Aspect manager handles as much of listcheckerpb.Config
	// as possible. Rest of the struct is packaged and sent down to NewAspect()
	// In case of listChecker; the listChecker.Manager fully handles
	// listcheckerpb.Config.
	// So we only pass along ImplConfig
	Config struct {
		ImplConfig proto.Message
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
		NewAspect(cfg *Config) (Aspect, error)
	}
)
