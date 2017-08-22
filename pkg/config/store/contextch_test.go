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

package store

import (
	"context"
	"sync"
	"testing"
	"time"
)

type contextChReceiver struct {
	ctx  context.Context
	ch   <-chan BackendEvent
	tick chan interface{}

	mu       sync.Mutex
	received []BackendEvent
}

func newContextChReceiver(ctx context.Context, cch *ContextChList) *contextChReceiver {
	ch := cch.Add(ctx)
	r := &contextChReceiver{ctx: ctx, ch: ch, tick: make(chan interface{}, 10)}
	go func() {
		for {
			select {
			case <-ctx.Done():
				close(r.tick)
				return
			case ev := <-ch:
				r.mu.Lock()
				r.received = append(r.received, ev)
				r.mu.Unlock()
				r.tick <- nil
			}
		}
	}()
	return r
}

func (r *contextChReceiver) GetReceivedAndReset() []BackendEvent {
	r.mu.Lock()
	received := r.received
	r.received = nil
	r.mu.Unlock()
	return received
}

func TestContextCh(t *testing.T) {
	cch := NewContextChList()

	ctx, cancel := context.WithCancel(context.Background())
	r1 := newContextChReceiver(ctx, cch)
	r2 := newContextChReceiver(ctx, cch)
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	r3 := newContextChReceiver(ctx2, cch)

	ev1 := BackendEvent{Key: Key{Kind: "kind", Namespace: "ns", Name: "name1"}, Type: Update}
	cch.Send(ev1)
	<-r1.tick
	<-r2.tick
	<-r3.tick
	rev := r1.GetReceivedAndReset()
	if len(rev) != 1 || rev[0].Key != ev1.Key {
		t.Errorf("Got %+v, Want %+v", rev, []BackendEvent{ev1})
	}
	rev = r2.GetReceivedAndReset()
	if len(rev) != 1 || rev[0].Key != ev1.Key {
		t.Errorf("Got %+v, Want %+v", rev, []BackendEvent{ev1})
	}
	rev = r3.GetReceivedAndReset()
	if len(rev) != 1 || rev[0].Key != ev1.Key {
		t.Errorf("Got %+v, Want %+v", rev, []BackendEvent{ev1})
	}

	cancel()
	ev2 := BackendEvent{Key: Key{Kind: "kind", Namespace: "ns", Name: "name2"}, Type: Update}
	cch.Send(ev2)
	<-r1.tick
	<-r2.tick
	<-r3.tick
	rev = r1.GetReceivedAndReset()
	if len(rev) != 0 {
		t.Errorf("Want empty, Got %+v", rev)
	}
	rev = r2.GetReceivedAndReset()
	if len(rev) != 0 {
		t.Errorf("Want empty, Got %+v", rev)
	}
	rev = r3.GetReceivedAndReset()
	if len(rev) != 1 || rev[0].Key != ev2.Key {
		t.Errorf("Got %+v, Want %+v", rev, []BackendEvent{ev2})
	}

	// make sure the remove runs.
	time.Sleep(time.Millisecond)
	var listSize int
	cch.mu.RLock()
	listSize = len(cch.list)
	cch.mu.RUnlock()
	if listSize != 1 {
		t.Errorf("Got %d Want 1", listSize)
	}
}
