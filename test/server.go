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
	"fmt"
	"net"

	rpc "github.com/googleapis/googleapis/google/rpc"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	mixerpb "istio.io/api/mixer/v1"
	"istio.io/api/mixer/v1/attributes"
	"istio.io/mixer/pkg/attribute"
)

// AttributesServer implements the Mixer API to send mutable attributes bags to
// a channel upon API requests. This can be used for tests that want to exercise
// the Mixer API and validate server handling of supplied attributes.
type AttributesServer struct {
	GlobalDict map[string]int32
	Attributes chan attribute.Bag
}

// NewAttributesServer creates an AttributesServer with the channel provided. If
// a nil channel is provided, one will be created with default len.
func NewAttributesServer(c chan attribute.Bag) *AttributesServer {
	list := attributes.GlobalList()
	globalDict := make(map[string]int32, len(list))
	for i := 0; i < len(list); i++ {
		globalDict[list[i]] = int32(i)
	}
	if c == nil {
		return &AttributesServer{globalDict, make(chan attribute.Bag)}
	}
	return &AttributesServer{globalDict, c}
}

// Check sends a copy of the protocol buffers attributes wrapper to the server
// channel and returns an OK response.
func (a *AttributesServer) Check(ctx context.Context, req *mixerpb.CheckRequest) (*mixerpb.CheckResponse, error) {

	requestBag := attribute.NewProtoBag(&req.Attributes, a.GlobalDict, attributes.GlobalList())
	defer requestBag.Done()
	a.Attributes <- attribute.CopyBag(requestBag)

	resp := &mixerpb.CheckResponse{
		Precondition: mixerpb.CheckResponse_PreconditionResult{
			Status:        rpc.Status{Code: int32(rpc.OK)},
			ValidUseCount: 1,
			Attributes:    req.Attributes,
		},
	}
	return resp, nil
}

// Report iterates through the supplied attributes sets, applying the deltas
// appropriately, and sending the generated bags to the channel.
func (a *AttributesServer) Report(ctx context.Context, req *mixerpb.ReportRequest) (*mixerpb.ReportResponse, error) {

	if len(req.Attributes) == 0 {
		// early out
		return &mixerpb.ReportResponse{}, nil
	}

	// apply the request-level word list to each attribute message if needed
	for i := 0; i < len(req.Attributes); i++ {
		if len(req.Attributes[i].Words) == 0 {
			req.Attributes[i].Words = req.DefaultWords
		}
	}

	protoBag := attribute.NewProtoBag(&req.Attributes[0], a.GlobalDict, attributes.GlobalList())
	requestBag := attribute.GetMutableBag(protoBag)
	defer requestBag.Done()
	defer protoBag.Done()

	a.Attributes <- attribute.CopyBag(requestBag)

	for i := 1; i < len(req.Attributes); i++ {
		// the first attribute block is handled by the protoBag as a foundation,
		// deltas are applied to the child bag (i.e. requestBag)
		if err := requestBag.UpdateBagFromProto(&req.Attributes[i], attributes.GlobalList()); err != nil {
			return &mixerpb.ReportResponse{}, fmt.Errorf("could not apply attribute delta: %v", err)
		}
		a.Attributes <- attribute.CopyBag(requestBag)
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
