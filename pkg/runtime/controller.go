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
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"

	"istio.io/mixer/pkg/adapter"
	adptTmpl "istio.io/mixer/pkg/adapter/template"
	cpb "istio.io/mixer/pkg/config/proto"
	"istio.io/mixer/pkg/config/store"
	"istio.io/mixer/pkg/expr"
	"istio.io/mixer/pkg/pool"
	"istio.io/mixer/pkg/template"
)

// Controller is responsible for watching configuration using the Store2 API.
// Controller produces a resolver and installs it in the dispatcher.
// Controller consumes potentially inconsistent configuration state from the config store
// and produces a consistent snapshot.
// Controller must not panic on configuration problems, it should issues a warning and continue.
type Controller struct {
	// Static information
	adapterInfo            map[string]*adapter.BuilderInfo // maps adapter shortName to Info.
	templateInfo           map[string]template.Info        // maps template name to Info.
	eval                   expr.Evaluator                  // Used to infer types. Used by resolver and dispatcher.
	attrDescFinder         expr.AttributeDescriptorFinder  // used by typeChecker
	identityAttribute      string                          // used by resolver
	defaultConfigNamespace string                          // used by resolver

	// configState is the current view of config
	// It receives updates from the underlying config store.
	// `Data` could be in an inconsistent state.
	configState map[store.Key]proto.Message

	// currently deployed resolver
	resolver *resolver

	// tableCache maps resolverId to handler table.
	//tableCache map[int]map[string]*HandlerEntry
	// table is the handler state currently in use.
	table map[string]*HandlerEntry

	// dispatcher is notified of changes.
	dispatcher ResolverChangeListener

	// handlerPool is the goroutine pool used by handler.
	handlerPool *pool.GoroutinePool

	// resolverID to be assigned to the next resolver.
	// It is incremented every call.
	resolverID int
}

// ResolverChangeListener is notified when a new resolver is created due to config change.
type ResolverChangeListener interface {
	ChangeResolver(rt Resolver)
}

// RulesKind defines the config kind name of mixer rules.
const RulesKind = "mixer-rules"

// New creates a new runtime Dispatcher
// Create a new controller and a dispatcher.
// Returns a ready to use dispatcher.
func New(eval expr.Evaluator, gp *pool.GoroutinePool, handlerPool *pool.GoroutinePool,
	identityAttribute string, defaultConfigNamespace string,
	s Store, adapterInfo map[string]*adapter.BuilderInfo,
	templateInfo map[string]template.Info, attrDescFinder expr.AttributeDescriptorFinder) (Dispatcher, error) {
	// controller will set Resolver before the dispatcher is used.
	d := newDispatcher(eval, nil, gp)
	err := startController(s, adapterInfo, templateInfo, eval, attrDescFinder, d,
		identityAttribute, defaultConfigNamespace, handlerPool)

	return d, err
}

// KindMap generates a map from object kind to its proto message.
func KindMap(adapterInfo map[string]*adapter.BuilderInfo,
	templateInfo map[string]template.Info) map[string]proto.Message {
	kindMap := make(map[string]proto.Message)
	// typed instances
	for kind, info := range templateInfo {
		kindMap[kind] = info.CtrCfg
	}
	// typed handlers
	for kind, info := range adapterInfo {
		kindMap[kind] = info.DefaultConfig
	}
	kindMap[RulesKind] = &cpb.Rule{}

	if glog.V(3) {
		glog.Info("kindMap = %v", kindMap)
	}
	return kindMap
}

// startWatch registers with store, initiates a watch, and returns the current config state.
func startWatch(s Store, adapterInfo map[string]*adapter.BuilderInfo,
	templateInfo map[string]template.Info) (map[store.Key]proto.Message, <-chan store.Event, error) {
	ctx := context.Background()
	kindMap := KindMap(adapterInfo, templateInfo)
	if err := s.Init(ctx, kindMap); err != nil {
		return nil, nil, err
	}
	// create channel before listing.
	watchChan, err := s.Watch(ctx)
	if err != nil {
		return nil, nil, err
	}
	return s.List(), watchChan, nil
}

// startController creates a controller from the given params.
func startController(s Store, adapterInfo map[string]*adapter.BuilderInfo,
	templateInfo map[string]template.Info, eval expr.Evaluator,
	attrDescFinder expr.AttributeDescriptorFinder, dispatcher ResolverChangeListener,
	identityAttribute string, defaultConfigNamespace string, handlerPool *pool.GoroutinePool) error {

	data, watchChan, err := startWatch(s, adapterInfo, templateInfo)
	if err != nil {
		return err
	}

	c := &Controller{
		adapterInfo:            adapterInfo,
		templateInfo:           templateInfo,
		eval:                   eval,
		attrDescFinder:         attrDescFinder,
		configState:            data,
		dispatcher:             dispatcher,
		resolver:               &resolver{}, // get an empty resolver
		identityAttribute:      identityAttribute,
		defaultConfigNamespace: defaultConfigNamespace,
		handlerPool:            handlerPool,
		table:                  make(map[string]*HandlerEntry),
	}

	c.publishSnapShot()
	glog.Info("Config controller has started with %d config elements", len(c.configState))
	go watchChanges(watchChan, c.applyEvents)
	return nil
}

type applyEvents func(events []*store.Event)

const maxEvents = 50

// watchChanges watches for changes and publishes a new resolver.
func watchChanges(wch <-chan store.Event, applyEvents applyEvents) {
	// consume changes and apply them to data indefinitely
	var timeChan <-chan time.Time
	var timer *time.Timer
	events := make([]*store.Event, 0, maxEvents)

	for {
		select {
		case ev := <-wch:
			if len(events) == 0 {
				timer = time.NewTimer(time.Second)
				timeChan = timer.C
			}
			events = append(events, &ev)
		case <-timeChan:
			timer.Stop()
			timeChan = nil
			glog.Info("Publishing %d events", len(events))
			applyEvents(events)
			events = events[:0]
		}
	}
}

func (c *Controller) applyEvents(events []*store.Event) {
	for _, ev := range events {
		switch ev.Type {
		case store.Update:
			c.configState[ev.Key] = ev.Value
		case store.Delete:
			delete(c.configState, ev.Key)
		}
	}
	c.publishSnapShot()
}

// publishSnapShot converts the currently available db into a resolver.
// The db may be in an inconsistent state, however it *must* be converted into a consistent resolver.
// The previous handler table enables connection re-use.
// This code is single threaded, it only runs on a config change.
func (c *Controller) publishSnapShot() {
	// current consistent view of handler configuration
	// keyed by Name.Kind.NameSpace
	handlerConfig := c.filterHandlerConfig()

	// current consistent view of the Instance configuration
	// keyed by Name.Kind.NameSpace
	instanceConfig := c.filterInstanceConfig()

	// new handler factory is created for every config change.
	hb := newHandlerFactory(c.templateInfo, c.eval, c.attrDescFinder, c.adapterInfo)

	// new handler table is created for every config change. It uses handler factory
	// to create new handlers.
	ht := newHandlerTable(instanceConfig, handlerConfig,
		func(handler *cpb.Handler, instances []*cpb.Instance) (adapter.Handler, error) {
			return hb.Build(handler, instances, newEnv(handler.Name, c.handlerPool))
		},
	)

	// current consistent view of the rules keyed by Namespace and then Name.
	// ht (handlerTable) keeps track of handler-instance association.
	ruleConfig := c.processRules(handlerConfig, instanceConfig, ht)

	// Initialize handlers that are used in the configuration.
	// Some handlers may not initialize due to errors.
	ht.Initialize(c.table)

	// Combine rules with the handler table.
	// Actions referring to handlers in error, will be purged.
	combineRulesHandlers(ruleConfig, ht.table)

	// create rules that are in the format that resolver needs.
	rules := convertToRuntimeRules(ruleConfig)

	// Create new resolver and cleanup the old resolver.
	c.resolverID++
	resolver := newResolver(c.eval, c.identityAttribute, c.defaultConfigNamespace, rules, c.resolverID)
	c.dispatcher.ChangeResolver(resolver)

	// copy old for deletion.
	oldTable := c.table
	oldResolver := c.resolver

	// set new
	c.table = ht.table
	c.resolver = resolver

	// synchronous call to cleanup.
	cleanupResolver(oldResolver, oldTable, maxCleanupDuration)

	glog.Infof("Published snapshot with %d config items", len(c.configState))
}

const maxCleanupDuration = 10 * time.Second

// filterInstanceConfig filters the configState and produces instanceConfig
// that points to valid templates.
func (c *Controller) filterInstanceConfig() map[string]*cpb.Instance {
	instanceConfig := make(map[string]*cpb.Instance)

	// first pass get all the validated instance and handler references
	for k, cfg := range c.configState {
		if _, found := c.templateInfo[k.Kind]; !found {
			continue
		}
		instanceConfig[k.String()] = &cpb.Instance{
			Name:     k.Name,
			Template: k.Kind,
			Params:   cfg,
		}
	}
	return instanceConfig
}

// filterHandlerConfig filters the configState and produces handlerConfig
// that points to valid adapters.
func (c *Controller) filterHandlerConfig() map[string]*cpb.Handler {
	handlerConfig := make(map[string]*cpb.Handler)
	for k, cfg := range c.configState {
		if _, found := c.adapterInfo[k.Kind]; !found {
			continue
		}
		handlerConfig[k.String()] = &cpb.Handler{
			Name:    k.Name,
			Adapter: k.Kind,
			Params:  cfg,
		}
	}
	return handlerConfig
}

// convertToRuntimeRules converts internal rules to the format that resolver needs.
func convertToRuntimeRules(ruleConfig map[string]map[string]*Rule) map[string][]*Rule {
	// convert rules
	rules := make(map[string][]*Rule)
	for ns, nsmap := range ruleConfig {
		rulesArr := make([]*Rule, 0, len(nsmap))
		for _, rule := range nsmap {
			rulesArr = append(rulesArr, rule)
		}
		rules[ns] = rulesArr
	}
	return rules
}

// processRules builds the current consistent view of the rules keyed by Namespace and then Name.
// ht (handlerTable) keeps track of handler-instance association.
func (c *Controller) processRules(handlerConfig map[string]*cpb.Handler,
	instanceConfig map[string]*cpb.Instance, ht *handlerTable) map[string]map[string]*Rule {
	// current consistent view of the rules
	// keyed by Namespace and then Name.
	ruleConfig := make(map[string]map[string]*Rule)

	// check rules and ensure only good handlers and instances are used.
	// record handler - instance associations
	for k, cfg := range c.configState {
		if k.Kind != RulesKind {
			continue
		}
		rulec := cfg.(*cpb.Rule)
		rule := &Rule{
			selector: rulec.Selector,
			name:     k.Name,
		}
		acts := c.processActions(rulec.Actions, handlerConfig, instanceConfig, ht)

		rn := ruleConfig[k.Namespace]
		if rn == nil {
			rn = make(map[string]*Rule)
			ruleConfig[k.Namespace] = rn
		}
		ruleActions := make(map[adptTmpl.TemplateVariety][]*Action)
		for vr, amap := range acts {
			for _, cf := range amap {
				ruleActions[vr] = append(ruleActions[vr], cf)
			}
		}
		rule.actions = ruleActions
		rn[k.Name] = rule
	}

	return ruleConfig
}

const cleanupSleepTime = 500 * time.Millisecond

// cleanupResolver cleans up handler table in the resolver
// after the resolver is no longer in use.
func cleanupResolver(r *resolver, table map[string]*HandlerEntry, timeout time.Duration) {
	start := time.Now()
	for {
		rc := atomic.LoadInt32(&r.refCount)
		if rc != 0 {
			if time.Since(start) > timeout {
				glog.Warningf("Unable to cleanup resolver in %v time. %d references remain", timeout, rc)
				return
			}
			if glog.V(2) {
				glog.Infof("Waiting for resolver %d %s to finish", r.ID, rc)
			}
			time.Sleep(cleanupSleepTime)
			continue
		}
		if glog.V(2) {
			glog.Infof("resolver %d handler table has %d entries", r.ID, len(table))
		}
		for _, he := range table {
			if he.notInUse && he.Handler != nil {
				msg := fmt.Sprintf("closing %s/%v", he.Name, he.Handler)
				err := he.Handler.Close()
				if err != nil {
					glog.Warningf("Error "+msg+": %s", err)
				} else {
					glog.Info(msg)
				}
			}
		}
		return
	}
}

// processActions prunes actions that lack referential integrity and associate instances with
// handlers that are later used to create new handlers.
func (c *Controller) processActions(acts []*cpb.Action, handlerConfig map[string]*cpb.Handler,
	instanceConfig map[string]*cpb.Instance, ht *handlerTable) map[adptTmpl.TemplateVariety]map[string]*Action {
	actions := make(map[adptTmpl.TemplateVariety]map[string]*Action)
	for _, ic := range acts {
		var hc *cpb.Handler
		if hc = handlerConfig[ic.Handler]; hc == nil {
			if glog.V(3) {
				glog.Warningf("ConfigWarning unknown handler: %s", ic.Handler)
			}
			continue
		}

		for _, instName := range ic.Instances {
			var inst *cpb.Instance
			if inst = instanceConfig[instName]; inst == nil {
				if glog.V(3) {
					glog.Warningf("ConfigWarning unknown instance: %s", instName)
				}
				continue
			}

			ht.Associate(ic.Handler, instName)

			ti := c.templateInfo[inst.Template]
			// grouped by adapterName.
			vAction := actions[ti.Variety]
			if vAction == nil {
				vAction = make(map[string]*Action)
				actions[ti.Variety] = vAction
			}

			act := vAction[hc.Adapter]
			if act == nil {
				act = &Action{
					processor:   &ti,
					handlerName: ic.Handler,
					adapterName: hc.Adapter,
				}
				vAction[hc.Adapter] = act
			}
			act.instanceConfig = append(act.instanceConfig, inst)
		}
	}
	return actions
}

// combineRulesHandlers set handler references into rulesConfig.
// Filter actions from rulesConfig whose adapter could not be initialized.
func combineRulesHandlers(ruleConfig map[string]map[string]*Rule, handlerTable map[string]*HandlerEntry) {
	// map by namespace
	for ns, nsmap := range ruleConfig {
		// map by rule name
		for rn, rule := range nsmap {
			// map by template variety
			for vr, vact := range rule.actions {
				newvact := vact[:0]
				for _, act := range vact {
					he := handlerTable[act.handlerName]
					if he == nil {
						glog.Warningf("Internal error: Handler %s could not be found", act.handlerName)
						continue
					}
					if he.Handler == nil {
						glog.Warningf("Filtering rule %s/%s. Handler %s could not be initialized due to %s.", ns, rn, act.handlerName, he.HandlerCreateError)
						continue
					}
					act.handler = he.Handler
					newvact = append(newvact, act)
				}
				rule.actions[vr] = newvact
			}
		}
	}
}
