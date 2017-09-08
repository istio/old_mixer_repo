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

package authzOpa // import "istio.io/mixer/adapter/authzOpa"

// NOTE: This adapter will eventually be auto-generated so that it automatically supports all CHECK and QUOTA
//       templates known to Mixer. For now, it's manually curated.

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang/glog"
	rpc "github.com/googleapis/googleapis/google/rpc"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"

	"istio.io/mixer/adapter/authzOpa/config"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/template/authz"
)

type (
	builder struct {
		adapterConfig *config.Params
	}

	handler struct {
		policy      string
		checkMethod string

		context  context.Context
		compiler *ast.Compiler

		env adapter.Env
	}
)

///////////////// Configuration Methods ///////////////

func (b *builder) SetAuthzTypes(map[string]*authz.Type) {}

func (b *builder) SetAdapterConfig(cfg adapter.Config) {
	b.adapterConfig = cfg.(*config.Params)
}

func (b *builder) Validate() (ce *adapter.ConfigErrors) {
	if len(b.adapterConfig.CheckMethod) == 0 {
		ce = ce.Appendf(GetInfo().Name,
			"CheckMethod was not configured")
	}

	parsed, err := ast.ParseModule("", string(b.adapterConfig.Policy))
	glog.Infof("%v", parsed)
	glog.Infof("%v", err)
	if err != nil {
		ce = ce.Appendf(GetInfo().Name,
			"Failed to parse the OPA policy: %v", err)
	}

	compiler := ast.NewCompiler()
	compiler.Compile(map[string]*ast.Module{"": parsed})
	glog.Infof("%v", compiler.Failed())
	glog.Infof("%v", compiler.Errors)
	if compiler.Failed() {
		ce = ce.Appendf(GetInfo().Name,
			"Failed to compile the OPA policy: %v", compiler.Errors)
	}

	return nil
}

func (b *builder) Build(context context.Context, env adapter.Env) (adapter.Handler, error) {
	h := handler{
		policy:      b.adapterConfig.Policy,
		checkMethod: b.adapterConfig.CheckMethod,
		env:         env,
	}

	parsed, err := ast.ParseModule("", string(h.policy))
	if err != nil {
		return nil, errors.New(fmt.Sprintf(GetInfo().Name,
			"Failed to parse the OPA policy: %v", err))
	}

	compiler := ast.NewCompiler()
	compiler.Compile(map[string]*ast.Module{"": parsed})
	if compiler.Failed() {
		return nil, errors.New(fmt.Sprintf(GetInfo().Name,
			"Failed to compile the OPA policy: %v", compiler.Errors))
	}

	h.compiler = compiler
	h.context = context

	return h, nil
}

////////////////// Runtime Methods //////////////////////////

func (h handler) HandleAuthz(context context.Context, instance *authz.Instance) (adapter.CheckResult, error) {
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

	// Run evaluation.
	rs, err := rego.Eval(h.context)

	if err != nil {
		return adapter.CheckResult{
			Status: rpc.Status{Code: int32(rpc.PERMISSION_DENIED),
				Message: fmt.Sprintf("authzOpa: request was rejected: %v", err)},
		}, nil
	}

	if len(rs) != 1 {
		return adapter.CheckResult{
			Status: rpc.Status{Code: int32(rpc.PERMISSION_DENIED),
				Message: "authzOpa: request was rejected"},
		}, nil
	}

	result, ok := rs[0].Expressions[0].Value.(bool)
	if !ok {
		return adapter.CheckResult{
			Status: rpc.Status{Code: int32(rpc.PERMISSION_DENIED),
				Message: "authzOpa: request was rejected"},
		}, nil
	}

	if result == false {
		return adapter.CheckResult{
			Status: rpc.Status{Code: int32(rpc.PERMISSION_DENIED),
				Message: "authzOpa: request was rejected"},
		}, nil
	}

	return adapter.CheckResult{
		Status: rpc.Status{Code: int32(rpc.OK)},
	}, nil
}

func (h handler) Close() error {
	return nil
}

////////////////// Bootstrap //////////////////////////

// GetBuilderInfo returns the BuilderInfo associated with
// this adapter implementation.
func GetInfo() adapter.BuilderInfo {
	return adapter.BuilderInfo{
		Name:        "authzOpa",
		Impl:        "istio.io/mixer/adapter/authzOpa",
		Description: "Istio Authorization with Open Policy Agent engine",
		SupportedTemplates: []string{
			authz.TemplateName,
		},
		DefaultConfig: &config.Params{},
		NewBuilder:    func() adapter.HandlerBuilder { return &builder{} },
	}
}
