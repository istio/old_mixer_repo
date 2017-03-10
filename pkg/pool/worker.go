// Copyright 2016 Istio Authors
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

// Package pool provides access to a mixer-global pool of buffers and a string interning table.
package pool

import (
	"sync"
)

// WorkFunc represents a function to invoke from a worker.
type WorkFunc func()

type goroutinePool struct {
	queue chan WorkFunc  // Channel providing the work that needs to be executed
	wg    sync.WaitGroup // Used to block shutdown until all workers complete
}

// The queue depth for the global worker pool
const globalPoolQueueDepth = 128

var globalGoroutinePool = newGoroutinePool(globalPoolQueueDepth)

func newGoroutinePool(queueDepth int) *goroutinePool {
	gp := &goroutinePool{
		queue: make(chan WorkFunc, queueDepth),
	}

	gp.AddWorkers(1)
	return gp
}

// Shutdown waits for all goroutines to terminate
func (gp *goroutinePool) Close() {
	close(gp.queue)
	gp.wg.Wait()
}

// ScheduleWork registers the given function to be executed at some point
func (gp *goroutinePool) ScheduleWork(fn WorkFunc) {
	gp.queue <- fn
}

// AddWorkers registers more goroutines in the worker pool, increasing potential parallelism.
func (gp *goroutinePool) AddWorkers(numWorkers int) {
	gp.wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			for fn := range gp.queue {
				fn()
			}

			gp.wg.Done()
		}()
	}
}

// ScheduleWork registers the given function to be executed at some point
func ScheduleWork(fn WorkFunc) {
	globalGoroutinePool.ScheduleWork(fn)
}

// AddWorkers registers more goroutines in the worker pool, increasing potential parallelism.
func AddWorkers(numWorkers int) {
	globalGoroutinePool.AddWorkers(numWorkers)
}
