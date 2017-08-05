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
	"testing"

	context "golang.org/x/net/context"
	"google.golang.org/grpc"

	mixerpb "istio.io/api/mixer/v1"
	"istio.io/api/mixer/v1/attributes"
	"istio.io/mixer/pkg/attribute"
)

func TestCheck(t *testing.T) {
	grpcSrv, attrSrv, addr, err := setup()
	if err != nil {
		t.Fatalf("Could not start local grpc server: %v", err)
	}
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		t.Fatalf("did not connect: %v", err)
	}
	defer func() {
		_ := conn.Close()
		grpcSrv.GracefulStop()
	}()

	client := mixerpb.NewMixerClient(conn)

	attrs := mixerpb.Attributes{
		Words:   []string{"test.attribute", "test.value"},
		Strings: map[int32]int32{-1: -2},
	}

	go func() {
		_, _ = client.Check(context.Background(), &mixerpb.CheckRequest{Attributes: attrs})
	}()

	got := <-attrSrv.Attributes
	want := attribute.CopyBag(attribute.NewProtoBag(&attrs, attrSrv.GlobalDict, attributes.GlobalList()))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Check() => %v; want %v", got, want)
	}
	want.Done()
}

func TestReport(t *testing.T) {
	grpcSrv, attrSrv, addr, err := setup()
	if err != nil {
		t.Fatalf("Could not start local grpc server: %v", err)
	}
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		t.Fatalf("did not connect: %v", err)
	}
	defer func() {
		_ := conn.Close()
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
	}

	go func() {
		_, _ = client.Report(context.Background(), &mixerpb.ReportRequest{Attributes: attrs})
	}()

	for _, src := range attrs {
		got := <-attrSrv.Attributes
		want := attribute.CopyBag(attribute.NewProtoBag(&src, attrSrv.GlobalDict, attributes.GlobalList()))
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Check() => %#v; want %#v", got, want)
		}
		want.Done()
	}
}

func setup() (*grpc.Server, *AttributesServer, string, error) {
	lis, port, err := ListenerAndPort()
	if err != nil {
		return nil, nil, "", fmt.Errorf("Could not find suitable listener: %v", err)
	}

	attrSrv := NewAttributesServer(make(chan attribute.Bag))
	grpcSrv := NewMixerServer(attrSrv)

	go func() {
		_ = grpcSrv.Serve(lis)
	}()

	return grpcSrv, attrSrv, fmt.Sprintf("localhost:%d", port), nil
}
