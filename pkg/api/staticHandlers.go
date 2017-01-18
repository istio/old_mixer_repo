// Copyright 2017 Google Inc.
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

// StaticHandlers.go exposes an implementation of MethodHandlers with static (compile time) configs.

package api

import (
	"context"
	"fmt"

	"github.com/golang/glog"
	"github.com/golang/protobuf/ptypes/struct"

	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/genproto/googleapis/rpc/status"

	denyadapter "istio.io/mixer/adapter/denyChecker"
	"istio.io/mixer/pkg/aspectsupport"
	denysupport "istio.io/mixer/pkg/aspectsupport/denyChecker"
	"istio.io/mixer/pkg/aspectsupport/uber"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/expr"

	mixerpb "istio.io/api/mixer/v1"
	istioconfig "istio.io/api/mixer/v1/config"
)

// StaticHandlers is an implementation of MethodHandlers whose configs are static and derived at compile time.
type StaticHandlers struct {
	mngr uber.Manager
	eval expr.Evaluator

	// Configs for the aspects that'll be used to serve each API method.
	check  []*aspectsupport.CombinedConfig
	report []*aspectsupport.CombinedConfig
	quota  []*aspectsupport.CombinedConfig
}

// NewStaticHandlers returns a new set of MethodHandlers whose configuration is determined at compile time. These static
// configs live in istio.io/mixer/pkg/api/staticHandlers.go
func NewStaticHandlers() *StaticHandlers {
	registry := uber.NewRegistry()

	// This impl loops over the configs for each method, i.e. when Mixer.Check is called we loop over all config
	// entries in the StaticHandlers.check array; the same is true for Report and Quota with their respective arrays.
	// To add a new adapter to the set used to handle requests:
	// 1. Add the call `myAdapter.Register(registry)`
	// 2. Add the manager to the `managers` array
	// 3. Add the minimum required config to the static config arrays; if you want an adapter to be called
	//    for e.g. both Check and Report, the config must be added to both arrays.

	if err := denyadapter.Register(registry); err != nil {
		panic("Failed to register denyChecker in our static handlers with err: " + err.Error())
	}

	managers := []aspectsupport.Manager{
		denysupport.NewManager(),
	}

	// When extracting the aspect config for a given adapter, the uber manager doesn't care about what constants we
	// shove into the config fields so long as Aspect.Kind == Adapter.Impl. Setting additional fields like Aspect.Adapter
	// or Adapter.Kind only help to stop collisions in the uber manager's cache. The only additional config needed
	// is the config required by the particular adapter impl.
	checkConfs := []*aspectsupport.CombinedConfig{
		// denyChecker ignores its configs
		{
			&istioconfig.Aspect{
				Kind:    "istio/denyChecker",
				Adapter: "",
				Inputs:  make(map[string]string),
				Params:  new(structpb.Struct),
			},
			&istioconfig.Adapter{
				Name:   "",
				Kind:   "",
				Impl:   "istio/denyChecker",
				Params: new(structpb.Struct),
			},
		},
	}

	reportConfs := []*aspectsupport.CombinedConfig{}
	quotaConfs := []*aspectsupport.CombinedConfig{}

	return &StaticHandlers{
		mngr:   *uber.NewManager(registry, managers),
		eval:   expr.NewIdentityEvaluator(),
		check:  checkConfs,
		report: reportConfs,
		quota:  quotaConfs,
	}
}

func (s *StaticHandlers) handle(
	ctx context.Context,
	tracker attribute.Tracker,
	attrs *mixerpb.Attributes,
	configs []*aspectsupport.CombinedConfig,
	output func(*status.Status)) {

	ab, err := tracker.StartRequest(attrs)
	defer tracker.EndRequest()
	if err != nil {
		glog.Warningf("Unable to process attribute update. error: '%v'", err)
		output(newStatus(code.Code_INVALID_ARGUMENT))
		return
	}

	// Loop through all of the configs registered for the call. We exit at the first error or non-OK status
	// returned by an adapter, and return the last adapter's status otherwise.
	var out *aspectsupport.Output
	for _, conf := range configs {
		out, err = s.mngr.Execute(conf, ab, s.eval)
		if err != nil {
			errorStr := fmt.Sprintf("Adapter %s returned err: %v", conf.Adapter.Name, err)
			glog.Warning(errorStr)
			output(newStatusWithMessage(code.Code_INTERNAL, errorStr))
			return
		}
		if out.Code != code.Code_OK {
			glog.Infof("Adapter '%s' returned not-OK code: %v", conf.Adapter.Name, out.Code)
			output(newStatusWithMessage(out.Code, "Rejected by adapter "+conf.Adapter.Name))
			return
		}
	}
	output(newStatus(code.Code_OK))
}

// Check performs the configured set of precondition checks.
// Note that the request parameter is immutable, while the response parameter is where
// results are specified
func (s *StaticHandlers) Check(ctx context.Context, tracker attribute.Tracker, req *mixerpb.CheckRequest, resp *mixerpb.CheckResponse) {
	resp.RequestIndex = req.RequestIndex
	s.handle(ctx, tracker, req.AttributeUpdate, s.check, func(s *status.Status) { resp.Result = s })
}

// Report performs the requested set of reporting operations.
// Note that the request parameter is immutable, while the response parameter is where
// results are specified
func (s *StaticHandlers) Report(ctx context.Context, tracker attribute.Tracker, req *mixerpb.ReportRequest, resp *mixerpb.ReportResponse) {
	resp.RequestIndex = req.RequestIndex
	s.handle(ctx, tracker, req.AttributeUpdate, s.report, func(s *status.Status) { resp.Result = s })
}

// Quota increments, decrements, or queries the specified quotas.
// Note that the request parameter is immutable, while the response parameter is where
// results are specified
func (s *StaticHandlers) Quota(ctx context.Context, tracker attribute.Tracker, req *mixerpb.QuotaRequest, resp *mixerpb.QuotaResponse) {
	resp.RequestIndex = req.RequestIndex
	okVal := code.Code_value["OK"]
	s.handle(ctx, tracker, req.AttributeUpdate, s.quota, func(s *status.Status) {
		if s.Code != okVal {
			resp.Result = &mixerpb.QuotaResponse_Error{Error: s}
		} else {
			resp.Result = &mixerpb.QuotaResponse_EffectiveAmount{EffectiveAmount: 0}
		}
	})
}
