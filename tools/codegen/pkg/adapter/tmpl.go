// Copyright 2016 Istio Authors
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

package adapter

var spyAdapterTemplate = `// Copyright 2017 Istio Authors
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

// THIS FILE IS AUTOMATICALLY GENERATED.

package {{.PkgName}}

import (
	"context"
	"github.com/gogo/protobuf/types"
	"istio.io/mixer/pkg/adapter"
	{{range .TemplateModels}}
		"{{.PackageImportPath}}"
	{{end}}
)

{{range .TemplateModels}}
  var _ {{.GoPackageName}}.HandlerBuilder = builder{}
  var _ {{.GoPackageName}}.Handler = handler{}
{{end}}

type (
	// Adapter is a fake Adapter. It is used for controlling the Adapter's behavior as well as
	// inspect the input values that adapter receives from Mixer
	Adapter struct {
		Behavior    AdapterBehavior
		BuilderData *builderData
		HandlerData *handlerData
	}

	// AdapterBehavior defines the behavior of the Adapter
	// nolint: aligncheck
	AdapterBehavior struct {
		Name    string
		Builder BuilderBehavior
		Handler HandlerBehavior
	}

	// nolint: aligncheck
	builder struct {
		behavior        BuilderBehavior
		handlerBehavior HandlerBehavior
		data            *builderData
		handlerData     *handlerData
	}

	handler struct {
		behavior HandlerBehavior
		data     *handlerData
	}

	// HandlerBehavior defines the behavior of the Handler
	// nolint: aligncheck
	HandlerBehavior struct {
        {{range .TemplateModels}}
		      Handle{{.InterfaceName}}Err   error
		      Handle{{.InterfaceName}}Panic bool
           {{if eq .VarietyName "TEMPLATE_VARIETY_CHECK"}}
              Handle{{.InterfaceName}}Result adapter.CheckResult
           {{end}}
           {{if eq .VarietyName "TEMPLATE_VARIETY_QUOTA"}}
			  Handle{{.InterfaceName}}Result adapter.QuotaResult
           {{end}}
        {{end}}

		CloseErr   error
		ClosePanic bool
	}

	// BuilderBehavior defines the behavior of the Builder
	// nolint: aligncheck
	BuilderBehavior struct {
        {{range .TemplateModels}}
              Set{{.InterfaceName}}TypesPanic bool
        {{end}}

		SetAdapterConfigPanic bool

		ValidateErr   *adapter.ConfigErrors
		ValidatePanic bool

		BuildErr   error
		BuildPanic bool
	}

	handlerData struct {
        {{range .TemplateModels}}
           {{if eq .VarietyName "TEMPLATE_VARIETY_REPORT"}}
              Handle{{.InterfaceName}}Instances []*{{.GoPackageName}}.Instance
           {{end}}
           {{if eq .VarietyName "TEMPLATE_VARIETY_CHECK"}}
              Handle{{.InterfaceName}}Instances *{{.GoPackageName}}.Instance
           {{end}}
           {{if eq .VarietyName "TEMPLATE_VARIETY_QUOTA"}}
			  Handle{{.InterfaceName}}Instances *{{.GoPackageName}}.Instance
			  Handle{{.InterfaceName}}QuotaArgs adapter.QuotaArgs
           {{end}}
           Handle{{.InterfaceName}}Count     int
        {{end}}

		CloseCount int
	}

	builderData struct {
        {{range .TemplateModels}}
           	Set{{.InterfaceName}}TypesCount int
			Set{{.InterfaceName}}TypesTypes map[string]*{{.GoPackageName}}.Type
        {{end}}

		SetAdapterConfigAdptCfg adapter.Config
		SetAdapterConfigCount   int

		ValidateCount int

		BuildCount int
		BuildCtx   context.Context
		BuildEnv   adapter.Env
	}
)

func (b builder) Build(ctx context.Context, env adapter.Env) (adapter.Handler, error) {
	b.data.BuildCount++
	if b.behavior.BuildPanic {
		panic("Build")
	}

	b.data.BuildCtx = ctx
	b.data.BuildEnv = env

	return handler{behavior: b.handlerBehavior, data: b.handlerData}, b.behavior.BuildErr
}

func (b builder) SetAdapterConfig(cfg adapter.Config) {
	b.data.SetAdapterConfigCount++
	b.data.SetAdapterConfigAdptCfg = cfg

	if b.behavior.SetAdapterConfigPanic {
		panic("SetAdapterConfig")
	}
}

func (b builder) Validate() *adapter.ConfigErrors {
	b.data.ValidateCount++
	if b.behavior.ValidatePanic {
		panic("Validate")
	}

	return b.behavior.ValidateErr
}

func (h handler) Close() error {
	h.data.CloseCount++
	if h.behavior.ClosePanic {
		panic("Close")
	}

	return h.behavior.CloseErr
}

// NewSpyAdapter returns a new instance of Adapter with the given behavior
func NewSpyAdapter(b AdapterBehavior) *Adapter {
	return &Adapter{Behavior: b, BuilderData: &builderData{}, HandlerData: &handlerData{}}
}

{{range .TemplateModels}}
func (b builder) Set{{.InterfaceName}}Types(typeParams map[string]*{{.GoPackageName}}.Type) {
	b.data.Set{{.InterfaceName}}TypesCount++
	b.data.Set{{.InterfaceName}}TypesTypes = typeParams

	if b.behavior.Set{{.InterfaceName}}TypesPanic {
		panic("Set{{.InterfaceName}}Types")
	}
}
{{if eq .VarietyName "TEMPLATE_VARIETY_REPORT"}}
func (h handler) Handle{{.InterfaceName}}(ctx context.Context, instances []*{{.GoPackageName}}.Instance) error {
	h.data.Handle{{.InterfaceName}}Count++
	if h.behavior.Handle{{.InterfaceName}}Panic {
		panic("Handle{{.InterfaceName}}")
	}

	h.data.Handle{{.InterfaceName}}Instances = instances
	return h.behavior.Handle{{.InterfaceName}}Err
}
{{end}}
{{if eq .VarietyName "TEMPLATE_VARIETY_CHECK"}}
func (h handler) Handle{{.InterfaceName}}(ctx context.Context, instance *{{.GoPackageName}}.Instance) (adapter.CheckResult, error) {
	h.data.Handle{{.InterfaceName}}Count++
	if h.behavior.Handle{{.InterfaceName}}Panic {
		panic("Handle{{.InterfaceName}}")
	}

	h.data.Handle{{.InterfaceName}}Instances = instance
	return h.behavior.Handle{{.InterfaceName}}Result, h.behavior.Handle{{.InterfaceName}}Err
}

{{end}}
{{if eq .VarietyName "TEMPLATE_VARIETY_QUOTA"}}
func (h handler) Handle{{.InterfaceName}}(ctx context.Context, instance *{{.GoPackageName}}.Instance, quotaArgs adapter.QuotaArgs) (adapter.QuotaResult, error) {
	h.data.Handle{{.InterfaceName}}Count++
	if h.behavior.Handle{{.InterfaceName}}Panic {
		panic("Handle{{.InterfaceName}}")
	}

	h.data.Handle{{.InterfaceName}}Instances = instance
	h.data.Handle{{.InterfaceName}}QuotaArgs = quotaArgs
	return h.behavior.Handle{{.InterfaceName}}Result, h.behavior.Handle{{.InterfaceName}}Err
}
{{end}}
{{end}}


// GetAdptInfoFn returns the infoFn for the Adapter.
func (s *Adapter) GetAdptInfoFn() adapter.InfoFn {
	return func() adapter.Info {
		return adapter.Info{
			Name:               s.Behavior.Name,
			Description:        "",
			SupportedTemplates: []string{
        	{{range .TemplateModels}}
				{{.GoPackageName}}.TemplateName,
        	{{end}}
			},
			NewBuilder: func() adapter.HandlerBuilder {
				return builder{
					behavior:        s.Behavior.Builder,
					data:            s.BuilderData,
					handlerBehavior: s.Behavior.Handler,
					handlerData:     s.HandlerData,
				}
			},
			DefaultConfig: &types.Empty{},
			Impl:          "ThisIsASpyAdapter",
		}
	}
}

`