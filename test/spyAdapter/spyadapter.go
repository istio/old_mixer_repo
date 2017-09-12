// Copyright 2017 Istio Authors
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

// NOTE: TODO : Auto-generate this file for given templates

package spyAdapter

import (
	"context"

	"github.com/gogo/protobuf/types"

	"istio.io/mixer/pkg/adapter"
	reportTmpl "istio.io/mixer/test/template/report"
)

type (

	// Adapter is a fake Adapter. It is used for controlling the Adapter's behavior as well as
	// inspect the input values that adapter receives from Mixer
	// nolint
	Adapter struct {
		Behavior    AdapterBehavior
		BuilderData *builderData
		HandlerData *handlerData
	}

	// nolint
	AdapterBehavior struct {
		Name    string
		Builder BuilderBehavior
		Handler HandlerBehavior
	}

	// nolint
	HandlerBehavior struct {
		HandleSampleReport_Error error
		HandleSampleReport_Panic bool

		Close_Error error
		Close_Panic bool
	}

	// nolint
	BuilderBehavior struct {
		SetSampleReportTypes_Panic bool

		SetAdapterConfig_Panic bool

		Validate_Err   *adapter.ConfigErrors
		Validate_Panic bool

		Build_Err   error
		Build_Panic bool
	}

	// nolint
	builder struct {
		behavior        BuilderBehavior
		handlerBehavior HandlerBehavior
		data            *builderData
		handlerData     *handlerData
	}

	// nolint
	handler struct {
		behavior HandlerBehavior
		data     *handlerData
	}

	// nolint
	handlerData struct {
		HandleSampleReport_Instances []*reportTmpl.Instance
		HandleSampleReport_Count     int

		Close_Count int
	}

	// nolint
	builderData struct {
		// no of time called
		SetSampleReportTypes_Count int
		// input to the method
		SetSampleReportTypes_Types map[string]*reportTmpl.Type

		SetAdapterConfig_AdptCfg adapter.Config
		SetAdapterConfig_Count   int

		Validate_Count int

		Build_Count int
		Build_Ctx   context.Context
		Build_Env   adapter.Env
	}
)

func (b builder) Build(ctx context.Context, env adapter.Env) (adapter.Handler, error) {
	b.data.Build_Count++
	if b.behavior.Build_Panic {
		panic("Build")
	}

	b.data.Build_Ctx = ctx
	b.data.Build_Env = env

	return handler{behavior: b.handlerBehavior, data: b.handlerData}, b.behavior.Build_Err
}

func (b builder) SetSampleReportTypes(typeParams map[string]*reportTmpl.Type) {
	b.data.SetSampleReportTypes_Count++
	b.data.SetSampleReportTypes_Types = typeParams

	if b.behavior.SetSampleReportTypes_Panic {
		panic("SetSampleReportTypes")
	}
}

func (b builder) SetAdapterConfig(cfg adapter.Config) {
	b.data.SetAdapterConfig_Count++
	b.data.SetAdapterConfig_AdptCfg = cfg

	if b.behavior.SetAdapterConfig_Panic {
		panic("SetAdapterConfig")
	}
}

func (b builder) Validate() *adapter.ConfigErrors {
	b.data.Validate_Count++
	if b.behavior.Validate_Panic {
		panic("Validate")
	}

	return b.behavior.Validate_Err
}

func (h handler) HandleSampleReport(ctx context.Context, instances []*reportTmpl.Instance) error {
	h.data.HandleSampleReport_Count++
	if h.behavior.HandleSampleReport_Panic {
		panic("HandleSampleReport")
	}

	h.data.HandleSampleReport_Instances = instances
	return h.behavior.HandleSampleReport_Error
}

func (h handler) Close() error {
	h.data.Close_Count++
	if h.behavior.Close_Panic {
		panic("Close")
	}

	return h.behavior.Close_Error
}

// NewSpyAdapter returns a new instance of Adapter with the given behavior
func NewSpyAdapter(b AdapterBehavior) *Adapter {
	return &Adapter{Behavior: b, BuilderData: &builderData{}, HandlerData: &handlerData{}}
}

// GetAdptInfoFn returns the infoFn for the Adapter.
func (s *Adapter) GetAdptInfoFn() adapter.InfoFn {
	return func() adapter.Info {
		return adapter.Info{
			Name:               s.Behavior.Name,
			Description:        "",
			SupportedTemplates: []string{reportTmpl.TemplateName},
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
