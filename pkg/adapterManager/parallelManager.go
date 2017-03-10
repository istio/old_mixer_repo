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

package adapterManager

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	rpc "github.com/googleapis/googleapis/google/rpc"

	"istio.io/mixer/pkg/aspect"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/config"
	"istio.io/mixer/pkg/pool"
	"istio.io/mixer/pkg/status"
)

// ParallelManager is an istio.io/mixer/pkg/api.Executor which wraps a Manager. Its Execute method calls the
// wrapped Manager's Execute in parallel across a pool of goroutines. The pool of worker goroutines is shared by all
// request goroutines, i.e. it's global.
type ParallelManager struct {
	*Manager
	wg sync.WaitGroup
}

// NewParallelManager returns a Manager who's Execute method calls the provided manager's Execute on each config in parallel.
func NewParallelManager(manager *Manager) *ParallelManager {
	return &ParallelManager{
		Manager: manager,
	}
}

// Execute takes a set of configurations and uses the ParallelManager's embedded Manager to execute all of them in parallel.
func (p *ParallelManager) Execute(ctx context.Context, cfgs []*config.Combined, attrs attribute.Bag, ma aspect.APIMethodArgs) aspect.Output {
	numCfgs := len(cfgs)
	// TODO: look into pooling both result array and channel, they're created per-request and are constant size for cfg lifetime.
	results := make([]result, numCfgs)
	resultChan := make(chan result, numCfgs)

	// schedule all the work that needs to happen
	for _, cfg := range cfgs {
		p.wg.Add(1)
		pool.ScheduleWork(func() {
			out := p.execute(ctx, cfg, attrs, ma)
			resultChan <- result{cfg, out}
			p.wg.Done()
		})
	}

	// wait for all the work to be done or the context to be cancelled
	for i := 0; i < numCfgs; i++ {
		select {
		case <-ctx.Done():
			return aspect.Output{Status: status.WithDeadlineExceeded(fmt.Sprintf("deadline exceeded waiting for adapter results with err: %v", ctx.Err()))}
		case res := <-resultChan:
			results[i] = res
		}
	}

	return combineResults(results)
}

// Combines a bunch of distinct result structs and turns 'em into one single Output struct
func combineResults(results []result) aspect.Output {
	var buf *bytes.Buffer
	code := rpc.OK

	for _, r := range results {
		if !r.out.IsOK() {
			if buf == nil {
				buf = pool.GetBuffer()
				// the first failure result's code becomes the result code for the output
				code = rpc.Code(r.out.Status.Code)
			} else {
				buf.WriteString(", ")
			}
			buf.WriteString(r.cfg.String() + ":" + r.out.Message())
		}
	}

	s := status.OK
	if buf != nil {
		s = status.WithMessage(code, buf.String())
		pool.PutBuffer(buf)
	}

	return aspect.Output{Status: s}
}

// Shutdown gracefully drains the ParallelManager's worker pool and terminates the worker go routines.
func (p *ParallelManager) Shutdown() {
	p.wg.Wait()
}

// result holds the values returned by the execution of an adapter
type result struct {
	cfg *config.Combined
	out aspect.Output
}
