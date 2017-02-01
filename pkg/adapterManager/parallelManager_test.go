// Copyright 2016 Google Inc.
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
	"context"
	"fmt"
	"testing"
	"time"

	"google.golang.org/genproto/googleapis/rpc/code"

	"istio.io/mixer/pkg/aspect"
	"istio.io/mixer/pkg/config"
	configpb "istio.io/mixer/pkg/config/proto"
)

func TestBulkExecute(t *testing.T) {
	cases := []struct {
		name     string
		inCode   code.Code
		inErr    error
		wantCode code.Code
	}{
		{"ok", code.Code_OK, nil, code.Code_OK},
		{"error", code.Code_UNKNOWN, fmt.Errorf("expected"), code.Code_UNKNOWN},
	}

	for _, c := range cases {
		mngr := newTestManager(c.name, false, func() (*aspect.Output, error) {
			return &aspect.Output{Code: c.inCode}, c.inErr
		})
		mreg := map[string]aspect.Manager{
			c.name: mngr,
		}
		breg := &fakeBuilderReg{
			adp:   mngr,
			found: true,
		}
		m := NewParallelManager(newManager(breg, mreg, nil, nil), 1)

		cfg := []*config.Combined{
			{&configpb.Adapter{Name: c.name}, &configpb.Aspect{Kind: c.name}},
		}

		o, err := m.Execute(context.Background(), cfg, nil)
		if c.inErr != nil && err == nil {
			t.Errorf("m.BulkExecute(...) = %v; want err: %v", err, c.inErr)
		}
		if c.inErr == nil && !(len(o) > 0 && o[0].Code == code.Code_OK) {
			t.Errorf("m.BulkExecute(...) = %v; wanted len(o) == 1 && o[0].Code == code.Code_OK", o)
		}
	}
}

func TestBulkExecute_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	// we're skipping NewMethodHandlers so we don't have to deal with config since configuration should've matter when we have a canceled ctx
	handler := NewParallelManager(&Manager{}, 1)
	cancel()

	if _, err := handler.Execute(ctx, []*config.Combined{nil}, &fakebag{}); err == nil {
		t.Error("handler.BulkExecute(canceledContext, ...) = _, nil; wanted any err")
	}

}

func TestBulkExecute_TimeoutWaitingForResults(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	blockChan := make(chan struct{})

	name := "blocked"
	mngr := newTestManager(name, false, func() (*aspect.Output, error) {
		<-blockChan
		return &aspect.Output{Code: code.Code_OK}, nil
	})
	mreg := map[string]aspect.Manager{
		name: mngr,
	}
	breg := &fakeBuilderReg{
		adp:   mngr,
		found: true,
	}
	m := NewParallelManager(newManager(breg, mreg, nil, nil), 1)

	go func() {
		time.Sleep(1 * time.Millisecond)
		cancel()
	}()

	cfg := []*config.Combined{{
		&configpb.Adapter{Name: name},
		&configpb.Aspect{Kind: name},
	}}
	if _, err := m.Execute(ctx, cfg, &fakebag{}); err == nil {
		t.Error("handler.BulkExecute(canceledContext, ...) = _, nil; wanted any err")
	}
	close(blockChan)
}

func TestPoolSize(t *testing.T) {
	name := "t"
	blockChan := make(chan struct{})
	testMngr := newTestManager(name, false, func() (*aspect.Output, error) {
		<-blockChan
		return &aspect.Output{Code: code.Code_OK}, nil
	})
	mreg := map[string]aspect.Manager{
		name: testMngr,
	}
	breg := &fakeBuilderReg{
		adp:   testMngr,
		found: true,
	}
	manager := newManager(breg, mreg, nil, nil)
	m := NewParallelManager(manager, 1)

	cfg := &config.Combined{
		&configpb.Adapter{Name: name},
		&configpb.Aspect{Kind: name},
	}
	// Easier than creating a new manager directly and having to register everything. We need all the config either way.

	// Note: we allocate a result buffer of size two by passing in numAdapters = 2
	res, enqueue := m.requestGroup(&fakebag{}, 2)

	// Enqueue work which will not complete until blockChan is closed; since the pool size == 1 this blocks the queue
	enqueue(context.Background(), cfg)

	second := make(chan struct{})
	go func() {
		// this second enqueue will block until blockChan is closed
		enqueue(context.Background(), cfg)
		close(second)
	}()

	if len(res) != 0 {
		t.Errorf("len(res) = %d, wanted 0", len(res))
	}

	close(blockChan) // unblock the queue
	<-second         // block the test go routine till the second enqueue completes

	// It takes a little time for the two goroutines to write their results; we loop to make the test more reliable.
	for count := 0; len(res) != 2 && count < 5; count++ {
		time.Sleep(10 * time.Millisecond)
	}
	if len(res) != 2 {
		t.Errorf("got %d finished tasks, wanted 2", len(res))
	}

	for i := 0; i < 2; i++ {
		r := <-res
		if r.out.Code != code.Code_OK {
			t.Errorf("r.out.Code = %s, wanted %s", code.Code_name[int32(r.out.Code)], code.Code_name[int32(code.Code_OK)])
		}
	}
}

func TestShutdown(t *testing.T) {
	fail := make(chan struct{})
	succeed := make(chan struct{})
	p := NewParallelManager(&Manager{}, 1)

	go func() {
		time.Sleep(1 * time.Second)
		close(fail)
	}()

	go func() {
		p.Shutdown()
		close(succeed)
	}()

	select {
	case <-fail:
		t.Error("pool.shutdown() didn't complete in the expected time")
	case <-succeed:
	}
}

func TestEnqueuePanics(t *testing.T) {
	name := "t"
	testMngr := newTestManager(name, false, func() (*aspect.Output, error) {
		return &aspect.Output{Code: code.Code_OK}, nil
	})
	mreg := map[string]aspect.Manager{
		name: testMngr,
	}
	breg := &fakeBuilderReg{
		adp:   testMngr,
		found: true,
	}
	manager := newManager(breg, mreg, nil, nil)
	p := NewParallelManager(manager, 1)

	cfg := &config.Combined{
		&configpb.Adapter{Name: name},
		&configpb.Aspect{Kind: name},
	}

	numCalls := 1
	_, enqueue := p.requestGroup(&fakebag{}, numCalls)
	for i := 0; i < numCalls; i++ {
		enqueue(nil, cfg)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("enqueue(nil, cfg) got nil, want panic and non-nil recover.")
		}
	}()
	enqueue(nil, cfg)
	t.Fail()
}
