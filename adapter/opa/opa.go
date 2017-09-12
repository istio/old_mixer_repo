// Copyright 2017 Istio Authors.
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

package opa // import "istio.io/mixer/adapter/opa"

// NOTE: This adapter will eventually be auto-generated so that it automatically supports all CHECK and QUOTA
//       templates known to Mixer. For now, it's manually curated.

import (
	"context"
	"fmt"

	rpc "github.com/googleapis/googleapis/google/rpc"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"

	"istio.io/mixer/adapter/opa/config"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/status"
	"istio.io/mixer/template/authz"
)

type (
	builder struct {
		types         map[string]*authz.Type
		adapterConfig *config.Params
	}

	handler struct {
		policy      string
		checkMethod string

		compiler *ast.Compiler

		env adapter.Env
	}
)

///////////////// Configuration Methods ///////////////

func (b *builder) SetAuthzTypes(types map[string]*authz.Type) {
	b.types = types
}

func (b *builder) SetAdapterConfig(cfg adapter.Config) {
	b.adapterConfig = cfg.(*config.Params)
}

func (b *builder) Validate() (ce *adapter.ConfigErrors) {
	name := GetInfo().Name

	dedup := map[string]bool{}
	for _, config := range b.types {
		for key := range config.Subject {
			if _, ok := dedup[key]; ok {
				ce = ce.Appendf(name, fmt.Sprintf("%s in subject is duplicated", key))
			} else {
				dedup[key] = true
			}
		}
		for key := range config.Resource {
			if _, ok := dedup[key]; ok {
				ce = ce.Appendf(name, fmt.Sprintf("%s in resource is duplicated", key))
			} else {
				dedup[key] = true
			}
		}
		for key := range config.Verb {
			if _, ok := dedup[key]; ok {
				ce = ce.Appendf(name, fmt.Sprintf("%s in verb is duplicated", key))
			} else {
				dedup[key] = true
			}
		}
	}

	if len(b.adapterConfig.CheckMethod) == 0 {
		ce = ce.Appendf(name, "CheckMethod was not configured")
	}

	parsed, err := ast.ParseModule("", b.adapterConfig.Policy)
	if err != nil {
		ce = ce.Appendf(name, "Failed to parse the OPA policy: %v", err)
		return
	}

	compiler := ast.NewCompiler()
	compiler.Compile(map[string]*ast.Module{"": parsed})
	if compiler.Failed() {
		ce = ce.Appendf(name, "Failed to compile the OPA policy: %v", compiler.Errors)
	}

	return
}

func (b *builder) Build(context context.Context, env adapter.Env) (adapter.Handler, error) {
	parsed, _ := ast.ParseModule("", b.adapterConfig.Policy)

	compiler := ast.NewCompiler()
	compiler.Compile(map[string]*ast.Module{"": parsed})

	return &handler{
		policy:      b.adapterConfig.Policy,
		checkMethod: b.adapterConfig.CheckMethod,
		env:         env,
		compiler:    compiler,
	}, nil
}

////////////////// Runtime Methods //////////////////////////

func (h *handler) HandleAuthz(context context.Context, instance *authz.Instance) (adapter.CheckResult, error) {
	variables := make(map[string]interface{})

	for k, v := range instance.Subject {
		variables[k] = v
	}
	for k, v := range instance.Resource {
		variables[k] = v
	}
	for k, v := range instance.Verb {
		variables[k] = v
	}

	rego := rego.New(
		rego.Compiler(h.compiler),
		rego.Query(h.checkMethod),
		rego.Input(variables),
	)

	rs, err := rego.Eval(context)
	if err != nil {
		return adapter.CheckResult{
			Status: status.WithPermissionDenied(fmt.Sprintf("opa: request was rejected: %v", err)),
		}, nil
	}

	if len(rs) != 1 {
		return adapter.CheckResult{
			Status: status.WithPermissionDenied("opa: request was rejected"),
		}, nil
	}

	result, ok := rs[0].Expressions[0].Value.(bool)
	if !ok || !result {
		return adapter.CheckResult{
			Status: status.WithPermissionDenied("opa: request was rejected"),
		}, nil
	}

	return adapter.CheckResult{
		Status: rpc.Status{Code: int32(rpc.OK)},
	}, nil
}

func (h *handler) Close() error {
	return nil
}

////////////////// Bootstrap //////////////////////////

// GetInfo returns the Info associated with this adapter implementation.
func GetInfo() adapter.Info {
	return adapter.Info{
		Name:        "opa",
		Impl:        "istio.io/mixer/adapter/opa",
		Description: "Istio Authorization with Open Policy Agent engine",
		SupportedTemplates: []string{
			authz.TemplateName,
		},
		DefaultConfig: &config.Params{},
		NewBuilder:    func() adapter.HandlerBuilder { return &builder{} },
	}
}
