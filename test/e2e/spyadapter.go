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
	fakeHndlr struct {
		hndlrbehavior hndlrBehavior
		hndlrCallData *hndlrCallData
	}

	fakeBldr struct {
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

	hndlrBehavior struct {
		// error to returned
		HandleSampleReportError error
		// error to returned
		CloseError error

		// should panic
		HandleSampleReportPanic bool
		// should panic
		ClosePanic bool
	}

	hndlrCallData struct {
		// input to the method
		HandleSampleReportInstances []*reportTmpl.Instance
		// no of time called
		HandleSampleReportCnt int

		// no of time called
		CloseCnt int
	}

	builderBehavior struct {
		// error to returned
		ConfigureSampleReportHandlerErr error
		// error to return
		BuildErr error

		// should panic
		ConfigureSampleReportHandlerPanic bool
		// should panic
		BuildPanic bool
	}

	bldrCallData struct {
		// no of time called
		ConfigureSampleReportHandlerCnt int
		// input to the method
		ConfigureSampleReportHandlerTypes map[string]*reportTmpl.Type

		// no of time called
		BuildCnt int
		// input to the method
		BuildAdptCnfg adapter.Config
	}
)

func (f fakeBldr) Build(cnfg adapter.Config, _ adapter.Env) (adapter.Handler, error) {
	f.bldrCallData.BuildCnt++
	if f.bldrbehavior.BuildPanic {
		panic("Build")
	}

	f.bldrCallData.BuildAdptCnfg = cnfg
	hndlr := fakeHndlr{hndlrbehavior: f.hndlrbehavior, hndlrCallData: f.hndlrCallData}
	return hndlr, f.bldrbehavior.BuildErr
}
func (f fakeBldr) ConfigureSampleReportHandler(typeParams map[string]*reportTmpl.Type) error {
	f.bldrCallData.ConfigureSampleReportHandlerCnt++
	if f.bldrbehavior.ConfigureSampleReportHandlerPanic {
		panic("ConfigureSampleReportHandler")
	}

	f.bldrCallData.ConfigureSampleReportHandlerTypes = typeParams
	return f.bldrbehavior.ConfigureSampleReportHandlerErr
}

func (f fakeHndlr) HandleSampleReport(ctx context.Context, instances []*reportTmpl.Instance) error {
	f.hndlrCallData.HandleSampleReportCnt++
	if f.hndlrbehavior.HandleSampleReportPanic {
		panic("HandleSampleReport")
	}

	f.hndlrCallData.HandleSampleReportInstances = instances
	return f.hndlrbehavior.HandleSampleReportError
}

func (f fakeHndlr) Close() error {
	f.hndlrCallData.CloseCnt++
	if f.hndlrbehavior.ClosePanic {
		panic("Close")
	}

	return f.hndlrbehavior.CloseError
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
