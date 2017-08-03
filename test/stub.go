package test

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strings"

	rpc "github.com/googleapis/googleapis/google/rpc"
	context "golang.org/x/net/context"
	"google.golang.org/grpc"

	mixerpb "istio.io/api/mixer/v1"
)

type AttributesServer struct {
	mixerpb.MixerServer

	Received []mixerpb.Attributes
}

func (a *AttributesServer) Check(ctx context.Context, req *mixerpb.CheckRequest) (*mixerpb.CheckResponse, error) {
	a.Received = []mixerpb.Attributes{req.Attributes}
	resp := &mixerpb.CheckResponse{
		Precondition: mixerpb.CheckResponse_PreconditionResult{
			Status:        rpc.Status{Code: int32(rpc.OK)},
			ValidUseCount: 1,
			Attributes:    req.Attributes,
		},
	}
	return resp, nil
}

func (a *AttributesServer) Report(ctx context.Context, req *mixerpb.ReportRequest) (*mixerpb.ReportResponse, error) {
	a.Received = req.Attributes
	return &mixerpb.ReportResponse{}, nil
}

func NewMixerAttributesServer() (*grpc.Server, *AttributesServer) {
	as := &AttributesServer{Received: []mixerpb.Attributes{}}
	return NewMixerGRPCServer(as), as
}

func NewMixerGRPCServer(impl mixerpb.MixerServer) *grpc.Server {
	gs := grpc.NewServer()
	mixerpb.RegisterMixerServer(gs, impl)
	return gs
}

func Listener() (net.Listener, string, error) {
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, "", fmt.Errorf("Could not find open port for server: %v", err)
	}

	addr := lis.Addr().String()
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return lis, addr[idx:], nil
	}

	// error condition: close listener abort
	lis.Close()
	return nil, "", errors.New("Could not find port from listener address")
}

func RunServer(lis net.Listener, s *grpc.Server) {
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Could not serve GRCP traffic: %v", err)
	}
}
