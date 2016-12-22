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

package aspect

import (
	"io"

	"github.com/golang/protobuf/proto"
	"google.golang.org/genproto/googleapis/rpc/code"
	istiopb "istio.io/api/istio/config/v1"

	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/expr"
)

type (
	// Adapter represents a factory to create an adapterImpl that provides a specific aspect
	// This interface is extended by specific aspects
	Adapter interface {
		// Close - ability to close Adapter
		io.Closer
		// Name returns the official name of this adapter. ex. "istio.io/statsd".
		Name() string
		// Description returns a user-friendly description of this adapter.
		Description() string
		// DefaultConfig returns a default configuration struct for this
		// adapter Implmentation. This will be used by the configuration system to establish
		// the shape of the block of configuration state passed to the NewAspect method.
		DefaultConfig() proto.Message
		// ValidateConfig determines whether the given configuration meets all correctness requirements.
		ValidateConfig(config proto.Message) error
	}

	// Aspect -- User visible cross cutting concern
	Aspect interface {
		// Close - ability to close aspect
		io.Closer
		// Name returns the official name of the aspect
		Name() string
	}

	// Cfg is the internal struct used for the Aspect proto
	// at present it only has an additional field that stores the converted
	// and typed proto
	Cfg struct {
		istiopb.Aspect
		// TypedParams points to the proto after google_protobuf.Struct is converted
		TypedParams proto.Message
	}

	// AdapterCfg is the internal struct used for the Adapter proto
	// at present it only has an additional field that stores the converted
	// and typed Args
	AdapterCfg struct {
		istiopb.Adapter
		// TypedParams points to the proto after google_protobuf.Struct is converted
		TypedArgs proto.Message
	}
	// CombinedConfig combines all configuration related to an aspect
	CombinedConfig struct {
		Aspect  *Cfg
		Adapter *AdapterCfg
	}

	// ImplConfig provides common configuration params
	// Impl config protobufs are required to support these
	ImplConfig struct {
		Debug bool
		// All adapters will be given their impl specific proto
		Impl proto.Message
	}
	// AdapterCfgReg registry maps from adapter "kind" / impl --> adapterCfg
	AdapterCfgReg interface {
		// ByKind given a kind string returns list of configured adapterCfgs
		ByKind(kind string) []*AdapterCfg
		// ByImpl given an impl string returns the configured adapterCfg
		ByImpl(impl string) *AdapterCfg
	}

	// Output from the Aspect Manager
	Output struct {
		// status code
		Code code.Code
		//TODO attribute mutator
		//If any attributes should change in the context for the next call
		//context remains immutable during the call
	}
	// Manager manages a specific aspect and presets a uniform interface
	// to the rest of system
	Manager interface {
		// Execute dispatch to the given aspect using aspect and adapter configs
		// A cached instance of Aspect is provided that was previsouly obtained by
		// calling NewAspect
		Execute(cfg *CombinedConfig, asp Aspect, ctx attribute.Context, mapper expr.Evaluator) (*Output, error)
		// NewAspect creates a new aspect instance given configuration
		NewAspect(cfg *CombinedConfig, adapter Adapter) (Aspect, error)
		// Kind return the kind of aspect
		Kind() string
	}
)
