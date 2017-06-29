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
	"time"

	ot "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/spf13/cobra"

	"sync"

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

	// TODO: one span for each request - not sure what we can get from the per-stream span.

	t := time.Now()

	var wg sync.WaitGroup
	wg.Add(rootArgs.clients)

	for s := 0; s < rootArgs.clients; s++ {
		go func() {
			sem := make(chan bool, rootArgs.concurrency)

			var cs *clientState
			if cs, err = createAPIClient(rootArgs.mixerAddress, rootArgs.enableTracing); err != nil {
				fatalf("Unable to establish connection to %s", rootArgs.mixerAddress)
			}
			defer deleteAPIClient(cs)
			span, ctx := ot.StartSpanFromContext(context.Background(), "mixc Check", ext.SpanKindRPCClient)

			// send requests in background, using the sem channel to limit the concurrency
			// TODO: use a timer, like wrk
			go func() {
				for i := 0; i < rootArgs.repeat; i++ {
					<-sem
					request := mixerpb.CheckRequest{RequestIndex: int64(i), AttributeUpdate: *attrs}

					response, err := cs.client.Check(ctx, &request)
					if err != nil {
						fatalf("Failed to send Check RPC: %v", err)
					}
					if rootArgs.repeat < 10 {
						printf("Check RPC returned %s", decodeStatus(response.Result))
						dumpAttributes(printf, fatalf, response.AttributeUpdate)
					}
					// Some progress indication, 100k seems to avoid verbosity.
					if i%100000 == 0 {
						print(".")
					}
					sem <- true
				}
			}()

			span.Finish()
			wg.Done()
		}()
	}
	wg.Wait()
	d := time.Since(t)

	totalRequests := int64(rootArgs.repeat * rootArgs.clients)
	printf("Completed %v in %v, avg:%v\n", totalRequests, d, time.Duration(int64(d)/totalRequests))
}
