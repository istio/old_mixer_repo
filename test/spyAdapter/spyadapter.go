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
		AdptBhvr      AdptBhvr
		BldrCallData  *bldrCallData
		HndlrCallData *hndlrCallData
	}

	// nolint
	AdptBhvr struct {
		Name      string
		BldrBhvr  BldrBhvr
		HndlrBhvr HndlrBhvr
	}

	// nolint
	HndlrBhvr struct {
		HandleSampleReport_Error error
		HandleSampleReport_Panic bool

		Close_Error error
		Close_Panic bool
	}

	// nolint
	BldrBhvr struct {
		SetSampleReportTypes_Panic bool

		SetAdapterConfig_Panic bool

		Validate_Err   *adapter.ConfigErrors
		Validate_Panic bool

		Build_Err   error
		Build_Panic bool
	}

	// nolint
	bldr struct {
		bldrbehavior  BldrBhvr
		hndlrbehavior HndlrBhvr

		bldrCallData  *bldrCallData
		hndlrCallData *hndlrCallData
	}

	// nolint
	hndlr struct {
		hndlrbehavior HndlrBhvr
		hndlrCallData *hndlrCallData
	}

	// nolint
	hndlrCallData struct {
		HandleSampleReport_Instances []*reportTmpl.Instance
		HandleSampleReport_Cnt       int

		Close_Cnt int
	}

	// nolint
	bldrCallData struct {
		// no of time called
		SetSampleReportTypes_Cnt int
		// input to the method
		SetSampleReportTypes_Types map[string]*reportTmpl.Type

		SetAdapterConfig_AdptCnfg adapter.Config
		SetAdapterConfig_Cnt      int

		Validate_Cnt int

		Build_Cnt int
		Build_Ctx context.Context
		Build_Env adapter.Env
	}
)

func (f bldr) Build(ctx context.Context, env adapter.Env) (adapter.Handler, error) {
	f.bldrCallData.Build_Cnt++
	if f.bldrbehavior.Build_Panic {
		panic("Build")
	}

	f.bldrCallData.Build_Ctx = ctx
	f.bldrCallData.Build_Env = env

	return hndlr{hndlrbehavior: f.hndlrbehavior, hndlrCallData: f.hndlrCallData}, f.bldrbehavior.Build_Err
}

func (f bldr) SetSampleReportTypes(typeParams map[string]*reportTmpl.Type) {
	f.bldrCallData.SetSampleReportTypes_Cnt++
	f.bldrCallData.SetSampleReportTypes_Types = typeParams

	if f.bldrbehavior.SetSampleReportTypes_Panic {
		panic("SetSampleReportTypes")
	}
}

func (f bldr) SetAdapterConfig(cnfg adapter.Config) {
	f.bldrCallData.SetAdapterConfig_Cnt++
	f.bldrCallData.SetAdapterConfig_AdptCnfg = cnfg

	if f.bldrbehavior.SetAdapterConfig_Panic {
		panic("SetAdapterConfig")
	}
}

func (f bldr) Validate() *adapter.ConfigErrors {
	f.bldrCallData.Validate_Cnt++
	if f.bldrbehavior.Validate_Panic {
		panic("Validate")
	}

	return f.bldrbehavior.Validate_Err
}

func (f hndlr) HandleSampleReport(ctx context.Context, instances []*reportTmpl.Instance) error {
	f.hndlrCallData.HandleSampleReport_Cnt++
	if f.hndlrbehavior.HandleSampleReport_Panic {
		panic("HandleSampleReport")
	}

	f.hndlrCallData.HandleSampleReport_Instances = instances
	return f.hndlrbehavior.HandleSampleReport_Error
}

func (f hndlr) Close() error {
	f.hndlrCallData.Close_Cnt++
	if f.hndlrbehavior.Close_Panic {
		panic("Close")
	}

	return f.hndlrbehavior.Close_Error
}

// NewSpyAdapter returns a new instance of Adapter with the given behavior
func NewSpyAdapter(b AdptBhvr) *Adptr {
	return &Adptr{AdptBhvr: b, BldrCallData: &bldrCallData{}, HndlrCallData: &hndlrCallData{}}
}

// GetAdptInfoFn returns the infoFn for the Adapter.
func (s *Adptr) GetAdptInfoFn() adapter.InfoFn {
	return func() adapter.Info {
		return adapter.Info{
			Name:               s.AdptBhvr.Name,
			Description:        "",
			SupportedTemplates: []string{reportTmpl.TemplateName},
			NewBuilder: func() adapter.HandlerBuilder {
				return bldr{
					bldrbehavior:  s.AdptBhvr.BldrBhvr,
					bldrCallData:  s.BldrCallData,
					hndlrbehavior: s.AdptBhvr.HndlrBhvr,
					hndlrCallData: s.HndlrCallData,
				}
			},
			DefaultConfig: &types.Empty{},
			Impl:          "ThisIsASpyAdapter",
		}
	}
}
