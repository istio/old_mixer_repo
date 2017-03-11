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

package api

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"testing"

	rpc "github.com/googleapis/googleapis/google/rpc"

	mixerpb "istio.io/api/mixer/v1"
	"istio.io/mixer/pkg/aspect"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/config"
	"istio.io/mixer/pkg/status"
)

type fakeresolver struct {
	ret []*config.Combined
	err error
}

func (f *fakeresolver) Resolve(bag attribute.Bag, aspectSet config.AspectSet) ([]*config.Combined, error) {
	return f.ret, f.err
}

type fakeExecutor struct {
	body func() aspect.Output
}

// Execute takes a set of configurations and Executes all of them.
func (f *fakeExecutor) Execute(ctx context.Context, cfgs []*config.Combined, attrs attribute.Bag, ma aspect.APIMethodArgs) aspect.Output {
	return f.body()
}

func TestAspectManagerErrorsPropagated(t *testing.T) {
	f := &fakeExecutor{func() aspect.Output {
		return aspect.Output{Status: status.WithError(fmt.Errorf("expected"))}
	}}
	h := NewHandler(f, map[aspect.APIMethod]config.AspectSet{}).(*handlerState)
	h.ConfigChange(&fakeresolver{[]*config.Combined{nil, nil}, nil})

	bag, _ := attribute.NewManager().NewTracker().ApplyAttributes(&mixerpb.Attributes{})
	o := h.execute(context.Background(), bag, aspect.CheckMethod, nil)
	if o.Status.Code != int32(rpc.INTERNAL) {
		t.Errorf("execute(..., invalidConfig, ...) returned %v, wanted status with code %v", o.Status, rpc.INTERNAL)
	}
}

func TestHandler(t *testing.T) {
	bag, _ := attribute.NewManager().NewTracker().ApplyAttributes(&mixerpb.Attributes{})

	checkReq := &mixerpb.CheckRequest{}
	checkResp := &mixerpb.CheckResponse{}
	reportReq := &mixerpb.ReportRequest{}
	reportResp := &mixerpb.ReportResponse{}
	quotaReq := &mixerpb.QuotaRequest{}
	quotaResp := &mixerpb.QuotaResponse{}

	// should all fail because there is no active config
	h := NewHandler(&fakeExecutor{}, map[aspect.APIMethod]config.AspectSet{}).(*handlerState)
	h.Check(context.Background(), bag, checkReq, checkResp)
	h.Report(context.Background(), bag, reportReq, reportResp)
	h.Quota(context.Background(), bag, quotaReq, quotaResp)

	if checkResp.Result.Code != int32(rpc.INTERNAL) || reportResp.Result.Code != int32(rpc.INTERNAL) || quotaResp.Result.Code != int32(rpc.INTERNAL) {
		t.Error("Expected rpc.INTERNAL for all responses")
	}

	r := &fakeresolver{[]*config.Combined{nil, nil}, nil}
	r.err = fmt.Errorf("RESOLVER")
	h.ConfigChange(r)

	// Should all fail due to a resolver error
	h.Check(context.Background(), bag, checkReq, checkResp)
	h.Report(context.Background(), bag, reportReq, reportResp)
	h.Quota(context.Background(), bag, quotaReq, quotaResp)

	if checkResp.Result.Code != int32(rpc.INTERNAL) || reportResp.Result.Code != int32(rpc.INTERNAL) || quotaResp.Result.Code != int32(rpc.INTERNAL) {
		t.Error("Expected rpc.INTERNAL for all responses")
	}

	if !strings.Contains(checkResp.Result.Message, "RESOLVER") ||
		!strings.Contains(reportResp.Result.Message, "RESOLVER") ||
		!strings.Contains(quotaResp.Result.Message, "RESOLVER") {
		t.Errorf("Expected RESOLVER in error messages, got %s, %s, %s", checkResp.Result.Message, reportResp.Result.Message, quotaResp.Result.Message)
	}

	f := &fakeExecutor{func() aspect.Output {
		return aspect.Output{Status: status.WithInternal("BADASPECT")}
	}}
	r = &fakeresolver{[]*config.Combined{nil, nil}, nil}
	h = NewHandler(f, map[aspect.APIMethod]config.AspectSet{}).(*handlerState)
	h.ConfigChange(r)

	// Should all fail due to a bad aspect
	h.Check(context.Background(), bag, checkReq, checkResp)
	h.Report(context.Background(), bag, reportReq, reportResp)
	h.Quota(context.Background(), bag, quotaReq, quotaResp)

	if checkResp.Result.Code != int32(rpc.INTERNAL) ||
		reportResp.Result.Code != int32(rpc.INTERNAL) ||
		quotaResp.Result.Code != int32(rpc.INTERNAL) {
		t.Error("Expected rpc.INTERNAL for all responses")
	}

	if !strings.Contains(checkResp.Result.Message, "BADASPECT") ||
		!strings.Contains(reportResp.Result.Message, "BADASPECT") ||
		!strings.Contains(quotaResp.Result.Message, "BADASPECT") {
		t.Errorf("Expected BADASPECT in error messages, got %s, %s, %s", checkResp.Result.Message, reportResp.Result.Message, quotaResp.Result.Message)
	}

	f = &fakeExecutor{func() aspect.Output {
		return aspect.Output{Status: status.OK, Response: &aspect.QuotaMethodResp{Amount: 42}}
	}}
	r = &fakeresolver{[]*config.Combined{nil, nil}, nil}
	h = NewHandler(f, map[aspect.APIMethod]config.AspectSet{}).(*handlerState)
	h.ConfigChange(r)

	// Should succeed
	h.Quota(context.Background(), bag, quotaReq, quotaResp)

	if !status.IsOK(quotaResp.Result) {
		t.Errorf("Expected successful quota allocation, got %v", quotaResp.Result)
	}

	if quotaResp.Amount != 42 {
		t.Errorf("Expected 42, got %v", quotaResp.Amount)
	}
}

func init() {
	// bump up the log level so log-only logic runs during the tests, for correctness and coverage.
	_ = flag.Lookup("v").Value.Set("99")
}
