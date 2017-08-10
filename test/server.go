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

// Package test supplies a fake Mixer server for use in testing. It should NOT
// be used outside of testing contexts.
package test // import "istio.io/mixer/test"

import (
	"errors"
	"fmt"
	"net"
	"time"

	rpc "github.com/googleapis/googleapis/google/rpc"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	mixerpb "istio.io/api/mixer/v1"
	"istio.io/mixer/pkg/attribute"
)

var DefaultValidUseCount = int32(1)
var DefaultValidDuration = 1 * time.Second

// QuotaDispatchInfo contains both the attribute bag generated by the server
// for Quota dispatch, as well as the corresponding method arguments.
type QuotaDispatchInfo struct {
	Attributes attribute.Bag
	MethodArgs QuotaMethodArgs
}

// QuotaMethodArgs mirrors aspect.QuotaMethodArgs. It allows tests to understand
// how their inputs map into quota calls within Mixer to validate the proper
// requests are being generated. It is NOT a contract for how a real Mixer would
// generate or dispatch Quota calls.
type QuotaMethodArgs struct {
	// Unique ID for Quota operation.
	DeduplicationID string

	// The quota to allocate from.
	Quota string

	// The amount of quota to allocate.
	Amount int64

	// If true, allows a response to return less quota than requested. When
	// false, the exact requested amount is returned or 0 if not enough quota
	// was available.
	BestEffort bool
}

// AttributesServer implements the Mixer API to send mutable attributes bags to
// a channel upon API requests. This can be used for tests that want to exercise
// the Mixer API and validate server handling of supplied attributes.
type AttributesServer struct {
	// GlobalDict controls the known global dictionary for attribute processing.
	GlobalDict map[string]int32

	// GenerateGRPCError instructs the server whether or not to fail-fast with
	// an error that will manifest as a GRPC error.
	GenerateGRPCError bool

	// CheckResponse controls the contents of a response from the server for
	// Check() calls.
	CheckResponse *mixerpb.CheckResponse

	// PublishToChannels controls whether or not to send attribute bags and
	// quota dispatch args to various channels.
	PublishToChannels bool

	// CheckAttributes is the channel on which the attribute bag generated by
	// the server for Check() requests is sent.
	CheckAttributes chan attribute.Bag

	// QuotaDispatches is the channel on which the quota information generated by
	// the server for Check() requests is sent.
	QuotaDispatches chan QuotaDispatchInfo

	// ReportAttributes is the channel on which the attribute bag generated by
	// the server for Report() requests is sent.
	ReportAttributes chan attribute.Bag
}

// NewAttributesServer creates an AttributesServer. All channels are set to
// default length.
func NewAttributesServer() *AttributesServer {
	list := attribute.GlobalList()
	globalDict := make(map[string]int32, len(list))
	for i := 0; i < len(list); i++ {
		globalDict[list[i]] = int32(i)
	}

	return &AttributesServer{
		globalDict,
		false,
		nil,
		true,
		make(chan attribute.Bag),
		make(chan QuotaDispatchInfo),
		make(chan attribute.Bag),
	}

}

// Check sends a copy of the protocol buffers attributes wrapper for the preconditions
// check as well as for each quotas check to the CheckAttributes channel. It also
// builds a CheckResponse based on server fields. All channel sends timeout to
// prevent problematic tests from blocking indefinitely.
func (a *AttributesServer) Check(ctx context.Context, req *mixerpb.CheckRequest) (*mixerpb.CheckResponse, error) {

	if a.GenerateGRPCError {
		return nil, errors.New("error handling check call")
	}

	requestBag := attribute.NewProtoBag(&req.Attributes, a.GlobalDict, attribute.GlobalList())
	defer requestBag.Done()

	if a.PublishToChannels {
		// avoid blocked go-routines in testing
		select {
		case a.CheckAttributes <- attribute.CopyBag(requestBag): // no-op
		case <-time.After(3 * time.Second):
			return nil, errors.New("timeout trying to send attribute bag")
		}
	}

	resp := &mixerpb.CheckResponse{
		Precondition: mixerpb.CheckResponse_PreconditionResult{
			Status:               rpc.Status{Code: int32(rpc.OK)},
			ValidUseCount:        DefaultValidUseCount,
			Attributes:           mixerpb.Attributes{},
			ReferencedAttributes: mixerpb.ReferencedAttributes{},
		},
	}

	if len(req.Quotas) > 0 {
		resp.Quotas = make(map[string]mixerpb.CheckResponse_QuotaResult, len(req.Quotas))
		for name, param := range req.Quotas {
			qma := QuotaMethodArgs{
				Quota:           name,
				Amount:          param.Amount,
				DeduplicationID: req.DeduplicationId + name,
				BestEffort:      param.BestEffort,
			}

			if a.PublishToChannels {
				// avoid blocked go-routines in testing
				select {
				case a.QuotaDispatches <- QuotaDispatchInfo{attribute.CopyBag(requestBag), qma}: //no-op
				case <-time.After(3 * time.Second):
					return nil, errors.New("timeout trying to send quota dispatch")
				}
			}

			qr := mixerpb.CheckResponse_QuotaResult{
				GrantedAmount:        param.Amount,
				ValidDuration:        DefaultValidDuration,
				ReferencedAttributes: mixerpb.ReferencedAttributes{},
			}
			resp.Quotas[name] = qr
		}
	}

	// override response if requested
	if a.CheckResponse != nil {
		resp = a.CheckResponse
	}

	return resp, nil
}

// Report iterates through the supplied attributes sets, applying the deltas
// appropriately, and sending the generated bags to the channel.
func (a *AttributesServer) Report(ctx context.Context, req *mixerpb.ReportRequest) (*mixerpb.ReportResponse, error) {

	if a.GenerateGRPCError {
		return nil, errors.New("error handling report call")
	}

	if len(req.Attributes) == 0 {
		// early out
		return &mixerpb.ReportResponse{}, nil
	}

	if a.PublishToChannels {
		// apply the request-level word list to each attribute message if needed
		for i := 0; i < len(req.Attributes); i++ {
			if len(req.Attributes[i].Words) == 0 {
				req.Attributes[i].Words = req.DefaultWords
			}
		}
	}

	protoBag := attribute.NewProtoBag(&req.Attributes[0], a.GlobalDict, attribute.GlobalList())
	requestBag := attribute.GetMutableBag(protoBag)
	defer requestBag.Done()
	defer protoBag.Done()

	// avoid blocked go-routines in testing
	select {
	case a.ReportAttributes <- attribute.CopyBag(requestBag): // no-op
	case <-time.After(3 * time.Second):
		return nil, errors.New("timeout trying to send attribute bag")
	}

	for i := 1; i < len(req.Attributes); i++ {
		// the first attribute block is handled by the protoBag as a foundation,
		// deltas are applied to the child bag (i.e. requestBag)
		if err := requestBag.UpdateBagFromProto(&req.Attributes[i], attribute.GlobalList()); err != nil {
			return &mixerpb.ReportResponse{}, fmt.Errorf("could not apply attribute delta: %v", err)
		}
		if a.PublishToChannels {
			// avoid blocked go-routines in testing
			select {
			case a.ReportAttributes <- attribute.CopyBag(requestBag): // no-op
			case <-time.After(3 * time.Second):
				return nil, errors.New("timeout trying to send attribute bag")
			}
		}
	}

	return &mixerpb.ReportResponse{}, nil
}

// NewMixerServer creates a new grpc.Server with the supplied implementation
// of the Mixer API.
func NewMixerServer(impl mixerpb.MixerServer) *grpc.Server {
	gs := grpc.NewServer()
	mixerpb.RegisterMixerServer(gs, impl)
	return gs
}

// ListenerAndPort starts a listener on an available port and returns both the
// listener and the port on which it is listening.
func ListenerAndPort() (net.Listener, int, error) {
	lis, err := net.Listen("tcp", ":0") // nolint: gas
	if err != nil {
		return nil, 0, fmt.Errorf("could not find open port for server: %v", err)
	}
	return lis, lis.Addr().(*net.TCPAddr).Port, nil
}