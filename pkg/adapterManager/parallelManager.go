// Copyright 2017 Google Inc.
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
	"fmt"
	"sync"
	"sync/atomic"

	"context"

	"istio.io/mixer/pkg/aspect"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/config"
)

// ParallelManager is an istio.io/mixer/pkg/api.Executor which wraps a Manager. Its BulkExecute method calls the
// wrapped Manager's Execute in parallel across a pool of go routines. The pool of worker go routines is shared by all
// request go routines, i.e. it's global.
type ParallelManager struct {
	*Manager

	work chan<- task     // Used to hand tasks to the workers in the pool
	quit chan<- struct{} // Used to shutdown the workers in the pool
	wg   *sync.WaitGroup // Used to block shutdown until all workers complete
}

// NewParallelManager returns a Manager who's Execute method calls the provided manager's Execute on each config in parallel.
// size is the number of worker go routines in the worker pool the Manager schedules on to. This pool of workers is global:
// the size of the pool should be roughly (max outstanding requests allowed)*(number of configs executed per request).
func NewParallelManager(manager *Manager, size int) *ParallelManager {
	work := make(chan task, size)
	quit := make(chan struct{})
	p := &ParallelManager{
		Manager: manager,
		work:    work,
		quit:    quit,
		wg:      &sync.WaitGroup{},
	}

	p.wg.Add(size)
	for i := 0; i < size; i++ {
		go p.worker(work, quit, p.wg)
	}

	return p
}

// Execute takes a set of configurations and uses the ParallelManager's embedded Manager to execute all of them in parallel.
func (p *ParallelManager) Execute(ctx context.Context, cfgs []*config.Combined, attrs attribute.Bag) ([]*aspect.Output, error) {
	numCfgs := len(cfgs)
	// TODO: since we don't currently implement using different adapter sets per request,
	// len(cfgs) is constant of the life of the configuration for all requests. We should probably use a
	// pool for both the array of aspect outputs here and the result channels from requestGroup.
	results := make([]*aspect.Output, numCfgs)

	r, enqueue := p.requestGroup(attrs, numCfgs)
	for _, cfg := range cfgs {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("failed to enqueue all config executions with err: %v", ctx.Err())
		default:
			enqueue(ctx, cfg)
		}
	}

	for i := 0; i < numCfgs; i++ {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("deadline exceeded waiting for adapter results with err: %v", ctx.Err())
		case result := <-r:
			if result.err != nil {
				// TODO: should we return partial results too when we encounter an err?
				return nil, fmt.Errorf("%s returned err: %v", result.name, result.err)
			}
			results[i] = result.out
		}
	}
	return results, nil
}

// Shutdown gracefully drains the ParallelManager's worker pool and terminates the worker go routines.
func (p *ParallelManager) Shutdown() {
	close(p.quit)
	p.wg.Wait()
}

// result holds the values returned by the execution of an adapter on a go routine in the pool
type result struct {
	name string
	out  *aspect.Output
	err  error
}

// enqueueFunc is a handle into the pool and is used to add work to the pool. It will block until a worker in the pool
// is available to execute the work item.
type enqueueFunc func(ctx context.Context, cfg *config.Combined)

// task describes one unit of work to be executed by the pool
type task struct {
	ctx  context.Context
	cfg  *config.Combined
	ab   attribute.Bag
	done chan<- result
}

// requestGroup is called on the pool to get a handle to enqueue all of the adapter executions for a single request into the pool.
// The returned channel is the channel the adapters write their results to, and the enqueueFunc is used to enqueue the execution
// of a single adapter. The enqueueFunc will bock until there is a worker available to perform the work being enqueued.
//
// numAdapters is used to size the buffer of the result chan; it's important that the pool's workers not block returning results
// to avoid a deadlock amongst request go routines attempting to enqueue work onto the pool, so we need a buffer large enough for all
// adapters in a request to return their values. To enforce this invariant, enqueueFunc will panic if called more than numAdpaters times.
//
// It's assumed that all adapter executions for a single request share the same manager, attribute bag, and evaluator.
func (p *ParallelManager) requestGroup(ab attribute.Bag, numAdapters int) (<-chan result, enqueueFunc) {
	c := make(chan result, numAdapters)
	timesCalled := int32(0)
	allowedCalls := int32(numAdapters)

	return c, func(ctx context.Context, config *config.Combined) {
		if atomic.AddInt32(&timesCalled, 1) > allowedCalls {
			panic(fmt.Errorf("called enqueueFunc too many times; expected at most %d calls", allowedCalls))
		}
		p.work <- task{ctx, config, ab, c}
	}
}

// worker grabs a task off the queue, executes it, then blocks for the next signal (either a quit or another task).
func (p *ParallelManager) worker(task <-chan task, quit <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case t := <-task:
			out, err := p.execute(t.ctx, t.cfg, t.ab)
			t.done <- result{t.cfg.Builder.Name, out, err}
		case <-quit:
			return
		}
	}
}
