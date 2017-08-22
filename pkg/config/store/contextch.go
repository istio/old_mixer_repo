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
)

// contextCh is a utility type of outgoing channel with context. It can send messages
// as long as the context is valid.
type contextCh struct {
	ctx context.Context
	ch  chan<- BackendEvent
}

// ContextChList manages the list of ContextCh.
type ContextChList struct {
	mu   sync.RWMutex
	list []*contextCh
}

// NewContextChList creates a new list of ContextCh.
func NewContextChList() *ContextChList {
	return &ContextChList{}
}

// Add adds a new ContextCh instance. The instance will be removed when the context is done.
func (l *ContextChList) Add(ctx context.Context) <-chan BackendEvent {
	l.mu.Lock()
	ch := make(chan BackendEvent)
	l.list = append(l.list, &contextCh{ctx, ch})
	go l.remove(ctx, ch)
	l.mu.Unlock()
	return ch
}

// Send sends the event to the channel in this list.
func (l *ContextChList) Send(ev BackendEvent) {
	l.mu.RLock()
	for _, ch := range l.list {
		select {
		case <-ch.ctx.Done():
		case ch.ch <- ev:
		}
	}
	l.mu.RUnlock()
}

func (l *ContextChList) remove(ctx context.Context, ch chan<- BackendEvent) {
	<-ctx.Done()
	l.mu.Lock()
	for i, cch := range l.list {
		if cch.ch == ch {
			l.list = append(l.list[:i], l.list[i+1:]...)
		}
	}
	l.mu.Unlock()
}
