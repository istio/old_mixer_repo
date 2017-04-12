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

package config

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/config/descriptor"
)

type mtest struct {
	gcContent string
	gc        string
	scContent string
	sc        string
	ada       map[string]adapter.ConfigValidator
	asp       map[Kind]AspectValidator
	errStr    string
}

type fakelistener struct {
	called int
	rt     Resolver
	df     descriptor.Finder
	sync.Mutex
}

func (f *fakelistener) ConfigChange(cfg Resolver, df descriptor.Finder) {
	f.Lock()
	f.rt = cfg
	f.df = df
	f.called++
	f.Unlock()
}
func (f *fakelistener) Called() int {
	f.Lock()
	called := f.called
	f.Unlock()
	return called
}

func TestConfigManager(t *testing.T) {
	evaluator := newFakeExpr()
	mlist := []mtest{
		{"", "", "", "", nil, nil, ""},
		{sGlobalConfig, "globalconfig", "", "", nil, nil, "failed validation"},
		{sGlobalConfig, "globalconfig", sSvcConfig, "serviceconfig", nil, nil, "failed validation"},
		{sGlobalConfigValid, "globalconfig", sSvcConfig2, "serviceconfig", map[string]adapter.ConfigValidator{
			"denyChecker": &lc{},
			"metrics":     &lc{},
			"listchecker": &lc{},
		}, map[Kind]AspectValidator{
			DenialsKind: &ac{},
			MetricsKind: &ac{},
			ListsKind:   &ac{},
		}, ""},
	}
	for idx, mt := range mlist {
		t.Run(strconv.Itoa(idx), func(t *testing.T) {
			loopDelay := time.Millisecond * 50
			vf := newVfinder(mt.ada, mt.asp)
			store := newFakeStore(mt.gcContent, mt.scContent)
			if mt.errStr != "" {
				store.err = errors.New(mt.errStr)
			}
			ma := NewManager(evaluator, vf.FindAspectValidator, vf.FindAdapterValidator, vf.AdapterToAspectMapperFunc, store, loopDelay)
			testConfigManager(t, ma, mt, loopDelay, idx)
		})
	}
}

func testConfigManager(t *testing.T, mgr *Manager, mt mtest, loopDelay time.Duration, idx int) {
	fl := &fakelistener{}
	mgr.Register(fl)

	mgr.Start()
	defer mgr.Close()

	le := mgr.LastError()

	if mt.errStr != "" && le == nil {
		t.Fatalf("Expected an error %s Got nothing", mt.errStr)
	}

	if mt.errStr == "" && le != nil {
		t.Fatalf("Unexpected error %s", le)
	}

	if mt.errStr == "" && fl.rt == nil {
		t.Error("Config listener was not notified")
	}

	if mt.errStr == "" && le == nil {
		called := fl.Called()
		if le == nil && called != 1 {
			t.Errorf("called Got: %d, want: 1", called)
		}
		// give mgr time to go thru the start Loop() go routine
		// fetchAndNotify should be indirectly called multiple times.
		time.Sleep(loopDelay * 2)
		// check again. should not change, no new data is available
		called = fl.Called()
		if le == nil && called != 1 {
			t.Errorf("called Got: %d, want: 1", called)
		}
		return
	}

	if !strings.Contains(le.Error(), mt.errStr) {
		t.Fatalf("Unexpected error. Expected %s\nGot: %s\n", mt.errStr, le)
	}
}

// fakeMemStore

type fakeMemStore struct {
	data     map[string]string
	index    int
	err      error
	writeErr error

	cl StoreListener
	sync.RWMutex
}

var _ KeyValueStore = &fakeMemStore{}

func (f *fakeMemStore) String() string {
	return "fakeMemStore"
}

// Get value at a key, false if not found.
func (f *fakeMemStore) Get(key string) (value string, index int, found bool) {
	f.RLock()
	defer f.RUnlock()

	if f.err != nil {
		return "", f.index, false
	}
	v, found := f.data[key]
	return v, f.index, found
}

// Set a value
func (f *fakeMemStore) Set(key string, value string) (index int, err error) {
	f.Lock()
	defer f.Unlock()
	if f.writeErr != nil {
		return f.index, f.writeErr
	}

	f.index++
	f.data[key] = value

	go func(idx int, cl StoreListener) {
		if cl != nil {
			cl.NotifyStoreChanged(idx)
		}
	}(f.index, f.cl)

	return f.index, f.writeErr
}

// List keys with the prefix
func (f *fakeMemStore) List(key string, recurse bool) (keys []string, index int, err error) {
	f.RLock()
	defer f.RUnlock()
	if f.err != nil {
		return nil, f.index, f.err
	}

	for k := range f.data {
		if strings.HasPrefix(k, key) {
			keys = append(keys, k)
		}
	}
	index = f.index
	return
}

// Delete
func (f *fakeMemStore) Delete(key string) error {
	f.Lock()
	defer f.Unlock()
	if f.err != nil {
		return f.err
	}

	delete(f.data, key)
	return nil
}

func (f *fakeMemStore) RegisterStoreChangeListener(s StoreListener) {
	f.cl = s
}

func newFakeStore(gc string, sc string) *fakeMemStore {
	return &fakeMemStore{
		data: newFakeMap(gc, sc),
	}
}

func newFakeMap(gc string, sc string) map[string]string {
	if gc == "" {
		gc = "{}"
	}
	if sc == "" {
		sc = "{}"
	}
	return map[string]string{
		keyGlobalServiceConfig: sc,
		keyAdapters:            gc,
		keyDescriptors:         "{}",
	}
}
