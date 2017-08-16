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

package adapterManager

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/googleapis/googleapis/google/rpc"

	"istio.io/mixer/pkg/adapter"
	adptTmpl "istio.io/mixer/pkg/adapter/template"
	"istio.io/mixer/pkg/aspect"
	"istio.io/mixer/pkg/attribute"
	cpb "istio.io/mixer/pkg/config/proto"
	"istio.io/mixer/pkg/expr"
	"istio.io/mixer/pkg/pool"
	"istio.io/mixer/pkg/status"
	"istio.io/mixer/pkg/template"
)

func TestReport2(t *testing.T) {
	gp := pool.NewGoroutinePool(1, true)
	tname := "metric1"
	badTname := "metric2"
	err1 := errors.New("internal error")

	for _, s := range []struct {
		tn         string
		callErr    error
		resolveErr bool
		ncalled    int
		badHandler bool
	}{{tn: tname, ncalled: 1},
		{tn: tname, callErr: err1},
		{tn: tname, callErr: err1, resolveErr: true},
		{tn: badTname},
		{badHandler: true},
	} {
		t.Run(fmt.Sprintf("%#v", s), func(t *testing.T) {
			fp := &fakeProc{
				err: s.callErr,
			}
			tr := newTemplateRepo(tname, fp)
			var resolveErr error
			if s.resolveErr {
				resolveErr = s.callErr
			}
			rt := newRuntimeState("myhandler", "i1", s.tn, resolveErr, s.badHandler)
			m := NewManager2(nil, tr, rt, gp)

			err := m.Report(context.Background(), nil)
			checkError(t, s.callErr, err)
			if s.callErr != nil {
				return
			}
			if fp.called != s.ncalled {
				t.Errorf("got %v, want %v", fp.called, s.ncalled)
			}
		})
	}

	gp.Close()
}

func TestCheck2(t *testing.T) {
	gp := pool.NewGoroutinePool(1, true)
	tname := "metric1"
	badTname := "metric2"
	err1 := errors.New("internal error")

	for _, s := range []struct {
		tn         string
		callErr    error
		resolveErr bool
		ncalled    int
		badHandler bool
		cr         adapter.CheckResult
	}{{tn: tname, ncalled: 2},
		{tn: tname, ncalled: 2, cr: adapter.CheckResult{ValidUseCount: 200}},
		{tn: tname, ncalled: 2, cr: adapter.CheckResult{ValidUseCount: 200, Status: status.WithPermissionDenied("bad user")}},
		{tn: tname, callErr: err1},
		{tn: tname, callErr: err1, resolveErr: true},
		{tn: badTname},
		{badHandler: true},
	} {
		t.Run(fmt.Sprintf("%#v", s), func(t *testing.T) {
			fp := &fakeProc{
				err:         s.callErr,
				checkResult: s.cr,
			}
			tr := newTemplateRepo(tname, fp)
			var resolveErr error
			if s.resolveErr {
				resolveErr = s.callErr
			}
			rt := newRuntimeState("myhandler", "i1", s.tn, resolveErr, s.badHandler)
			m := NewManager2(nil, tr, rt, gp)

			cr, err := m.Check(context.Background(), nil)

			checkError(t, s.callErr, err)

			if s.callErr != nil {
				return
			}
			if fp.called != s.ncalled {
				t.Fatalf("got %v, want %v", fp.called, s.ncalled)
			}
			if s.ncalled == 0 {
				return
			}
			if cr == nil {
				t.Fatalf("got %v, want %v", cr, fp.checkResult)
			}
			if !reflect.DeepEqual(fp.checkResult.Status.Code, cr.Status.Code) {
				t.Fatalf("got %v, want %v", *cr, fp.checkResult)
			}
		})
	}

	gp.Close()
}

func TestQuota2(t *testing.T) {
	gp := pool.NewGoroutinePool(1, true)
	tname := "metric1"
	badTname := "metric2"
	err1 := errors.New("internal error")

	for _, s := range []struct {
		tn         string
		callErr    error
		resolveErr bool
		ncalled    int
		badHandler bool
		cr         adapter.QuotaResult2
	}{{tn: tname, ncalled: 1},
		{tn: tname, ncalled: 1, cr: adapter.QuotaResult2{Amount: 200}},
		{tn: tname, ncalled: 1, cr: adapter.QuotaResult2{Amount: 200, Status: status.WithPermissionDenied("bad user")}},
		{tn: tname, callErr: err1},
		{tn: tname, callErr: err1, resolveErr: true},
		{tn: badTname},
		{badHandler: true},
	} {
		t.Run(fmt.Sprintf("%#v", s), func(t *testing.T) {
			fp := &fakeProc{
				err:         s.callErr,
				quotaResult: s.cr,
			}
			tr := newTemplateRepo(tname, fp)
			var resolveErr error
			if s.resolveErr {
				resolveErr = s.callErr
			}
			rt := newRuntimeState("myhandler", "i1", s.tn, resolveErr, s.badHandler)
			m := NewManager2(nil, tr, rt, gp)

			cr, err := m.Quota(context.Background(), nil,
				&aspect.QuotaMethodArgs{
					Quota: "i1",
				})

			checkError(t, s.callErr, err)

			if s.callErr != nil {
				return
			}
			if fp.called != s.ncalled {
				t.Fatalf("got %v, want %v", fp.called, s.ncalled)
			}
			if s.ncalled == 0 {
				return
			}
			if cr == nil {
				t.Fatalf("got %v, want %v", cr, fp.quotaResult)
			}
			if !reflect.DeepEqual(fp.quotaResult.Status.Code, cr.Status.Code) {
				t.Fatalf("got %v, want %v", *cr, fp.quotaResult)
			}
		})
	}

	gp.Close()
}

func TestPreprocess2(t *testing.T) {
	m := Manager2{}

	err := m.Preprocess(context.TODO(), nil, nil)
	if err == nil {
		t.Fatalf("not working yet")
	}
}

// fakes

type fakeTRepo struct {
	template.Repository
	t map[string]template.Info
}

func (t *fakeTRepo) GetTemplateInfo(templateName string) (template.Info, bool) {
	ti, found := t.t[templateName]
	return ti, found
}

type fakeRuntimeState struct {
	ra  []*RuntimeAction
	err error
	hs  *HandlerState
}

func checkError(t *testing.T, want error, err error) {
	if err == nil {
		if want != nil {
			t.Fatalf("got %v, want %v", err, want)
		}
	} else {
		if want == nil {
			t.Fatalf("got %v, want %v", err, want)
		}
		if !strings.Contains(err.Error(), want.Error()) {
			t.Fatalf("got %v, want %v", err, want)
		}
	}
}

// Resolve resolves configuration to a list of actions.
func (f *fakeRuntimeState) ResolveConfig(bag attribute.Bag, variety adptTmpl.TemplateVariety) ([]*RuntimeAction, error) {
	return f.ra, f.err
}

// Handler returns a ready to use handler
func (f *fakeRuntimeState) Handler(name string) (*HandlerState, bool) {
	if f.hs == nil {
		return nil, false
	}
	return f.hs, true
}

func newRuntimeState(hndlr string, instanceName string, tname string, resolveErr error, badhandler bool) *fakeRuntimeState {
	rt := &fakeRuntimeState{
		ra: []*RuntimeAction{
			{
				hndlr,
				[]*cpb.Instance{
					{
						instanceName,
						tname,
						&google_rpc.Status{},
					},
					{
						instanceName + "1",
						tname,
						&google_rpc.Status{},
					},
				},
			},
		},
		hs: &HandlerState{
			Info: &adapter.BuilderInfo{},
		},
		err: resolveErr,
	}

	if badhandler {
		rt.hs = nil
	}

	return rt
}

func newTemplateRepo(name string, fproc *fakeProc) *fakeTRepo {
	return &fakeTRepo{
		t: map[string]template.Info{
			name: {
				Name:          name,
				ProcessReport: fproc.ProcessReport,
				ProcessCheck:  fproc.ProcessCheck,
				ProcessQuota:  fproc.ProcessQuota,
			},
		},
	}
}

type fakeProc struct {
	called      int
	err         error
	checkResult adapter.CheckResult
	quotaResult adapter.QuotaResult2
}

func (f *fakeProc) ProcessReport(ctx context.Context, instCfg map[string]proto.Message,
	attrs attribute.Bag, mapper expr.Evaluator, handler adapter.Handler) error {
	f.called++
	return f.err
}
func (f *fakeProc) ProcessCheck(ctx context.Context, instName string, instCfg proto.Message, attrs attribute.Bag,
	mapper expr.Evaluator, handler adapter.Handler) (adapter.CheckResult, error) {
	f.called++
	return f.checkResult, f.err
}

func (f *fakeProc) ProcessQuota(ctx context.Context, quotaName string, quotaCfg proto.Message, attrs attribute.Bag,
	mapper expr.Evaluator, handler adapter.Handler, args adapter.QuotaRequestArgs) (adapter.QuotaResult2, error) {
	f.called++
	return f.quotaResult, f.err
}
