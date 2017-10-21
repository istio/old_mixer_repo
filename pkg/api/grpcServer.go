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

package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	rpc "github.com/googleapis/googleapis/google/rpc"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	tags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	legacyContext "golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	mixerpb "istio.io/api/mixer/v1"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/adapterManager"
	"istio.io/mixer/pkg/aspect"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/pool"
	"istio.io/mixer/pkg/runtime"
	"istio.io/mixer/pkg/status"
)

// We have a slightly messy situation around the use of context objects. gRPC stubs are
// generated to expect the old "x/net/context" types instead of the more modern "context".
// We end up doing a quick switcharoo from the gRPC defined type to the modern type so we can
// use the modern type elsewhere in the code.

type (
	// grpcServer holds the dispatchState for the gRPC API server.
	grpcServer struct {
		dispatcher       runtime.Dispatcher
		aspectDispatcher adapterManager.AspectDispatcher
		gp               *pool.GoroutinePool

		// the global dictionary. This will eventually be writable via config
		globalWordList []string
		globalDict     map[string]int32
	}
)

const (
	// defaultValidDuration is the default duration for which a check or quota result is valid.
	defaultValidDuration = 10 * time.Second
	// defaultValidUseCount is the default number of calls for which a check or quota result is valid.
	defaultValidUseCount = 200
)

var checkOk = &adapter.CheckResult{
	ValidDuration: defaultValidDuration,
	ValidUseCount: defaultValidUseCount,
}

// NewGRPCServer creates a gRPC serving stack.
func NewGRPCServer(aspectDispatcher adapterManager.AspectDispatcher, dispatcher runtime.Dispatcher, gp *pool.GoroutinePool) mixerpb.MixerServer {
	list := attribute.GlobalList()
	globalDict := make(map[string]int32, len(list))
	for i := 0; i < len(list); i++ {
		globalDict[list[i]] = int32(i)
	}

	return &grpcServer{
		dispatcher:       dispatcher,
		aspectDispatcher: aspectDispatcher,
		gp:               gp,
		globalWordList:   list,
		globalDict:       globalDict,
	}
}

// compatBag implements compatibility between destination.* and target.* attributes.
type compatBag struct {
	parent attribute.Bag
}

func (c *compatBag) DebugString() string {
	return c.parent.DebugString()
}

// if a destination.* attribute is missing, check the corresponding target.* attribute.
func (c *compatBag) Get(name string) (v interface{}, found bool) {
	v, found = c.parent.Get(name)
	if found {
		return
	}
	if !strings.HasPrefix(name, "destination.") {
		return
	}
	compatAttr := strings.Replace(name, "destination.", "target.", 1)
	v, found = c.parent.Get(compatAttr)
	if found {
		glog.Warningf("Deprecated attribute %s found", compatAttr)
	}
	return
}

func (c *compatBag) Names() []string {
	return c.parent.Names()
}

func (c *compatBag) Done() {
	c.parent.Done()
}

// Check is the entry point for the external Check method
func (s *grpcServer) Check(legacyCtx legacyContext.Context, req *mixerpb.CheckRequest) (*mixerpb.CheckResponse, error) {
	// TODO: this code doesn't distinguish between RPC failures when communicating with adapters and
	//       their backend vs. semantic failures. For example, if the adapterDispatcher.Check
	//       method returns a bad status, is that because an adapter failed an RPC or because the
	//       request was denied? This will need to be addressed in the new adapter model. In the meantime,
	//       RPC failure is treated as a semantic denial.

	requestBag := attribute.NewProtoBag(&req.Attributes, s.globalDict, s.globalWordList)

	if reqID, found := requestBag.Get("request.id"); found {
		tags.Extract(legacyCtx).Set("request.id", reqID)
	}
	requestBag.ClearReferencedAttributes()

	logger := grpc_zap.Extract(legacyCtx)

	globalWordCount := int(req.GlobalWordCount)

	// compatReqBag ensures that preprocessor input handles deprecated attributes gracefully.
	compatReqBag := &compatBag{requestBag}
	preprocResponseBag := attribute.GetMutableBag(requestBag)
	// compatRespBag ensures that check input handles deprecated attributes gracefully.
	compatRespBag := &compatBag{preprocResponseBag}

	if logger.Core().Enabled(zapcore.InfoLevel) {
		logger.Info("Dispatching Preprocess")
	}

	out := s.aspectDispatcher.Preprocess(legacyCtx, compatReqBag, preprocResponseBag)

	if !status.IsOK(out) {
		if logger.Core().Enabled(zapcore.ErrorLevel) {
			logger.Info("Preprocess Check returned with: " + status.String(out))
		}
		requestBag.Done()
		preprocResponseBag.Done()
		return nil, makeGRPCError(out)
	}
	if logger.Core().Enabled(zapcore.InfoLevel) {
		logger.Info("Preprocess Check returned with: " + status.String(out))
	}

	if logger.Core().Enabled(zapcore.DebugLevel) {
		logger.Debug("Dispatching to main adapters after running processors.", zap.String("attributes", preprocResponseBag.DebugString()))
	}
	dest, _ := compatRespBag.Get("destination.service")
	tags.Extract(legacyCtx).Set("destination.service", dest)
	logger = logger.With(zap.String("destination.service", dest.(string)))

	if logger.Core().Enabled(zapcore.InfoLevel) {
		logger.Info("Dispatching Check")
	}

	cr, err := s.dispatcher.Check(legacyCtx, compatRespBag)
	if err != nil {
		out = status.WithError(err)
	}

	if cr == nil {
		// This request was NOT subject to any checks, let it through.
		cr = checkOk
	} else {
		out = cr.Status
	}

	resp := &mixerpb.CheckResponse{
		Precondition: mixerpb.CheckResponse_PreconditionResult{
			ValidDuration:        cr.ValidDuration,
			ValidUseCount:        cr.ValidUseCount,
			Status:               out,
			ReferencedAttributes: requestBag.GetReferencedAttributes(s.globalDict, globalWordCount),
		},
	}

	if status.IsOK(out) {
		logger.Debug("Check successful", zap.String("handler.response.code", "OK"))
	} else {
		logger.Warn(out.Message, zap.String("handler.response.code", rpc.Code_name[out.Code]), zap.Any("handler.response.details", out.Details))
	}
	requestBag.ClearReferencedAttributes()

	if status.IsOK(resp.Precondition.Status) && len(req.Quotas) > 0 {
		resp.Quotas = make(map[string]mixerpb.CheckResponse_QuotaResult, len(req.Quotas))
		var qr *mixerpb.CheckResponse_QuotaResult

		// TODO: should dispatch this loop in parallel
		// WARNING: if this is dispatched in parallel, then we need to do
		//          use a different protoBag for each individual goroutine
		//          such that we can get valid usage info for individual attributes.
		for name, param := range req.Quotas {
			qma := &aspect.QuotaMethodArgs{
				Quota:           name,
				Amount:          param.Amount,
				DeduplicationID: req.DeduplicationId + name,
				BestEffort:      param.BestEffort,
			}
			var err error

			qr, err = quota(legacyCtx, s.dispatcher, compatRespBag, qma)
			// if quota check fails, set status for the entire request and stop processing.
			if err != nil {
				resp.Precondition.Status = status.WithError(err)
				requestBag.ClearReferencedAttributes()
				break
			}

			// If qma.Quota does not apply to this request give the client what it asked for.
			// Effectively the quota is unlimited.
			if qr == nil {
				qr = &mixerpb.CheckResponse_QuotaResult{
					ValidDuration: defaultValidDuration,
					GrantedAmount: qma.Amount,
				}
			}

			msg := "Quota Allocated"
			if qr.GrantedAmount == 0 {
				msg = "Quota NOT Allocated"
			}
			quotaLogger := logger.Named("quota")
			quotaLogger.Debug(msg, zap.Int64("quota.result.granted", qr.GrantedAmount), zap.Duration("quota.result.validity", qr.ValidDuration))

			qr.ReferencedAttributes = requestBag.GetReferencedAttributes(s.globalDict, globalWordCount)
			resp.Quotas[name] = *qr
			requestBag.ClearReferencedAttributes()
		}
	}

	requestBag.Done()
	preprocResponseBag.Done()

	return resp, nil
}

func quota(legacyCtx legacyContext.Context, d runtime.Dispatcher, bag attribute.Bag,
	qma *aspect.QuotaMethodArgs) (*mixerpb.CheckResponse_QuotaResult, error) {
	if d == nil {
		return nil, nil
	}

	logger := grpc_zap.Extract(legacyCtx)
	logger.Info("Dispatching Quota", zap.String("quota", qma.Quota))
	qmr, err := d.Quota(legacyCtx, bag, qma)
	if err != nil {
		// TODO record the error in the quota specific result
		logger.Warn("Quota Error", zap.String("quota", qma.Quota), zap.Error(err))
		return nil, err
	}

	if qmr == nil { // no quota applied for the given request
		return nil, nil
	}

	logger.Debug("Quota finished", zap.String("quota", qma.Quota), zap.Any("result", qmr))

	return &mixerpb.CheckResponse_QuotaResult{
		GrantedAmount: qmr.Amount,
		ValidDuration: qmr.ValidDuration,
	}, nil
}

var reportResp = &mixerpb.ReportResponse{}

// Report is the entry point for the external Report method
func (s *grpcServer) Report(legacyCtx legacyContext.Context, req *mixerpb.ReportRequest) (*mixerpb.ReportResponse, error) {
	if len(req.Attributes) == 0 {
		// early out
		return reportResp, nil
	}

	// apply the request-level word list to each attribute message if needed
	for i := 0; i < len(req.Attributes); i++ {
		if len(req.Attributes[i].Words) == 0 {
			req.Attributes[i].Words = req.DefaultWords
		}
	}

	protoBag := attribute.NewProtoBag(&req.Attributes[0], s.globalDict, s.globalWordList)
	requestBag := attribute.GetMutableBag(protoBag)
	// compatReqBag ensures that preprocessor input handles deprecated attributes gracefully.
	compatReqBag := &compatBag{requestBag}
	preprocResponseBag := attribute.GetMutableBag(requestBag)
	// compatRespBag ensures that report input handles deprecated attributes gracefully.
	compatRespBag := &compatBag{preprocResponseBag}

	logger := grpc_zap.Extract(legacyCtx)

	var err error
	for i := 0; i < len(req.Attributes); i++ {
		span, newctx := opentracing.StartSpanFromContext(legacyCtx, fmt.Sprintf("Attributes %d", i))

		logger = logger.With(zap.Int("report.id", i))

		// the first attribute block is handled by the protoBag as a foundation,
		// deltas are applied to the child bag (i.e. requestBag)
		if i > 0 {
			err = requestBag.UpdateBagFromProto(&req.Attributes[i], s.globalWordList)
			if err != nil {
				msg := "Request could not be processed due to invalid attributes."
				logger.Error(msg)
				details := status.NewBadRequest("attributes", err)
				err = makeGRPCError(status.InvalidWithDetails(msg, details))
				break
			}
		}

		if reqID, found := requestBag.Get("request.id"); found {
			logger = logger.With(zap.String("request.id", reqID.(string)))
		}

		if logger.Core().Enabled(zapcore.DebugLevel) {
			logger.Info("Dispatching Preprocess")
		}
		out := s.aspectDispatcher.Preprocess(newctx, compatReqBag, preprocResponseBag)
		if !status.IsOK(out) {
			logger.Error(out.Message, zap.String("handler.response.code", rpc.Code_name[out.Code]), zap.Any("handler.response.details", out.Details))
			err = makeGRPCError(out)
			span.LogFields(log.String("error", err.Error()))
			span.Finish()
			break
		}
		if logger.Core().Enabled(zapcore.InfoLevel) {
			logger.Info("Preprocess successful", zap.String("handler.response.code", rpc.Code_name[out.Code]), zap.Any("handler.response.details", out.Details))
		}

		if logger.Core().Enabled(zapcore.DebugLevel) {
			logger.Debug("Dispatching to main adapters after running processors", zap.String("attribute bag", preprocResponseBag.DebugString()))
		}

		if logger.Core().Enabled(zapcore.InfoLevel) {
			logger.Info("Dispatching Report")
		}
		err = s.dispatcher.Report(legacyCtx, compatRespBag)
		if err != nil {
			out = status.WithError(err)
			logger.Warn("Report error", zap.Error(err))
		}

		if !status.IsOK(out) {
			logger.Error(out.Message, zap.String("handler.response.code", rpc.Code_name[out.Code]), zap.Any("handler.response.details", out.Details))
			err = makeGRPCError(out)
			span.LogFields(log.String("error", err.Error()))
			span.Finish()
			break
		}

		logger.Info("Report finished", zap.String("handler.response.code", rpc.Code_name[out.Code]), zap.Any("handler.response.details", out.Details))

		span.LogFields(log.String("success", fmt.Sprintf("finished Report for attribute bag %d", i)))
		span.Finish()
		preprocResponseBag.Reset()
	}

	preprocResponseBag.Done()
	requestBag.Done()
	protoBag.Done()

	if err != nil {
		return nil, err
	}

	return reportResp, nil
}

func makeGRPCError(status rpc.Status) error {
	return grpc.Errorf(codes.Code(status.Code), status.Message)
}
