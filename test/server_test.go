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

package test

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	rpc "github.com/googleapis/googleapis/google/rpc"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	mixerpb "istio.io/api/mixer/v1"
	"istio.io/mixer/pkg/attribute"
)

var (
	ok      = rpc.Status{Code: int32(rpc.OK)}
	invalid = rpc.Status{Code: int32(rpc.INVALID_ARGUMENT)}

	attrs = mixerpb.Attributes{
		Words:   []string{"test.attribute", "test.value"},
		Strings: map[int32]int32{-1: -2},
	}

	emptyRefAttrs = mixerpb.ReferencedAttributes{}

	refAttrs = mixerpb.ReferencedAttributes{
		Words: []string{"test", "attributes"},
		AttributeMatches: []mixerpb.ReferencedAttributes_AttributeMatch{
			{4, mixerpb.EXACT, ""},
			{3, mixerpb.ABSENCE, ""},
		},
	}

	testQuotas = map[string]mixerpb.CheckRequest_QuotaParams{
		"foo": {Amount: 55, BestEffort: false},
		"bar": {Amount: 21, BestEffort: true},
	}

	quotaOverrides = map[string]mixerpb.CheckResponse_QuotaResult{
		"foo": {ValidDuration: 3 * time.Second, GrantedAmount: 55, ReferencedAttributes: emptyRefAttrs},
		"bar": {ValidDuration: 15 * time.Second, GrantedAmount: 7, ReferencedAttributes: refAttrs},
	}
)

type attributeServerFn func(server *AttributesServer)

func noop(s *AttributesServer) {}

func setGRPCErr(s *AttributesServer) {
	s.GenerateGRPCError = true
}

func clearGRPCErr(s *AttributesServer) {
	s.GenerateGRPCError = false
}

func setCheckResponse(s *AttributesServer) {
	s.CheckResponse = &mixerpb.CheckResponse{
		Precondition: precondition(invalid, attrs, refAttrs),
		Quotas:       quotaOverrides,
	}
}

func clearCheckResponse(s *AttributesServer) {
	s.CheckResponse = nil
}

func TestCheck(t *testing.T) {
	grpcSrv, attrSrv, addr, err := setup()
	if err != nil {
		t.Fatalf("Could not start local grpc server: %v", err)
	}
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() {
		err = conn.Close()
		grpcSrv.GracefulStop()
	}()

	client := mixerpb.NewMixerClient(conn)

	attrBag := attribute.CopyBag(attribute.NewProtoBag(&attrs, attrSrv.GlobalDict, attribute.GlobalList()))

	noQuotaReq := &mixerpb.CheckRequest{Attributes: attrs}
	quotaReq := &mixerpb.CheckRequest{Attributes: attrs, Quotas: testQuotas, DeduplicationId: "baz"}
	okCheckResp := &mixerpb.CheckResponse{Precondition: precondition(ok, mixerpb.Attributes{}, emptyRefAttrs)}
	customResp := &mixerpb.CheckResponse{
		Precondition: precondition(invalid, attrs, refAttrs),
		Quotas: map[string]mixerpb.CheckResponse_QuotaResult{
			"foo": {ValidDuration: 3 * time.Second, GrantedAmount: 55, ReferencedAttributes: emptyRefAttrs},
			"bar": {ValidDuration: 15 * time.Second, GrantedAmount: 7, ReferencedAttributes: refAttrs},
		},
	}
	quotaDispatches := []QuotaDispatchInfo{
		{Attributes: attrBag, MethodArgs: QuotaMethodArgs{"bazfoo", "foo", 55, false}},
		{Attributes: attrBag, MethodArgs: QuotaMethodArgs{"bazbar", "bar", 21, true}},
	}

	cases := []struct {
		name           string
		req            *mixerpb.CheckRequest
		setupFn        attributeServerFn
		teardownFn     attributeServerFn
		wantCallErr    bool
		wantResponse   *mixerpb.CheckResponse
		wantAttributes attribute.Bag
		wantDispatches []QuotaDispatchInfo
	}{
		{"basic", noQuotaReq, noop, noop, false, okCheckResp, attrBag, nil},
		{"grpc err", noQuotaReq, setGRPCErr, clearGRPCErr, true, okCheckResp, nil, nil},
		{"check response", quotaReq, setCheckResponse, clearCheckResponse, false, customResp, attrBag, quotaDispatches},
	}

	for _, v := range cases {
		t.Run(v.name, func(t *testing.T) {
			v.setupFn(attrSrv)

			var wg sync.WaitGroup
			wg.Add(1)
			go func(req *mixerpb.CheckRequest, wantErr bool, wantResp *mixerpb.CheckResponse) {
				defer wg.Done()
				resp, err := client.Check(context.Background(), req)
				if wantErr {
					if err == nil {
						t.Error("No error in Check() call")
					}
					return
				}
				if err != nil {
					t.Errorf("Unexpected error in Check() call: %v", err)
					return
				}
				if !proto.Equal(resp, wantResp) {
					t.Errorf("Check() => %#v, \n\n wanted: %#v", resp, wantResp)
				}
			}(v.req, v.wantCallErr, v.wantResponse)

			if v.wantAttributes != nil {
				select {
				case got := <-attrSrv.CheckAttributes:
					if !reflect.DeepEqual(got, v.wantAttributes) {
						t.Errorf("Check() => %v; want %v", got, v.wantAttributes)
					}
				case <-time.After(500 * time.Millisecond):
					t.Error("Check() => timed out waiting for attributes")
				}
			}

			// we have no control over order of emitted dispatches
			holder := make(map[string]QuotaDispatchInfo, len(v.wantDispatches))
			for range v.wantDispatches {
				select {
				case got := <-attrSrv.QuotaDispatches:
					holder[got.MethodArgs.DeduplicationID] = got
				case <-time.After(500 * time.Millisecond):
					t.Error("Check() => timed out waiting for quota dispatches")
				}
			}

			for _, dispatch := range v.wantDispatches {
				got, found := holder[dispatch.MethodArgs.DeduplicationID]
				if !found {
					t.Errorf("No matching Quota dispatch found for: %v", dispatch)
					continue
				}
				if !reflect.DeepEqual(got, dispatch) {
					t.Errorf("Check() => %v; want %v", got, dispatch)
				}
			}

			wg.Wait()

			v.teardownFn(attrSrv)
		})
	}
}

func TestReport(t *testing.T) {
	grpcSrv, attrSrv, addr, err := setup()
	if err != nil {
		t.Fatalf("Could not start local grpc server: %v", err)
	}
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() {
		err = conn.Close()
		grpcSrv.GracefulStop()
	}()

	client := mixerpb.NewMixerClient(conn)

	attrs := []mixerpb.Attributes{
		{
			Words:   []string{"some_user"},
			Strings: map[int32]int32{6: -1}, // 6 is global index for "source_user"
		},
		{
			Words:   []string{"updated_user", "another.attribute", "another.value"},
			Strings: map[int32]int32{6: -1, -2: -3}, // 6 is global index for "source_user"
		},
		{
			Strings: map[int32]int32{6: -1, -2: -3}, // 6 is global index for "source_user"
		},
	}

	words := []string{"foo", "bar", "baz"}

	baseBag := attribute.CopyBag(attribute.NewProtoBag(&attrs[0], attrSrv.GlobalDict, attribute.GlobalList()))
	middleBag := attribute.CopyBag(baseBag)
	if err = middleBag.UpdateBagFromProto(&attrs[1], attribute.GlobalList()); err != nil {
		t.Fatalf("Could not set up attribute bags for testing: %v", err)
	}

	finalAttr := &mixerpb.Attributes{Words: words, Strings: attrs[2].Strings}
	finalBag := attribute.CopyBag(middleBag)
	if err = finalBag.UpdateBagFromProto(finalAttr, attribute.GlobalList()); err != nil {
		t.Fatalf("Could not set up attribute bags for testing: %v", err)
	}

	attrBags := []attribute.Bag{baseBag, middleBag, finalBag}

	cases := []struct {
		name           string
		req            *mixerpb.ReportRequest
		setupFn        attributeServerFn
		teardownFn     attributeServerFn
		wantCallErr    bool
		wantAttributes []attribute.Bag
	}{
		{"basic", &mixerpb.ReportRequest{Attributes: attrs, DefaultWords: words}, noop, noop, false, attrBags},
		{"grpc err", &mixerpb.ReportRequest{Attributes: attrs, DefaultWords: words}, setGRPCErr, clearGRPCErr, true, nil},
		{"no attributes", &mixerpb.ReportRequest{}, noop, noop, false, []attribute.Bag{}},
	}

	for _, v := range cases {
		t.Run(v.name, func(t *testing.T) {

			v.setupFn(attrSrv)

			var wg sync.WaitGroup
			wg.Add(1)
			go func(req *mixerpb.ReportRequest, wantErr bool) {
				defer wg.Done()
				_, err := client.Report(context.Background(), req)
				if err == nil && wantErr {
					t.Error("No error in Report() call")
					return
				}
				if err != nil && !wantErr {
					t.Errorf("Unexpected error in Report() call: %v", err)
				}
			}(v.req, v.wantCallErr)

			for _, want := range v.wantAttributes {
				select {
				case got := <-attrSrv.ReportAttributes:
					if !reflect.DeepEqual(got, want) {
						t.Errorf("Report() => %#v; want %#v", got, want)
					}
				case <-time.After(500 * time.Millisecond):
					t.Error("Report() => timed out waiting for report attributes")
				}
			}

			wg.Wait()

			v.teardownFn(attrSrv)
		})
	}
}

func setup() (*grpc.Server, *AttributesServer, string, error) {
	lis, port, err := ListenerAndPort()
	if err != nil {
		return nil, nil, "", fmt.Errorf("could not find suitable listener: %v", err)
	}

	attrSrv := NewAttributesServer()
	grpcSrv := NewMixerServer(attrSrv)

	go func() {
		_ = grpcSrv.Serve(lis)
	}()

	return grpcSrv, attrSrv, fmt.Sprintf("localhost:%d", port), nil
}

func precondition(status rpc.Status, attrs mixerpb.Attributes, refAttrs mixerpb.ReferencedAttributes) mixerpb.CheckResponse_PreconditionResult {
	return mixerpb.CheckResponse_PreconditionResult{
		Status:               status,
		ValidUseCount:        DefaultValidUseCount,
		Attributes:           attrs,
		ReferencedAttributes: refAttrs,
	}
}
