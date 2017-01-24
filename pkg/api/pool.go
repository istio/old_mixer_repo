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

// pool.go contains an implementation of a pool of workers that execute aspects.

package api

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"istio.io/mixer/pkg/adapterManager"
	"istio.io/mixer/pkg/aspect"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/expr"
)

// result holds the values returned by the execution of an adapter on a thread in the pool
type result struct {
	name string
	out  *aspect.Output
	err  error
}

// enqueueFunc is a handle into the pool and is used to add work to the pool. It will block until a worker in the pool
// is available to execute the work item.
type enqueueFunc func(ctx context.Context, config *aspect.CombinedConfig)

// task describes one unit of work to be executed by the pool
type task struct {
	ctx    context.Context
	mngr   *adapterManager.Manager
	cfg    *aspect.CombinedConfig
	ab     attribute.Bag
	mapper expr.Evaluator
	done   chan<- result
}

// pool refers to a group of goroutinues that execute tasks and return results
type pool struct {
	work chan<- task     // Used to hand work items to the workers in the pool
	quit chan<- struct{} // Used to kill the workers in the queue
	wg   *sync.WaitGroup // Used to block shutdown until all workers complete
}

// newPool returns a new pool of workers; it should be called once at server startup, and requestGroup should be called
// per request to get a handle to enqueue work on the pool.
func newPool(size uint) *pool {
	work := make(chan task, size)
	quit := make(chan struct{})
	wg := &sync.WaitGroup{}

	isize := int(size)
	if isize < 0 {
		panic("Number of workers must be less than max int val")
	}

	wg.Add(isize)
	for i := 0; i < isize; i++ {
		go worker(work, quit, wg)
	}

	return &pool{
		work: work,
		quit: quit,
		wg:   wg,
	}
}

// requestGroup is called on the pool to get a handle to enqueue all of the adapter executions for a single request into the pool.
// The returned channel is the channel the adapters write their results to, and the enqueueFunc is used to enqueue the execution
// of a single adapter. The enqueueFunc will bock until there is a worker available to perform the work being enqueued.
//
// numAdapters is used to size the buffer of the result chan; it's important that the pool's workers not block returning results
// to avoid a deadlock amongst request threads attempting to enqueue work onto the pool, so we need a buffer large enough for all
// adapters in a request to return their values.
//
// It's assumed that all adapter executions for a single request share the same manager, attribute bag, and evaluator.
func (p *pool) requestGroup(mngr *adapterManager.Manager, ab attribute.Bag, eval expr.Evaluator, numAdapters int) (<-chan result, enqueueFunc) {
	c := make(chan result, numAdapters)
	timesCalled := int32(0)
	allowedCalls := int32(numAdapters)

	return c, func(ctx context.Context, config *aspect.CombinedConfig) {
		if atomic.AddInt32(&timesCalled, 1) > allowedCalls {
			panic(fmt.Errorf("called enqueueFunc too many times; expected at most %d calls", allowedCalls))
		}
		p.work <- task{ctx, mngr, config, ab, eval, c}
	}
}

// shutdown sends a close signal to all workers and blocks until they all exit.
func (p *pool) shutdown() {
	close(p.quit)
	p.wg.Wait()
}

// worker grabs a task off the queue, executes it, then blocks for the next signal (either a quit or another task).
func worker(task <-chan task, quit <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case t := <-task:
			// TODO: plumb ctx through to adapterManager.Execute.
			_ = t.ctx
			out, err := t.mngr.Execute(t.cfg, t.ab, t.mapper)
			t.done <- result{t.cfg.Builder.Name, out, err}
		case <-quit:
			return
		}
	}
}
