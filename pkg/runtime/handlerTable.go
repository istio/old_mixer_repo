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

package runtime

import (
	"bytes"
	"crypto/sha1"
	"encoding/gob"
	"sort"

	"github.com/golang/glog"

	"istio.io/mixer/pkg/adapter"
	cpb "istio.io/mixer/pkg/config/proto"
)

type buildHandlerFn func(*cpb.Handler, []*cpb.Instance) (adapter.Handler, error)

// HandlerEntry is an entry in the runtime handler table.
// The entire handler table is initialized at startup and sha
// is recorded per entry, so on config change the correct
// handlerEntries can be replaced.
type HandlerEntry struct {
	// Name of the handler
	Name string

	// Handler is the initialized handler object.
	Handler adapter.Handler

	// HandlerCreateError records error while creating the handler
	HandlerCreateError error

	// Instances is the global list of instances associated with this handler
	Instances map[string]bool

	// sha is used to verify and update the handlerEntry.
	// sha must use handler.Params since any change to Params
	// requires creation of a new handler instance.
	// Some handlers need to be told about data they are about to receive.
	// For example prometheus adapter initialized objects based on this configuration.
	// If new configInstances are associated with such a handler, it also necessitates
	// recreation of the handler.
	sha [sha1.Size]byte

	// If this is set, the handler is closed during cleanup.
	notInUse bool
}

func newHandlerTable(instanceConfig map[string]*cpb.Instance, handlerConfig map[string]*cpb.Handler,
	buildHandler buildHandlerFn) *handlerTable {
	return &handlerTable{
		instanceConfig: instanceConfig,
		handlerConfig:  handlerConfig,
		buildHandler:   buildHandler,
		table:          make(map[string]*HandlerEntry),
	}
}

// handlerTable initializes and maintains handlers.
// It can efficiently initialize the next version of handlers based
// on the previous handler table.
type handlerTable struct {
	instanceConfig map[string]*cpb.Instance
	handlerConfig  map[string]*cpb.Handler

	// Used to build handler
	buildHandler buildHandlerFn

	// table that maintains handler state
	table map[string]*HandlerEntry
}

// Associate an instance with a handler
func (t *handlerTable) Associate(handleName string, instanceName string) {
	h := t.table[handleName]
	if h == nil {
		h = &HandlerEntry{
			Name:      handleName,
			Instances: make(map[string]bool),
		}
		t.table[handleName] = h
	}
	h.Instances[instanceName] = true
}

// Initialize the handler table based on configuration and the old handler table.
// If handler config has not changed, connections from the old handler table are re-used.
// This method does not return an error, it records errors in the handlerEntry.
func (t *handlerTable) Initialize(oldTable map[string]*HandlerEntry) {
	t.computeSha()
	// run diff with the old handlerTable
	for oh, ohe := range oldTable {
		var he *HandlerEntry
		if he = t.table[oh]; he == nil {
			// old handler is not needed any more
			ohe.notInUse = true
			if glog.V(3) {
				glog.Infof("handler: %s will be removed", oh)
			}
			continue
		}

		if he.sha != ohe.sha {
			ohe.notInUse = true
			if glog.V(3) {
				glog.Infof("handler: %s will be replaced", oh)
			}
			continue
		}
		// shas match reuse
		he.Handler = ohe.Handler
	}

	// initialize handlers that were not previously covered.
	for _, he := range t.table {
		// this was already initialized.
		if he.Handler != nil {
			continue
		}
		// create a new handler
		// handler error is marked inside the entry.
		t.initHandler(he)
	}
}

// initialize handler, mark the handler as bad
func (t *handlerTable) initHandler(he *HandlerEntry) {
	hc := t.handlerConfig[he.Name]
	insts := make([]*cpb.Instance, 0, len(he.Instances))

	for instName := range he.Instances {
		insts = append(insts, t.instanceConfig[instName])
	}
	he.Handler, he.HandlerCreateError = t.buildHandler(hc, insts)
}

func encode(enc *gob.Encoder, e interface{}) {
	if err := enc.Encode(e); err != nil {
		glog.Warningf("Unable to encode %v", e)
	}
}

// computeSha for individual handler entries
func (t *handlerTable) computeSha() {
	var buff bytes.Buffer
	enc := gob.NewEncoder(&buff)
	for _, nh := range t.table {
		h := t.handlerConfig[nh.Name]
		encode(enc, h.Adapter)
		encode(enc, h.Params)

		// instances in alphabetical order
		// TODO add instance details only if the handler cares about it.
		insts := make([]string, 0, len(nh.Instances))
		for k := range nh.Instances {
			insts = append(insts, k)
		}
		sort.Strings(insts)
		for _, iname := range insts {
			inst := t.instanceConfig[iname]
			encode(enc, inst.Template)
			encode(enc, inst.Params)
		}
		nh.sha = sha1.Sum(buff.Bytes())
		buff.Reset()
	}
}
