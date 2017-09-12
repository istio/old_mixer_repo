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

	// Adptr is a fake Adapter. It is used for controlling the Adapter's behavior as well as
	// inspect the input values that adapter receives from Mixer
	// nolint
	Adptr struct {
		AdptBehavior    AdptBehavior
		BldrCallData    *builderCallData
		HandlerCallData *handlerCallData
	}

	// nolint
	AdptBehavior struct {
		Name            string
		BuilderBehavior BuilderBehavior
		HandlerBhavior  HandlerBehavior
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
	bldr struct {
		builderBehavior BuilderBehavior
		handlerBehavior HandlerBehavior

		builderCallData *builderCallData
		handlerCallData *handlerCallData
	}

	// nolint
	hndlr struct {
		hndlrbehavior HandlerBehavior
		hndlrCallData *handlerCallData
	}

	// nolint
	handlerCallData struct {
		HandleSampleReport_Instances []*reportTmpl.Instance
		HandleSampleReport_Ctr       int

		Close_Ctr int
	}

	// nolint
	builderCallData struct {
		// no of time called
		SetSampleReportTypes_Ctr int
		// input to the method
		SetSampleReportTypes_Types map[string]*reportTmpl.Type

		SetAdapterConfig_AdptCfg adapter.Config
		SetAdapterConfig_Ctr     int

		Validate_Ctr int

		Build_Ctr int
		Build_Ctx context.Context
		Build_Env adapter.Env
	}
)

func (f bldr) Build(ctx context.Context, env adapter.Env) (adapter.Handler, error) {
	f.builderCallData.Build_Ctr++
	if f.builderBehavior.Build_Panic {
		panic("Build")
	}

	f.builderCallData.Build_Ctx = ctx
	f.builderCallData.Build_Env = env

	return hndlr{hndlrbehavior: f.handlerBehavior, hndlrCallData: f.handlerCallData}, f.builderBehavior.Build_Err
}

func (f bldr) SetSampleReportTypes(typeParams map[string]*reportTmpl.Type) {
	f.builderCallData.SetSampleReportTypes_Ctr++
	f.builderCallData.SetSampleReportTypes_Types = typeParams

	if f.builderBehavior.SetSampleReportTypes_Panic {
		panic("SetSampleReportTypes")
	}
}

func (f bldr) SetAdapterConfig(cfg adapter.Config) {
	f.builderCallData.SetAdapterConfig_Ctr++
	f.builderCallData.SetAdapterConfig_AdptCfg = cfg

	if f.builderBehavior.SetAdapterConfig_Panic {
		panic("SetAdapterConfig")
	}
}

func (f bldr) Validate() *adapter.ConfigErrors {
	f.builderCallData.Validate_Ctr++
	if f.builderBehavior.Validate_Panic {
		panic("Validate")
	}

	return f.builderBehavior.Validate_Err
}

func (f hndlr) HandleSampleReport(ctx context.Context, instances []*reportTmpl.Instance) error {
	f.hndlrCallData.HandleSampleReport_Ctr++
	if f.hndlrbehavior.HandleSampleReport_Panic {
		panic("HandleSampleReport")
	}

	f.hndlrCallData.HandleSampleReport_Instances = instances
	return f.hndlrbehavior.HandleSampleReport_Error
}

func (f hndlr) Close() error {
	f.hndlrCallData.Close_Ctr++
	if f.hndlrbehavior.Close_Panic {
		panic("Close")
	}

	return f.hndlrbehavior.Close_Error
}

// NewSpyAdapter returns a new instance of Adapter with the given behavior
func NewSpyAdapter(b AdptBehavior) *Adptr {
	return &Adptr{AdptBehavior: b, BldrCallData: &builderCallData{}, HandlerCallData: &handlerCallData{}}
}

// GetAdptInfoFn returns the infoFn for the Adapter.
func (s *Adptr) GetAdptInfoFn() adapter.InfoFn {
	return func() adapter.Info {
		return adapter.Info{
			Name:               s.AdptBehavior.Name,
			Description:        "",
			SupportedTemplates: []string{reportTmpl.TemplateName},
			NewBuilder: func() adapter.HandlerBuilder {
				return bldr{
					builderBehavior: s.AdptBehavior.BuilderBehavior,
					builderCallData: s.BldrCallData,
					handlerBehavior: s.AdptBehavior.HandlerBhavior,
					handlerCallData: s.HandlerCallData,
				}
			},
			DefaultConfig: &types.Empty{},
			Impl:          "ThisIsASpyAdapter",
		}
	}
}
