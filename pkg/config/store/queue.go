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
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/glog"
)

type eventQueue struct {
	ctx   context.Context
	chout chan Event
	chin  <-chan Event
	kinds map[string]proto.Message
}

func newQueue(ctx context.Context, chin <-chan Event, kinds map[string]proto.Message) *eventQueue {
	eq := &eventQueue{
		ctx:   ctx,
		chout: make(chan Event),
		chin:  chin,
		kinds: kinds,
	}
	go eq.run()
	return eq
}

func (q *eventQueue) transformValue(ev *Event) error {
	spec, ok := ev.Value.(map[string]interface{})
	if !ok {
		return nil
	}
	var err error
	ev.Value, err = convertWithKind(spec, ev.Kind, q.kinds)
	return err
}

func (q *eventQueue) run() {
loop:
	for {
		select {
		case <-q.ctx.Done():
			fmt.Printf("done0\n")
			break loop
		case ev := <-q.chin:
			fmt.Printf("ev0: %+v\n", ev)
			evs := []Event{ev}
			for len(evs) > 0 {
				if err := q.transformValue(&evs[0]); err != nil {
					glog.Errorf("Failed to transform an event: %v", err)
					evs = evs[1:]
					continue
				}
				fmt.Printf("%d\n", len(evs))
				select {
				case <-q.ctx.Done():
					fmt.Printf("done\n")
					break loop
				case ev := <-q.chin:
					fmt.Printf("ev: %+v\n", ev)
					evs = append(evs, ev)
				case q.chout <- evs[0]:
					evs = evs[1:]
				}
			}
		}
	}
	fmt.Printf("closed\n")
	close(q.chout)
}
