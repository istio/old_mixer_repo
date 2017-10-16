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
	"crypto/sha256"
	"encoding/base64"
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
		configError   bool
	}

	handler struct {
		policy      []string
		checkMethod string

		compiler *ast.Compiler

		env         adapter.Env
		configError bool
		failClose   bool
	}
)

///////////////// Configuration Methods ///////////////

func (b *builder) SetAuthzTypes(types map[string]*authz.Type) {
	b.types = types
}

func (b *builder) SetAdapterConfig(cfg adapter.Config) {
	b.adapterConfig = cfg.(*config.Params)
}

func (b *builder) getShaHash(policy string) string {
	hasher := sha256.New()
	_, err := hasher.Write([]byte(policy))
	hashKey := ""
	if err == nil {
		hashKey = base64.URLEncoding.EncodeToString(hasher.Sum(nil))
	}
	return hashKey
}

// To support fail close, Validate will not append errors to ce
// It set configError to true, then HandleAuthz return permission denied respose
func (b *builder) Validate() (ce *adapter.ConfigErrors) {
	b.configError = false
	if len(b.adapterConfig.CheckMethod) == 0 {
		b.configError = true
		return
	}

	modules := map[string]*ast.Module{}
	for _, policy := range b.adapterConfig.Policy {
		filename := b.getShaHash(policy)
		if modules[filename] != nil {
			b.configError = true
			return
		}
		parsed, err := ast.ParseModule(filename, policy)
		if err != nil {
			b.configError = true
			return
		}
		modules[filename] = parsed
	}

	if !b.configError {
		compiler := ast.NewCompiler()
		compiler.Compile(modules)
		if compiler.Failed() {
			b.configError = true
		}
	}

	return
}

func (b *builder) Build(context context.Context, env adapter.Env) (adapter.Handler, error) {
	var compiler *ast.Compiler

	if !b.configError {
		modules := map[string]*ast.Module{}
		for _, policy := range b.adapterConfig.Policy {
			filename := b.getShaHash(policy)
			parsed, _ := ast.ParseModule(filename, policy)
			modules[filename] = parsed
		}

		compiler = ast.NewCompiler()
		compiler.Compile(modules)
	}

	return &handler{
		policy:      b.adapterConfig.Policy,
		checkMethod: b.adapterConfig.CheckMethod,
		env:         env,
		compiler:    compiler,
		configError: b.configError,
		failClose:   b.adapterConfig.FailClose,
	}, nil
}

////////////////// Runtime Methods //////////////////////////

func (h *handler) HandleAuthz(context context.Context, instance *authz.Instance) (adapter.CheckResult, error) {
	// Handle configuration error
	if h.configError {
		retStatus := status.OK

		if h.failClose {
			retStatus = status.WithPermissionDenied("opa: request was rejected")
		}

		return adapter.CheckResult{
			Status: retStatus,
		}, nil
	}

	rego := rego.New(
		rego.Compiler(h.compiler),
		rego.Query(h.checkMethod),
		rego.Input(map[string]interface{}{
			"action":  instance.Action,
			"subject": instance.Subject,
		}),
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
