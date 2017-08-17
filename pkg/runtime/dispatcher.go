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
	"context"
	"errors"
	"fmt"
	"strconv"
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

// Action is the runtime representation of configured action - cpb.Action.
// Configuration is processed to hydrate instance names to Instances and handler.
type Action struct {
	// ready to use handler
	handler adapter.Handler
	// configuration for the handler
	handlerConfig *cpb.Handler
	// instanceConfigs to dispatch to the handler.
	// instanceConfigs must belong to the same template.
	instanceConfig []*cpb.Instance
}

// Resolver represents the current snapshot of the configuration database
// and associated, initialized handlers.
type Resolver interface {
	// Resolve resolves configuration to a list of actions.
	Resolve(bag attribute.Bag, variety adptTmpl.TemplateVariety) ([]*Action, error)
}

// Newdispatcher creates a new dispatcher.
func Newdispatcher(mapper expr.Evaluator, templateRepo template.Repository,
	rt Resolver, gp *pool.GoroutinePool) Dispatcher {
	m := &dispatcher{
		mapper:       mapper,
		templateRepo: templateRepo,
		gp:           gp,
	}
	m.SetResolver(rt)
	return m
}

// dispatcher is responsible for dispatching incoming API calls
// to the configured adapters. It implements the Dispatcher interface.
type dispatcher struct {
	// mapper is the selector and expression evaluator.
	// It is not directly used by dispatcher.
	mapper expr.Evaluator

	// Repository of templates that are compiled in this Mixer.
	// It provides a way to perform template operations.
	templateRepo template.Repository

	// <resolver>
	resolver atomic.Value

	// gp is used to dispatch multiple adapters concurrently.
	gp *pool.GoroutinePool
}

// SetState installs a new runtime state.
func (m *dispatcher) SetResolver(rt Resolver) {
	m.resolver.Store(rt)
}

// State gets the current runtime state.
func (m *dispatcher) Resolver() Resolver {
	return m.resolver.Load().(Resolver)
}

// Dispatcher dispatches incoming API calls to configured adapters.
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
	// Name of the handler being called. Informational.
	handlerName string
	// Name of adapter that created the handler. Informational.
	adapterName string
	// handler to call.
	handler adapter.Handler
	// instance configuration to call the handler with.
	instances []*cpb.Instance
}

// genDispatchFn creates dispatchFn closures based on the given call.
type genDispatchFn func(call *dispatchInfo) []dispatchFn

// dispatch dispatches to all function based on function pointers.
func (m *dispatcher) dispatch(ctx context.Context, requestBag attribute.Bag, variety adptTmpl.TemplateVariety,
	filterFunc filterFunc, genDispatchFn genDispatchFn) (adapter.Result, error) {
	rt := m.Resolver()
	// cannot be null by construction.
	calls, nInstances, err := fetchDispatchInfo(requestBag, variety, rt, m.templateRepo, filterFunc)
	if err != nil {
		return nil, err
	}

	ra := make([]*runArg, 0, nInstances)
	for _, call := range calls {
		for _, df := range genDispatchFn(call) {
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
// Dispatcher#Report.
func (m *dispatcher) Report(ctx context.Context, requestBag attribute.Bag) error {
	_, err := m.dispatch(ctx, requestBag, adptTmpl.TEMPLATE_VARIETY_REPORT,
		nil,
		func(call *dispatchInfo) []dispatchFn {
			instCfg := make(map[string]proto.Message)
			for _, inst := range call.instances {
				instCfg[inst.Name] = inst.Params.(proto.Message)
			}
			return []dispatchFn{func(ctx context.Context) *result {
				err := call.processor.ProcessReport(ctx, instCfg, requestBag, m.mapper, call.handler)
				return &result{err: err, callinfo: call}
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
func (m *dispatcher) Check(ctx context.Context, requestBag attribute.Bag) (*adapter.CheckResult, error) {
	cres, err := m.dispatch(ctx, requestBag, adptTmpl.TEMPLATE_VARIETY_CHECK,
		nil,
		func(call *dispatchInfo) []dispatchFn {
			ra := make([]dispatchFn, 0, len(call.instances))
			for _, inst := range call.instances {
				ra = append(ra,
					func(ctx context.Context) *result {
						resp, err := call.processor.ProcessCheck(ctx, inst.Name,
							inst.Params.(proto.Message),
							requestBag, m.mapper,
							call.handler)
						return &result{err, &resp, call}
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
// Quota calls are dispatched to at most one handler.
// Dispatcher#Quota.
func (m *dispatcher) Quota(ctx context.Context, requestBag attribute.Bag,
	qma *aspect.QuotaMethodArgs) (*adapter.QuotaResult2, error) {
	dispatched := false
	qres, err := m.dispatch(ctx, requestBag, adptTmpl.TEMPLATE_VARIETY_QUOTA,
		func(inst *cpb.Instance) bool {
			return inst.Name == qma.Quota
		},
		func(call *dispatchInfo) []dispatchFn {
			// assert len(call.instances) != 0 by construction.
			inst := call.instances[0] // the 1st instance will be dispatched.
			if dispatched {           // ensures only one call is dispatched.
				glog.Warningf("Multiple dispatch: not dispatching %s to handler %s", inst.Name, call.handlerName)
				return nil
			}
			dispatched = true
			return []dispatchFn{
				func(ctx context.Context) *result {
					resp, err := call.processor.ProcessQuota(ctx, inst.Name,
						inst.Params.(proto.Message), requestBag, m.mapper, call.handler,
						adapter.QuotaRequestArgs{
							DeduplicationID: qma.DeduplicationID,
							QuotaAmount:     qma.Amount,
							BestEffort:      qma.BestEffort,
						})
					return &result{err, &resp, call}
				},
			}
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
func (m *dispatcher) Preprocess(ctx context.Context, requestBag attribute.Bag, responseBag *attribute.MutableBag) error {
	// FIXME
	return errors.New("not implemented")
}

// combineResults combines results
func combineResults(results []*result) (adapter.Result, error) {
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
			if rc, ok := res.(adapter.ResultCombiner); ok { // only check is expected to supported combining.
				rc.Combine(rs.res)
			}
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

// dispatchFn is the abstraction used by runAsync to dispatch to adapters.
type dispatchFn func(context.Context) *result

// result encapsulates commonalities between all adapter returns.
type result struct {
	// all results return an error
	err error
	// CheckResult or QuotaResult
	res adapter.Result
	// callinfo that resulted in "res". Used for informational purposes.
	callinfo *dispatchInfo
}

// runArg encapsulates callinfo with the dispatchFn that acts on it.
type runArg struct {
	callinfo *dispatchInfo
	do       dispatchFn
}

// run runArgs using runAsync and return results.
func (m *dispatcher) run(ctx context.Context, runArgs []*runArg) (adapter.Result, error) {
	nresults := len(runArgs)
	resultsChan := make(chan *result, nresults)
	results := make([]*result, nresults)

	for _, ra := range runArgs {
		m.runAsync(ctx, ra.callinfo, resultsChan, ra.do)
	}

	for i := 0; i < nresults; i++ {
		results[i] = <-resultsChan
	}
	return combineResults(results)
}

// runAsync runs the dispatchFn using a scheduler. It also adds a new span and records prometheus metrics.
func (m *dispatcher) runAsync(ctx context.Context, callinfo *dispatchInfo, results chan *result, do dispatchFn) {
	m.gp.ScheduleWork(func() {
		// tracing
		op := fmt.Sprintf("%s:%s(%s)", callinfo.processor.Name, callinfo.handlerName, callinfo.adapterName)
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
			tracelog.String(adapterName, callinfo.adapterName),
			tracelog.String(responseCode, rpc.Code_name[st.Code]),
			tracelog.String(responseMsg, st.Message),
			tracelog.Bool(adapterError, out.err != nil),
		)

		dispatchLbls := prometheus.Labels{
			meshFunction: callinfo.processor.Name,
			handlerName:  callinfo.handlerName,
			adapterName:  callinfo.adapterName,
			responseCode: rpc.Code_name[st.Code],
			adapterError: strconv.FormatBool(out.err != nil),
		}
		dispatchCounter.With(dispatchLbls).Inc()
		dispatchDuration.With(dispatchLbls).Observe(duration.Seconds())

		results <- out
		span.Finish()
	})
}

// filterFunc decides if a particular action is interesting to the caller.
type filterFunc func(inst *cpb.Instance) bool

// instancesPerHandler is the expected number of instances per handler.
// It is used to avoid slice reallocation.
const instancesPerHandler = 5

// fetchDispatchInfo returns an array of dispatches.
// dispatchInfo struct has all the information necessary to realize a function call to a handler.
// dispatchInfo is grouped by template name. It also returns the number of unique handler-instance pairs.
func fetchDispatchInfo(requestBag attribute.Bag, variety adptTmpl.TemplateVariety, rt Resolver,
	templateRepo template.Repository, filter filterFunc) ([]*dispatchInfo, int, error) {
	actions, err := rt.Resolve(requestBag, variety)
	if err != nil {
		glog.Error(err)
		return nil, 0, err
	}
	if glog.V(2) {
		glog.Infof("Resolved (%v) %d actions", variety, len(actions))
	}

	var ti template.Info
	var found bool
	nInstances := 0

	callmap := make(map[string]*dispatchInfo)

	for _, action := range actions {
		for _, inst := range action.instanceConfig {
			if filter != nil && !filter(inst) {
				continue
			}

			if ti, found = templateRepo.GetTemplateInfo(inst.Template); !found {
				glog.Warningf("template: %s is not available", inst.Template)
				continue
			}
			mapKey := action.handlerConfig.Name + "/" + inst.Template
			di := callmap[mapKey]
			if di == nil {
				di = &dispatchInfo{
					processor:   &ti,
					handlerName: action.handlerConfig.Name,
					adapterName: action.handlerConfig.Adapter,
					handler:     action.handler,
					instances:   make([]*cpb.Instance, 0, instancesPerHandler),
				}
				callmap[mapKey] = di
			}

			di.instances = append(di.instances, inst)
			nInstances++
		}
	}

	calls := make([]*dispatchInfo, 0, len(callmap))
	for _, di := range callmap {
		calls = append(calls, di)
	}

	if glog.V(2) {
		glog.Infof("Resolved (%v) %d calls, %d instances", variety, len(actions), nInstances)
	}

	return calls, nInstances, nil
}
