package test

import (
	"log"
	"os"
	"reflect"
	"testing"

	context "golang.org/x/net/context"
	"google.golang.org/grpc"

	mixerpb "istio.io/api/mixer/v1"
)

var attrSrv *AttributesServer
var port string

func TestCheck(t *testing.T) {

	addr := "localhost" + port
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		t.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	client := mixerpb.NewMixerClient(conn)

	want := mixerpb.Attributes{
		Words:   []string{"test.attribute", "test.value"},
		Strings: map[int32]int32{-1: -2},
	}

	t.Run("Check", func(t *testing.T) {
		resp, err := client.Check(context.Background(), &mixerpb.CheckRequest{Attributes: want})
		if err != nil {
			t.Fatalf("Failure sending check: %v", err)
		}

		if got := resp.Precondition.Attributes; !reflect.DeepEqual(got, want) {
			t.Fatalf("Check() => %#v; want %#v", got, want)
		}
	})
}

func TestReport(t *testing.T) {
	addr := "localhost" + port
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		t.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	client := mixerpb.NewMixerClient(conn)

	want := []mixerpb.Attributes{
		{
			Words:   []string{"report.attribute", "report.value"},
			Strings: map[int32]int32{-1: -2},
		},
		{
			Words:   []string{"another.attribute", "another.value"},
			Strings: map[int32]int32{-1: -2},
		},
	}

	t.Run("Report", func(t *testing.T) {
		_, err := client.Report(context.Background(), &mixerpb.ReportRequest{Attributes: want})
		if err != nil {
			t.Fatalf("Failure sending check: %v", err)
		}

		if got := attrSrv.Received; !reflect.DeepEqual(got, want) {
			t.Fatalf("Report() => %#v; want %#v", got, want)
		}
	})
}

func TestMain(m *testing.M) {
	lis, availPort, err := Listener()
	if err != nil {
		log.Fatalf("Could not find suitable listener: %v", err)
	}
	port = availPort

	var grpcSrv *grpc.Server
	grpcSrv, attrSrv = NewMixerAttributesServer()

	go RunServer(lis, grpcSrv)
	os.Exit(m.Run())
}
