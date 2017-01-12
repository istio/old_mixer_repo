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

package logger

import (
	"fmt"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/struct"
	"google.golang.org/genproto/googleapis/rpc/code"
	"istio.io/mixer/pkg/aspect"
	"istio.io/mixer/pkg/aspect/logger"
	"istio.io/mixer/pkg/aspectsupport"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/expr"

	aspectpb "istio.io/api/mixer/v1/config/aspect"
	dpb "istio.io/api/mixer/v1/config/descriptor"
)

type (
	manager struct{}

	executor struct {
		logName     string
		descriptors []dpb.LogEntryDescriptor // describe entries to gen
		inputs      map[string]string        // map from param to expr
		aspect      logger.Aspect
	}
)

// NewManager returns an aspect manager for the logger aspect.
func NewManager() aspectsupport.Manager {
	return &manager{}
}

func (m *manager) NewAspect(c *aspectsupport.CombinedConfig, a aspect.Adapter, env aspect.Env) (aspectsupport.AspectWrapper, error) {
	// Handle aspect config to get log name and log entry descriptors.
	aspectCfg := m.DefaultConfig()
	if c.Aspect.Params != nil {
		if err := structToProto(c.Aspect.Params, aspectCfg); err != nil {
			return nil, fmt.Errorf("could not parse aspect config: %v", err)
		}
	}

	logCfg := aspectCfg.(*aspectpb.LoggerConfig)
	logName := logCfg.LogName
	// TODO: look up actual descriptors by name and build an array

	// cast to logger.Adapter from aspect.Adapter
	logAdapter, ok := a.(logger.Adapter)
	if !ok {
		return nil, fmt.Errorf("adapter of incorrect type. Expected logger.Adapter got %#v %T", a, a)
	}

	// Handle adapter config
	cpb := logAdapter.DefaultConfig()
	if c.Adapter.Params != nil {
		if err := structToProto(c.Adapter.Params, cpb); err != nil {
			return nil, fmt.Errorf("could not parse adapter config: %v", err)
		}
	}

	aspectImpl, err := logAdapter.NewAspect(env, cpb)
	if err != nil {
		return nil, err
	}

	var inputs map[string]string
	if c.Aspect != nil && c.Aspect.Inputs != nil {
		inputs = c.Aspect.Inputs
	}

	return &executor{logName, []dpb.LogEntryDescriptor{}, inputs, aspectImpl}, nil
}

func (*manager) Kind() string { return "istio/logger" }
func (*manager) DefaultConfig() proto.Message {
	return &aspectpb.LoggerConfig{LogName: "istio_log"}
}
func (*manager) ValidateConfig(implConfig proto.Message) (ce *aspect.ConfigErrors) { return nil }

func (e *executor) Execute(attrs attribute.Bag, mapper expr.Evaluator) (*aspectsupport.Output, error) {
	var entries []logger.Entry
	labels := make(map[string]interface{})
	for attr, expr := range e.inputs {
		if val, err := mapper.Eval(expr, attrs); err == nil {
			labels[attr] = val
		}
	}

	for _, d := range e.descriptors {
		entry := logger.Entry{LogName: e.logName, Labels: make(map[string]interface{})}
		for _, a := range d.Attributes {
			if val, ok := labels[a]; ok {
				entry.Labels[a] = val
				continue
			}
			if val, found := attribute.Value(attrs, a); found {
				entry.Labels[a] = val
			}

			// TODO: do we want to error for attributes that cannot
			// be found?
		}
		if d.PayloadAttribute != "" {
			payload, found := attrs.String(d.PayloadAttribute)
			if !found {
				// TODO: should this be an error that is returned
				// or should we skip this log entry or
				// should we simply not pass a payload in the
				// entry?
				continue
			}
			entry.Payload = payload
		}
		entries = append(entries, entry)
	}

	if len(entries) > 0 {
		if err := e.aspect.Log(entries); err != nil {
			return nil, err
		}
	}
	return &aspectsupport.Output{Code: code.Code_OK}, nil
}

func structToProto(in *structpb.Struct, out proto.Message) error {
	mm := &jsonpb.Marshaler{}
	str, err := mm.MarshalToString(in)
	if err != nil {
		return fmt.Errorf("failed to marshal to string: %v", err)
	}
	return jsonpb.UnmarshalString(str, out)
}
