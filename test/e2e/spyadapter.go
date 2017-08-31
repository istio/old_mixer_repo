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

package e2e

import (
	"context"

	"github.com/gogo/protobuf/types"

	"istio.io/mixer/pkg/adapter"
	reportTmpl "istio.io/mixer/test/e2e/template/report"
)

type (
	fakeHndlr struct { //nolint: aligncheck
		hndlrbehavior hndlrBehavior
		hndlrCallData *hndlrCallData
	}

	fakeBldr struct { //nolint: aligncheck
		// return values from the builder
		bldrbehavior builderBehavior
		// return values from the handler
		hndlrbehavior hndlrBehavior

		// records input to the builder function.
		bldrCallData *bldrCallData
		// records input to the handler function.
		hndlrCallData *hndlrCallData
	}

	// SpyAdapter is a fake Adapter. It is used for controlling the Adapter's behavior as well as
	// inspect the input values that adapter receives from Mixer
	SpyAdapter struct {
		// builder and handler behavior
		behavior adptBehavior
		// records input to the builder function.
		bldrCallData *bldrCallData
		// records input to the handler function.
		hndlrCallData *hndlrCallData
	}

	adptBehavior struct {
		// adapter name
		name string
		// builder behavior (return values + should panic)
		bldrBehavior builderBehavior
		// handler behavior (return values + should panic)
		hndlrBehavior hndlrBehavior
	}

	// nolint
	hndlrBehavior struct {
		// error to returned
		HandleSampleReport_Error error
		// error to returned
		Close_Error error

		// should panic
		HandleSampleReport_Panic bool
		// should panic
		Close_Panic bool
	}

	// nolint
	hndlrCallData struct {
		// input to the method
		HandleSampleReport_Instances []*reportTmpl.Instance
		// no of time called
		HandleSampleReport_Cnt int

		// no of time called
		Close_Cnt int
	}

	// nolint
	builderBehavior struct {
		// error to returned
		ConfigureSampleReportHandler_Err error
		// should panic
		ConfigureSampleReportHandler_Panic bool

		// error to return
		Build_Err error
		// should panic
		Build_Panic bool
	}

	// nolint
	bldrCallData struct {
		// no of time called
		ConfigureSampleReportHandler_Cnt int
		// input to the method
		ConfigureSampleReportHandler_Types map[string]*reportTmpl.Type

		// input to the method
		Build_AdptCnfg adapter.Config
		// no of time called
		Build_Cnt int
	}
)

func (f fakeBldr) Build(cnfg adapter.Config, _ adapter.Env) (adapter.Handler, error) {
	f.bldrCallData.Build_Cnt++
	if f.bldrbehavior.Build_Panic {
		panic("Build")
	}

	f.bldrCallData.Build_AdptCnfg = cnfg
	hndlr := fakeHndlr{hndlrbehavior: f.hndlrbehavior, hndlrCallData: f.hndlrCallData}
	return hndlr, f.bldrbehavior.Build_Err
}
func (f fakeBldr) ConfigureSampleReportHandler(typeParams map[string]*reportTmpl.Type) error {
	f.bldrCallData.ConfigureSampleReportHandler_Cnt++
	if f.bldrbehavior.ConfigureSampleReportHandler_Panic {
		panic("ConfigureSampleReportHandler")
	}

	f.bldrCallData.ConfigureSampleReportHandler_Types = typeParams
	return f.bldrbehavior.ConfigureSampleReportHandler_Err
}

func (f fakeHndlr) HandleSampleReport(ctx context.Context, instances []*reportTmpl.Instance) error {
	f.hndlrCallData.HandleSampleReport_Cnt++
	if f.hndlrbehavior.HandleSampleReport_Panic {
		panic("HandleSampleReport")
	}

	f.hndlrCallData.HandleSampleReport_Instances = instances
	return f.hndlrbehavior.HandleSampleReport_Error
}

func (f fakeHndlr) Close() error {
	f.hndlrCallData.Close_Cnt++
	if f.hndlrbehavior.Close_Panic {
		panic("Close")
	}

	return f.hndlrbehavior.Close_Error
}

func newSpyAdapter(b adptBehavior) *SpyAdapter {
	return &SpyAdapter{behavior: b, bldrCallData: &bldrCallData{}, hndlrCallData: &hndlrCallData{}}
}

func (s *SpyAdapter) getAdptInfoFn() adapter.InfoFn {
	return func() adapter.BuilderInfo {
		return adapter.BuilderInfo{
			Name:               s.behavior.name,
			Description:        "",
			SupportedTemplates: []string{reportTmpl.TemplateName},
			CreateHandlerBuilder: func() adapter.HandlerBuilder {
				return fakeBldr{
					bldrbehavior:  s.behavior.bldrBehavior,
					bldrCallData:  s.bldrCallData,
					hndlrbehavior: s.behavior.hndlrBehavior,
					hndlrCallData: s.hndlrCallData,
				}
			},
			DefaultConfig: &types.Empty{},
			ValidateConfig: func(msg adapter.Config) *adapter.ConfigErrors {
				return nil
			},
		}
	}
}

/*
I0831 21:54:01.727] test/e2e/spyadapter.go:55:2:warning: struct adptBehavior could have size 104 (currently 112) (aligncheck)
I0831 21:54:01.727] test/e2e/spyadapter.go:64:2:warning: struct hndlrBehavior could have size 40 (currently 48) (aligncheck)
I0831 21:54:01.728] test/e2e/spyadapter.go:86:2:warning: struct builderBehavior could have size 40 (currently 48) (aligncheck)
*/
