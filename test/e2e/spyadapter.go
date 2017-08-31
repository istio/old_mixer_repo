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
	"github.com/gogo/protobuf/types"

	"context"
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
		// records input to the builder function.
		bldrCallData *bldrCallData

		// return values from the handler
		hndlrbehavior hndlrBehavior
		// records input to the handler function.
		hndlrCallData *hndlrCallData
	}

	spyAdapter struct {
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
		// handler behavior (return values + should panic)
		hndlrBehavior hndlrBehavior
		// builder behavior (return values + should panic)
		bldrBehavior builderBehavior
	}

	hndlrBehavior struct {
		// error to returned
		HandleSampleReport_error error
		// should panic
		HandleSampleReport_panic bool

		// error to returned
		Close_error error
		// should panic
		Close_panic bool
	}

	hndlrCallData struct {
		// input to the method
		HandleSampleReport_instances []*reportTmpl.Instance
		// no of time called
		HandleSampleReport_cnt int

		// no of time called
		Close_cnt int
	}

	builderBehavior struct {
		// error to returned
		ConfigureSampleReportHandler_err error
		// should panic
		ConfigureSampleReportHandler_panic bool

		// error to return
		Build_err error
		// should panic
		Build_panic bool
	}

	bldrCallData struct {
		// no of time called
		ConfigureSampleReportHandler_cnt int
		// input to the method
		ConfigureSampleReportHandler_types map[string]*reportTmpl.Type

		// no of time called
		Build_cnt int
		// input to the method
		Build_adptCnfg adapter.Config
	}
)

func (f fakeBldr) Build(cnfg adapter.Config, _ adapter.Env) (adapter.Handler, error) {
	f.bldrCallData.Build_cnt++
	if f.bldrbehavior.Build_panic {
		panic("Build")
	}

	f.bldrCallData.Build_adptCnfg = cnfg
	hndlr := fakeHndlr{hndlrbehavior: f.hndlrbehavior, hndlrCallData: f.hndlrCallData}
	return hndlr, f.bldrbehavior.Build_err
}
func (f fakeBldr) ConfigureSampleReportHandler(typeParams map[string]*reportTmpl.Type) error {
	f.bldrCallData.ConfigureSampleReportHandler_cnt++
	if f.bldrbehavior.ConfigureSampleReportHandler_panic {
		panic("ConfigureSampleReportHandler")
	}

	f.bldrCallData.ConfigureSampleReportHandler_types = typeParams
	return f.bldrbehavior.ConfigureSampleReportHandler_err
}

func (f fakeHndlr) HandleSampleReport(ctx context.Context, instances []*reportTmpl.Instance) error {
	f.hndlrCallData.HandleSampleReport_cnt++
	if f.hndlrbehavior.HandleSampleReport_panic {
		panic("HandleSampleReport")
	}

	f.hndlrCallData.HandleSampleReport_instances = instances
	return f.hndlrbehavior.HandleSampleReport_error
}

func (f fakeHndlr) Close() error {
	f.hndlrCallData.Close_cnt++
	if f.hndlrbehavior.Close_panic {
		panic("Close")
	}

	return f.hndlrbehavior.Close_error
}

func newSpyAdapter(b adptBehavior) *spyAdapter {
	return &spyAdapter{behavior: b, bldrCallData: &bldrCallData{}, hndlrCallData: &hndlrCallData{}}
}

func (s *spyAdapter) getAdptInfoFn() adapter.InfoFn {
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
