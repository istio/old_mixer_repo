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
	istiopb "istio.io/api/istio/config/v1"
)

type (

	// AdapterConfig provides common configuration params
	// Impl config protobufs are requested to support these
	AdapterConfig struct {
		Debug bool
	}
	// Adapter represents a factory to create an adapterImpl that provides a specific aspect
	// This interface is extended by specific aspects
	Adapter interface {
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
		// Name returns the official name of the aspect
		Name() string
		// Close - ability to close aspect
		io.Closer
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

	// AdapterCfgReg registry maps from adapter "kind" -->
	AdapterCfgReg interface {
		// ByKind given a kind string returns list of configured adapterCfgs
		ByKind(kind string) []*AdapterCfg
		// ByImpl given an impl string returns the configured adapterCfg
		ByImpl(impl string) *AdapterCfg
	}

	// Manager manages a specific type of aspect and presets a uniform interface
	// to the rest of system
	Manager interface {
		Execute(aspectCfg *Cfg, adapterCfg *AdapterCfg)
	}
)
