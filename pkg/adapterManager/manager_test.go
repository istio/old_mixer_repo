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
	"strings"
	"testing"

	"istio.io/mixer/pkg/config"
	configpb "istio.io/mixer/pkg/config/proto"

	"fmt"

	"google.golang.org/genproto/googleapis/rpc/status"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/aspect"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/expr"
)

type (
	fakereg struct {
		adp   adapter.Adapter
		found bool
	}

	fakemgr struct {
		kind string
		aspect.Manager
		w      *fakewrapper
		called int8
	}

	fakebag struct {
		attribute.Bag
	}

	fakeevaluator struct {
		expr.Evaluator
	}

	fakewrapper struct {
		called int8
	}

	fakeadp struct {
		name string
		adapter.Adapter
	}
)

func (f *fakeadp) Name() string { return f.name }

func (f *fakewrapper) Execute(attrs attribute.Bag, mapper expr.Evaluator) (output *aspect.Output, err error) {
	f.called++
	return
}

func (m *fakemgr) Kind() string {
	return m.kind
}

func (m *fakemgr) NewAspect(cfg *config.Combined, adp adapter.Adapter, env adapter.Env) (aspect.Wrapper, error) {
	m.called++
	if m.w == nil {
		return nil, fmt.Errorf("unable to create aspect")
	}

	return m.w, nil
}

func (m *fakereg) ByImpl(adapterName string) (adapter.Adapter, bool) {
	return m.adp, m.found
}

type ttable struct {
	mgrFound  bool
	kindFound bool
	errString string
	wrapper   *fakewrapper
	cfg       *config.Combined
}

func getReg(found bool) *fakereg {
	return &fakereg{&fakeadp{name: "k1impl1"}, found}
}

func mgrs(w *fakewrapper) []aspect.Manager {
	return []aspect.Manager{&fakemgr{kind: "k1", w: w}, &fakemgr{kind: "k2"}}
}

func TestManager(t *testing.T) {
	goodcfg := &config.Combined{
		Aspect:  &configpb.Aspect{Kind: "k1", Params: &status.Status{}},
		Adapter: &configpb.Adapter{Kind: "k1", Impl: "k1impl1", Params: &status.Status{}},
	}

	badcfg1 := &config.Combined{
		Aspect: &configpb.Aspect{Kind: "k1", Params: &status.Status{}},
		Adapter: &configpb.Adapter{Kind: "k1", Impl: "k1impl1",
			Params: make(chan int)},
	}
	badcfg2 := &config.Combined{
		Aspect: &configpb.Aspect{Kind: "k1", Params: make(chan int)},
		Adapter: &configpb.Adapter{Kind: "k1", Impl: "k1impl1",
			Params: &status.Status{}},
	}
	emptyMgrs := []aspect.Manager{}
	attrs := &fakebag{}
	mapper := &fakeevaluator{}

	ttt := []ttable{
		{false, false, "could not find aspect manager", nil, goodcfg},
		{true, false, "could not find registered adapter", nil, goodcfg},
		{true, true, "", &fakewrapper{}, goodcfg},
		{true, true, "", nil, goodcfg},
		{true, true, "can't handle type", nil, badcfg1},
		{true, true, "can't handle type", nil, badcfg2},
	}

	for idx, tt := range ttt {
		r := getReg(tt.kindFound)
		mgr := emptyMgrs
		if tt.mgrFound {
			mgr = mgrs(tt.wrapper)
		}
		m := NewManager(r, mgr)
		errStr := ""
		if _, err := m.Execute(tt.cfg, attrs, mapper); err != nil {
			errStr = err.Error()
		}
		if !strings.Contains(errStr, tt.errString) {
			t.Errorf("[%d] expected: '%s' \ngot: '%s'", idx, tt.errString, errStr)
		}

		if tt.errString != "" || tt.wrapper == nil {
			continue
		}

		if tt.wrapper.called != 1 {
			t.Errorf("[%d] Expected wrapper call", idx)
		}
		fmgr := mgr[0].(*fakemgr)

		if fmgr.called != 1 {
			t.Errorf("[%d] Expected mgr.NewAspect call", idx)
		}

		// call again
		// check for cache
		_, _ = m.Execute(tt.cfg, attrs, mapper)
		if tt.wrapper.called != 2 {
			t.Errorf("[%d] Expected 2nd wrapper call", idx)
		}

		if fmgr.called != 1 {
			t.Errorf("[%d] UnExpected mgr.NewAspect call %d", idx, fmgr.called)
		}

	}

}
