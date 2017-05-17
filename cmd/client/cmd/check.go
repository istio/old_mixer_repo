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

package cmd

import (
	"context"
	"io"
	"time"

	"github.com/opentracing/opentracing-go/ext"
	"github.com/spf13/cobra"

	mixerpb "istio.io/api/mixer/v1"
	"istio.io/mixer/cmd/shared"
)

func checkCmd(rootArgs *rootArgs, printf, fatalf shared.FormatFn) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Invokes Mixer's Check API to perform precondition checks.",
		Long: "The Check method is used to perform precondition checks. Mixer\n" +
			"expects a set of attributes as input, which it uses, along with\n" +
			"its configuration, to determine which adapters to invoke and with\n" +
			"which parameters in order to perform the precondition check.",

		Run: func(cmd *cobra.Command, args []string) {
			check(rootArgs, printf, fatalf)
		}}
}

func check(rootArgs *rootArgs, printf, fatalf shared.FormatFn) {
	var attrs *mixerpb.Attributes
	var err error

	if attrs, err = parseAttributes(rootArgs); err != nil {
		fatalf("%v", err)
	}

	var cs *clientState
	if cs, err = createAPIClient(rootArgs.mixerAddress, rootArgs.enableTracing); err != nil {
		fatalf("Unable to establish connection to %s", rootArgs.mixerAddress)
	}
	defer deleteAPIClient(cs)

	// TODO: one span for each request - not sure what we can get from the per-stream span.
	span, ctx := cs.tracer.StartRootSpan(context.Background(), "mixc Check", ext.SpanKindRPCClient)
	_, ctx = cs.tracer.PropagateSpan(ctx, span)

	t := time.Now()

	sem := make(chan bool, rootArgs.concurrency)

	// start the stream
	var stream mixerpb.Mixer_CheckClient
	if stream, err = cs.client.Check(ctx); err != nil {
		fatalf("Check RPC failed: %v", err)
	}
	// send requests in background, using the sem channel to limit the concurrency
	// TODO: use a timer, like wrk
	go func() {
		for i := 0; i < rootArgs.repeat; i++ {
			request := mixerpb.CheckRequest{RequestIndex: int64(i), AttributeUpdate: *attrs}

			if err = stream.Send(&request); err != nil {
				fatalf("Failed to send Check RPC: %v", err)
			}
			sem <- true
		}
	}()

	// Read responses.
	for i := 0; i < rootArgs.repeat; i++ {
		var response *mixerpb.CheckResponse
		response, err = stream.Recv()
		if err == io.EOF {
			fatalf("Got no response from Check RPC")
		} else if err != nil {
			fatalf("Failed to receive a response from Check RPC: %v", err)
		}

		<-sem

		// To limit verbosity, only show responses for small repeat counts (otherwise logging slows down the test)
		if rootArgs.repeat < 10 {
			printf("Check RPC returned %s", decodeStatus(response.Result))
			dumpAttributes(printf, fatalf, response.AttributeUpdate)
		}
	}
	if err = stream.CloseSend(); err != nil {
		fatalf("Failed to close gRPC stream: %v", err)
	}

	println("Done in ", time.Since(t), " avg(ms)=", (time.Since(t).Nanoseconds()/1000)/int64(rootArgs.repeat))
	span.Finish()
}
