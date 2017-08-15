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
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	rpc "github.com/googleapis/googleapis/google/rpc"
	"github.com/hashicorp/go-multierror"
	"github.com/opentracing/opentracing-go"
	tracelog "github.com/opentracing/opentracing-go/log"
	"github.com/prometheus/client_golang/prometheus"

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

// RuntimeAction is the runtime representation of configured action - cpb.Action.
// Configuration is processed to hydrate instance names to Instances.
type RuntimeAction struct {
	// HandlerName to dispatch instances to.
	HandlerName string
	// Instances to dispatch to the handler.
	Instances []*cpb.Instance
}

// HandlerState represents a ready to use handler.
type HandlerState struct {
	// ready to use handler.
	Handler adapter.Handler
	// builder that produced the above handler.
	Builder *adapter.BuilderInfo
}

// RuntimeState represents the current snapshot of the configuration database
// and associated, initialized handlers.
type RuntimeState interface {
	// Resolve resolves configuration to a list of actions.
	ResolveConfig(bag attribute.Bag, variety adptTmpl.TemplateVariety) ([]*RuntimeAction, error)

	// Handler returns a ready to use handler
	Handler(name string) (*HandlerState, bool)
}

// NewManager2 creates a new Manager2 dispatcher.
func NewManager2(mapper expr.Evaluator, templateRepo template.Repository,
	rt RuntimeState, gp *pool.GoroutinePool) Dispatcher {
	m := &Manager2{
		mapper:       mapper,
		templateRepo: templateRepo,
		gp:           gp,
	}
	m.SetRuntimeState(rt)
	return m
}

// Manager2 is the 0.2 adapter manager.
type Manager2 struct {
	// mapper is the selector and expression evaluator.
	// It is passed through by Manager.
	mapper expr.Evaluator

	// Repository of templates that are compiled in this Mixer.
	// It provides a way to perform template operations.
	templateRepo template.Repository

	// <RuntimeState>
	rtState atomic.Value

	// gp is used to run manager code asynchronously.
	gp *pool.GoroutinePool
}

// SetRuntimeState installs a new runtime state.
func (m *Manager2) SetRuntimeState(state RuntimeState) {
	m.rtState.Store(state)
}

// RuntimeState gets the current runtime state.
func (m *Manager2) RuntimeState() RuntimeState {
	v := m.rtState.Load()
	if v == nil {
		return nil
	}
	return v.(RuntimeState)
}

// Dispatcher executes aspects associated with individual API methods
type Dispatcher interface {

	// Preprocess dispatches to the set of aspects that will run before any
	// other aspects in Mixer (aka: the Check, Report, Quota aspects).
	Preprocess(ctx context.Context, requestBag attribute.Bag, responseBag *attribute.MutableBag) error

	// Check dispatches to the set of aspects associated with the Check API method
	Check(ctx context.Context, requestBag attribute.Bag) (*adapter.CheckResult, error)

	// Report dispatches to the set of aspects associated with the Report API method
	Report(ctx context.Context, requestBag attribute.Bag) error

	// Quota dispatches to the set of aspects associated with the Quota API method
	Quota(ctx context.Context, requestBag attribute.Bag,
		qma *aspect.QuotaMethodArgs) (*adapter.QuotaResult2, error)
}

// dispatchInfo contains details needed to make a function call
type dispatchInfo struct {
	// generated code - processor.ProcessXXX ()
	processor *template.Info
	// Name of the handler being called.
	handlerName string
	// handler to call.
	handlerState *HandlerState
	// instance configuration to call the handler with.
	instances []*cpb.Instance
}

// genDoFunc creates doFuncs closures based on the given call.
type genDoFunc func(call *dispatchInfo) []doFunc

// dispatch dispatches to all function based on function pointers.
func (m *Manager2) dispatch(ctx context.Context, requestBag attribute.Bag, variety adptTmpl.TemplateVariety,
	filterFunc filterFunc, genDoFunc genDoFunc) (adapter.Result, error) {
	rt := m.RuntimeState()
	// cannot be null by construction.
	calls, err := fetchDispatchInfo(requestBag, variety, rt, m.templateRepo, filterFunc)
	if err != nil {
		return nil, err
	}

	ra := make([]*runArg, 0, 2*len(calls))
	for _, call := range calls {
		for _, df := range genDoFunc(call) {
			ra = append(ra, &runArg{
				call,
				df,
			})
		}
	}
	return m.run(ctx, ra)
}

// Report dispatches to the set of adapters associated with the Report API method
// Config validation ensures that things are consistent.
// If they are not, we should continue as far as possible on the runtime path
// before aborting. Returns an error if any of the adapters return an error.
// If not the results are combined to a single CheckResult.
// Dispatcher#Report.
func (m *Manager2) Report(ctx context.Context, requestBag attribute.Bag) error {
	_, err := m.dispatch(ctx, requestBag, adptTmpl.TEMPLATE_VARIETY_REPORT,
		nil,
		func(call *dispatchInfo) []doFunc {
			instCfg := make(map[string]proto.Message)
			for _, inst := range call.instances {
				instCfg[inst.Name] = inst.Params.(proto.Message)
			}
			return []doFunc{func(ctx context.Context) *result2 {
				err := call.processor.ProcessReport(ctx, instCfg, requestBag, m.mapper, call.handlerState.Handler)
				return &result2{err: err, callinfo: call}
			}}
		},
	)
	return err
}

// Check dispatches to the set of adapters associated with the Check API method
// Config validation ensures that things are consistent.
// If they are not, we should continue as far as possible on the runtime path
// before aborting. Returns an error if any of the adapters return an error.
// If not the results are combined to a single CheckResult.
// Dispatcher#Check.
func (m *Manager2) Check(ctx context.Context, requestBag attribute.Bag) (*adapter.CheckResult, error) {
	cres, err := m.dispatch(ctx, requestBag, adptTmpl.TEMPLATE_VARIETY_CHECK,
		nil,
		func(call *dispatchInfo) []doFunc {
			ra := make([]doFunc, 0, len(call.instances))
			for _, inst := range call.instances {
				ra = append(ra,
					func(ctx context.Context) *result2 {
						resp, err := call.processor.ProcessCheck(ctx, inst.Name,
							inst.Params.(proto.Message),
							requestBag, m.mapper,
							call.handlerState.Handler)
						return &result2{err, &resp, call}
					})
			}
			return ra
		},
	)

	var res *adapter.CheckResult
	if cres != nil {
		res = cres.(*adapter.CheckResult)
	}
	return res, err
}

// Quota dispatches to the set of aspects associated with the Quota API method
// Config validation ensures that things are consistent.
// If they are not, we should continue as far as possible on the runtime path
// before aborting. Returns an error if any of the adapters return an error.
// If not the results are combined to a single QuotaResult.
// Dispatcher#Quota.
func (m *Manager2) Quota(ctx context.Context, requestBag attribute.Bag,
	qma *aspect.QuotaMethodArgs) (*adapter.QuotaResult2, error) {
	qres, err := m.dispatch(ctx, requestBag, adptTmpl.TEMPLATE_VARIETY_QUOTA,
		func(inst *cpb.Instance) bool {
			return inst.Name == qma.Quota
		},
		func(call *dispatchInfo) []doFunc {
			ra := make([]doFunc, 0, len(call.instances))
			for _, inst := range call.instances {
				ra = append(ra,
					func(ctx context.Context) *result2 {
						resp, err := call.processor.ProcessQuota(ctx, inst.Name,
							inst.Params.(proto.Message), requestBag, m.mapper, call.handlerState.Handler,
							adapter.QuotaRequestArgs{
								DeduplicationID: qma.DeduplicationID,
								QuotaAmount:     qma.Amount,
								BestEffort:      qma.BestEffort,
							})
						return &result2{err, &resp, call}
					})
			}
			return ra
		},
	)
	var res *adapter.QuotaResult2
	if qres != nil {
		res = qres.(*adapter.QuotaResult2)
	}
	return res, err
}

// Preprocess runs the first phase of adapter processing before any other adapters are run.
// Attribute producing adapters are run in this phase.
func (m *Manager2) Preprocess(ctx context.Context, requestBag attribute.Bag, responseBag *attribute.MutableBag) error {
	// FIXME
	return errors.New("not implemented")
}

// combineResults2 combines results
func combineResults2(results []*result2) (adapter.Result, error) {
	var res adapter.Result
	var err *multierror.Error
	var buf *bytes.Buffer
	code := rpc.OK

	for _, rs := range results {
		if rs.err != nil {
			err = multierror.Append(err, rs.err)
		}

		if rs.res == nil { // When there is no return value like ProcessReport().
			continue
		}
		if res == nil {
			res = rs.res
		} else {
			res.Combine(rs.res)
		}
		st := rs.res.GetStatus()
		if !status.IsOK(st) {
			if buf == nil {
				buf = pool.GetBuffer()
				// the first failure result's code becomes the result code for the output
				code = rpc.Code(st.Code)
			} else {
				buf.WriteString(", ")
			}

			buf.WriteString(rs.callinfo.handlerName + ":" + st.Message)
		}
	}

	if buf != nil {
		res.SetStatus(status.WithMessage(code, buf.String()))
		pool.PutBuffer(buf)
	}
	return res, err.ErrorOrNil()
}

// doFunc is the abstraction used by runAsync to do work.
type doFunc func(context.Context) *result2

// result2 encapsulates commonalities between all adapter returns.
type result2 struct {
	// all results return an error
	err error
	// CheckResult or QuotaResult
	res adapter.Result
	// callinfo that resulted in "res"
	callinfo *dispatchInfo
}

// runArg encapsulates callinfo with the doFunc that acts on it.
type runArg struct {
	callinfo *dispatchInfo
	do       doFunc
}

// run runArgs using runAsync and return results.
func (m *Manager2) run(ctx context.Context, runArgs []*runArg) (adapter.Result, error) {
	nresults := len(runArgs)
	resultsChan := make(chan *result2, nresults)
	results := make([]*result2, nresults)

	for _, ra := range runArgs {
		m.runAsync(ctx, ra.callinfo, resultsChan, ra.do)
	}

	for i := 0; i < nresults; i++ {
		results[i] = <-resultsChan
	}
	return combineResults2(results)
}

// runAsync runs the doFunc using a scheduler. It also adds a new span and records prometheus metrics.
func (m *Manager2) runAsync(ctx context.Context, callinfo *dispatchInfo, results chan *result2, do doFunc) {
	m.gp.ScheduleWork(func() {
		// tracing
		op := fmt.Sprintf("%s:%s(%s)", callinfo.processor.Name, callinfo.handlerName, callinfo.handlerState.Builder.Name)
		span, ctx := opentracing.StartSpanFromContext(ctx, op)

		start := time.Now()
		out := do(ctx)

		st := status.OK
		if out.err != nil {
			st = status.WithError(out.err)
		}

		duration := time.Since(start)
		span.LogFields(
			tracelog.String(meshFunction, callinfo.processor.Name),
			tracelog.String(handlerName, callinfo.handlerName),
			tracelog.String(adapterName, callinfo.handlerState.Builder.Name),
			tracelog.String(responseCode, rpc.Code_name[st.Code]),
			tracelog.String(responseMsg, st.Message),
		)

		dispatchLbls := prometheus.Labels{
			meshFunction: callinfo.processor.Name,
			handlerName:  callinfo.handlerName,
			adapterName:  callinfo.handlerState.Builder.Name,
			responseCode: rpc.Code_name[st.Code],
		}
		dispatchCounter.With(dispatchLbls).Inc()
		dispatchDuration.With(dispatchLbls).Observe(duration.Seconds())

		results <- out
		span.Finish()
	})
}

// filterFunc decides if a particular action is interesting to the caller.
type filterFunc func(inst *cpb.Instance) bool

// fetchDispatchInfo returns an array of dispatches.
// dispatchInfo struct has all the information necessary to realize a function call to a handler.
// dispatchInfo is grouped by template name.
func fetchDispatchInfo(requestBag attribute.Bag, variety adptTmpl.TemplateVariety, rt RuntimeState,
	templateRepo template.Repository, filter filterFunc) ([]*dispatchInfo, error) {
	actions, err := rt.ResolveConfig(requestBag, variety)
	if err != nil {
		glog.Error(err)
		return nil, err
	}

	var hs *HandlerState
	var ti template.Info
	var found bool

	callmap := make(map[string]*dispatchInfo)

	for _, action := range actions {
		if hs, found = rt.Handler(action.HandlerName); !found {
			glog.Warningf("handler: %s is not available", action.HandlerName)
			continue
		}

		for _, inst := range action.Instances {
			if filter != nil && !filter(inst) {
				continue
			}

			if ti, found = templateRepo.GetTemplateInfo(inst.Template); !found {
				glog.Warningf("template: %s is not available", inst.Template)
				continue
			}
			mapKey := action.HandlerName + "/" + inst.Template
			di := callmap[mapKey]
			if di == nil {
				di = &dispatchInfo{
					processor:    &ti,
					handlerName:  action.HandlerName,
					handlerState: hs,
					instances:    make([]*cpb.Instance, 0, 5),
				}
				callmap[mapKey] = di
			}

			di.instances = append(di.instances, inst)
		}
	}

	calls := make([]*dispatchInfo, 0, len(callmap))
	for _, di := range callmap {
		calls = append(calls, di)
	}
	return calls, nil
}
