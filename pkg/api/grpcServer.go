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
	"context"
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	rpc "github.com/googleapis/googleapis/google/rpc"
	context2 "golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	mixerpb "istio.io/api/mixer/v1"
	"istio.io/mixer/pkg/adapterManager"
	"istio.io/mixer/pkg/aspect"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/pool"
	"istio.io/mixer/pkg/status"
)

type (
	// grpcServer holds the dispatchState for the gRPC API server.
	grpcServer struct {
		aspectDispatcher adapterManager.AspectDispatcher
		gp               *pool.GoroutinePool

		// replaceable sendMsg so we can inject errors in tests
		sendMsg func(grpc.Stream, proto.Message) error

		words []string
	}

	// dispatchState holds the set of information used for dispatch and
	// request handling.
	dispatchState struct {
		request, response proto.Message
		inAttrs, outAttrs *mixerpb.Attributes
		result            *rpc.Status
	}

	// dispatchArgs holds the set of information passed into dispatchFns.
	// These args are derived from the dispatchState.
	dispatchArgs struct {
		request, response       proto.Message
		requestBag, responseBag *attribute.MutableBag
	}

	dispatchFn func(ctx context.Context, args dispatchArgs) rpc.Status
)

// NewGRPCServer creates a gRPC serving stack.
func NewGRPCServer(aspectDispatcher adapterManager.AspectDispatcher, gp *pool.GoroutinePool) mixerpb.MixerServer {
	return &grpcServer{
		aspectDispatcher: aspectDispatcher,
		gp:               gp,
		sendMsg: func(stream grpc.Stream, m proto.Message) error {
			return stream.SendMsg(m)
		},
	}
}

// dispatch implements all the nitty-gritty details of handling the mixer's low-level API
// protocol and dispatching to the right API dispatchWrapperFn.
func (s *grpcServer) dispatch(ctx context.Context, dState *dispatchState, worker dispatchFn) error {
	protoBag, err := attribute.GetBagFromProto(dState.inAttrs, s.words)
	if err != nil {
		msg := "Request could not be processed due to invalid 'attributes'."
		glog.Error(msg, "\n", err)
		details := status.NewBadRequest("attributes", err)
		out := status.InvalidWithDetails(msg, details)
		return makeGRPCError(out)
	}

	requestBag := attribute.GetMutableBag(protoBag)
	responseBag := attribute.GetMutableBag(nil)

	// do the actual work for the message
	args := dispatchArgs{
		dState.request,
		dState.response,
		requestBag,
		responseBag,
	}

	out := worker(ctx, args)

	if dState.outAttrs != nil {
		// TODO		responseBag.ToProto(dState.outAttrs, s.words)
	}

	requestBag.Done()
	responseBag.Done()
	protoBag.Done()

	return makeGRPCError(out)
}

func makeGRPCError(status rpc.Status) error {
	return grpc.Errorf(codes.Code(status.Code), status.Message)
}

// Check is the entry point for the external Check2 method
func (s *grpcServer) Check(ctx context2.Context, req *mixerpb.CheckRequest) (*mixerpb.CheckResponse, error) {
	resp := &mixerpb.CheckResponse{}
	var result rpc.Status

	dState := dispatchState{
		request:  req,
		response: resp,
		inAttrs:  &req.Attributes,
		outAttrs: &resp.Attributes,
		result:   &result,
	}

	err := s.dispatch(ctx, &dState, s.preprocess(s.handleCheck))
	return resp, err
}

// Report is the entry point for the external Report2 method
func (s *grpcServer) Report(ctx context2.Context, req *mixerpb.ReportRequest) (*mixerpb.ReportResponse, error) {
	resp := &mixerpb.ReportResponse{}
	var result rpc.Status

	for i := 0; i < len(req.Attributes); i++ {
		if len(req.Attributes[i].Words) == 0 {
			req.Attributes[i].Words = req.DefaultWords
		}

		dState := dispatchState{
			request:  req,
			response: resp,
			inAttrs:  &req.Attributes[i],
			outAttrs: nil,
			result:   &result,
		}

		err := s.dispatch(ctx, &dState, s.preprocess(s.handleReport))
		if err != nil {
			return resp, err
		}
	}

	return resp, nil
}

// Quota is the entry point for the external Quota2 method
func (s *grpcServer) Quota(ctx context2.Context, req *mixerpb.QuotaRequest) (*mixerpb.QuotaResponse, error) {
	resp := &mixerpb.QuotaResponse{}
	var result rpc.Status

	dState := dispatchState{
		request:  req,
		response: resp,
		inAttrs:  &req.Attributes,
		outAttrs: nil,
		result:   &result,
	}

	err := s.dispatch(ctx, &dState, s.preprocess(s.handleQuota))
	return resp, err
}

func (s *grpcServer) handleCheck(ctx context.Context, args dispatchArgs) rpc.Status {
	resp := args.response.(*mixerpb.CheckResponse)

	glog.Info("Dispatching Check")

	out := s.aspectDispatcher.Check(ctx, args.requestBag, args.responseBag)

	// TODO: this value needs to initially come from config, and be modulated by the kind of attribute
	//       that was used in the check and the in-used aspects (for example, maybe an auth check has a
	//       30s TTL but a whitelist check has got a 120s TTL)
	resp.Expiration = 5 * time.Second

	glog.Info("Check returned with: ", statusString(out))
	return out
}

func (s *grpcServer) handleReport(ctx context.Context, args dispatchArgs) rpc.Status {
	glog.Info("Dispatching Report")
	out := s.aspectDispatcher.Report(ctx, args.requestBag, args.responseBag)
	glog.Info("Report returned with: ", statusString(out))
	return out
}

func (s *grpcServer) handleQuota(ctx context.Context, args dispatchArgs) rpc.Status {
	req := args.request.(*mixerpb.QuotaRequest)
	resp := args.response.(*mixerpb.QuotaResponse)

	qma := &aspect.QuotaMethodArgs{
		Quota:           req.Quota,
		Amount:          req.Amount,
		DeduplicationID: req.DeduplicationId,
		BestEffort:      req.BestEffort,
	}

	glog.Info("Dispatching Report")
	qmr, out := s.aspectDispatcher.Quota(ctx, args.requestBag, args.responseBag, qma)

	if qmr != nil {
		resp.Amount = qmr.Amount
		resp.Expiration = qmr.Expiration
	}
	glog.Infof("Report returned with status '%v' and quota response '%v'", statusString(out), qmr)
	return out
}

func (s *grpcServer) preprocess(dispatch dispatchFn) dispatchFn {
	return func(ctx context.Context, args dispatchArgs) rpc.Status {

		out := s.aspectDispatcher.Preprocess(ctx, args.requestBag, args.responseBag)

		if !status.IsOK(out) {
			return out
		}

		if err := args.requestBag.Merge(args.responseBag); err != nil {
			// TODO: better error messages that push internal details into debuginfo messages
			glog.Errorf("Could not merge mutable bags for request: %v", err)
			return status.WithInternal("The results from the request preprocessing could not be merged.")
		}

		return dispatch(ctx, args)
	}
}

func statusString(status rpc.Status) string {
	var ok bool
	var name string
	if name, ok = rpc.Code_name[status.Code]; !ok {
		name = rpc.Code_name[int32(rpc.UNKNOWN)]
	}
	return fmt.Sprintf("%s %s", name, status.Message)
}
