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
	"fmt"
	"sync/atomic"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/glog"

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

	// table is the handler state currently in use.
	table map[string]*HandlerEntry

	// dispatcher is notified of changes.
	dispatcher ResolverChangeListener

	// handlerPool is the goroutine pool used by handler.
	handlerPool *pool.GoroutinePool

	// resolverID to be assigned to the next resolver.
	// It is incremented every call.
	resolverID int

	// Fields below are used for testing.

	// createHandlerFactory for testing.
	createHandlerFactory factoryCreatorFunc
}

// RulesKind defines the config kind name of mixer rules.
const RulesKind = "mixer-rules"

// ResolverChangeListener is notified when a new resolver is created due to config change.
type ResolverChangeListener interface {
	ChangeResolver(rt Resolver)
}

// factoryCreatorFunc creates a handler factory. It is used for testing.
type factoryCreatorFunc func(templateInfo map[string]template.Info, expr expr.TypeChecker,
	df expr.AttributeDescriptorFinder, builderInfo map[string]*adapter.BuilderInfo) HandlerFactory

// applyEventsFn is used for testing
type applyEventsFn func(events []*store.Event)

// maxEvents is the likely maximum number of events
// we can expect in a second. It is used to avoid slice reallocation.
const maxEvents = 50

// publishSnapShot converts the currently available configState into a resolver.
// The config may be in an inconsistent state, however it *must* be converted into a consistent resolver.
// The previous handler table enables handler cleanup and reuse.
// This code is single threaded, it only runs on a config change control loop.
func (c *Controller) publishSnapShot() {
	// current consistent view of handler configuration
	// keyed by Name.Kind.NameSpace
	handlerConfig := c.validHandlerConfigs()

	// current consistent view of the Instance configuration
	// keyed by Name.Kind.NameSpace
	instanceConfig := c.validInstanceConfigs()

	// new handler factory is created for every config change.
	hb := c.createHandlerFactory(c.templateInfo, c.eval, c.attrDescFinder, c.adapterInfo)

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
	rules, nrules := convertToRuntimeRules(ruleConfig)

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

	glog.Infof("Published snapshot[%d] with %d rules, %d handlers", resolver.ID, nrules, len(c.table))

	// synchronous call to cleanup.
	err := cleanupResolver(oldResolver, oldTable, maxCleanupDuration)
	if err != nil {
		glog.Warningf("Unable to perform cleanup: %v", err)
	}

}

// maxCleanupDuration is the maximum amount of time cleanup operation will wait
// before resolver ref count does to 0. It will return after this duration without
// calling Close() on handlers.
var maxCleanupDuration = 10 * time.Second

var watchFlushDuration = time.Second

// watchChanges watches for changes and publishes a new resolver.
// watchChanges is started in a goroutine.
func watchChanges(wch <-chan store.Event, applyEvents applyEventsFn) {
	// consume changes and apply them to data indefinitely
	var timeChan <-chan time.Time
	var timer *time.Timer
	events := make([]*store.Event, 0, maxEvents)

	for {
		select {
		case ev := <-wch:
			if len(events) == 0 {
				timer = time.NewTimer(watchFlushDuration)
				timeChan = timer.C
			}
			events = append(events, &ev)
		case <-timeChan:
			timer.Stop()
			timeChan = nil
			glog.Infof("Publishing %d events", len(events))
			applyEvents(events)
			events = events[:0]
		}
	}
}

// applyEvents applies given events to config state and then publishes a snapshot.
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

// validInstanceConfigs returns instanceConfigs from the configState that
// point to valid templates.
func (c *Controller) validInstanceConfigs() map[string]*cpb.Instance {
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

// validHandlerConfigs returns handlerConfigs from the configState that
// point to valid adapters.
func (c *Controller) validHandlerConfigs() map[string]*cpb.Handler {
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

type rulesByName map[string]*Rule

// rulesByName indexed by namespace
type rulesMapByNamespace map[string]rulesByName

// rulesListByNamespace is the type needed by resolver.
type rulesListByNamespace map[string][]*Rule

// convertToRuntimeRules converts internal rules to the format that resolver needs.
func convertToRuntimeRules(ruleConfig rulesMapByNamespace) (rulesListByNamespace, int) {
	// convert rules
	nrules := 0
	rules := make(rulesListByNamespace)
	for ns, nsmap := range ruleConfig {
		rulesArr := make([]*Rule, 0, len(nsmap))
		for _, rule := range nsmap {
			rulesArr = append(rulesArr, rule)
		}
		rules[ns] = rulesArr
		nrules += len(rulesArr)
	}
	return rules, nrules
}

// processRules builds the current consistent view of the rules keyed by Namespace and then Name.
// ht (handlerTable) keeps track of handler-instance association.
func (c *Controller) processRules(handlerConfig map[string]*cpb.Handler,
	instanceConfig map[string]*cpb.Instance, ht *handlerTable) rulesMapByNamespace {
	// current consistent view of the rules
	// keyed by Namespace and then Name.
	ruleConfig := make(rulesMapByNamespace)

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

var cleanupSleepTime = 500 * time.Millisecond

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

// combineRulesHandlers sets handler references in rulesConfig.
// Reject actions from rulesConfig whose handler could not be initialized.
func combineRulesHandlers(ruleConfig rulesMapByNamespace, handlerTable map[string]*HandlerEntry) {
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
						glog.Warningf("Filtering action from rule %s/%s. Handler %s could not be initialized due to %s.", ns, rn, act.handlerName, he.HandlerCreateError)
						continue
					}
					act.handler = he.Handler
					newvact = append(newvact, act)
				}
				if len(newvact) > 0 {
					rule.actions[vr] = newvact
				} else {
					delete(rule.actions, vr)
				}
			}
			if len(rule.actions) == 0 {
				glog.Warningf("Purging rule %v with no actions")
				delete(nsmap, rn)
			}
		}
	}
}

// cleanupResolver cleans up handler table in the resolver
// after the resolver is no longer in use.
func cleanupResolver(r *resolver, table map[string]*HandlerEntry, timeout time.Duration) error {
	start := time.Now()
	for {
		rc := atomic.LoadInt32(&r.refCount)
		if rc > 0 {
			if time.Since(start) > timeout {
				return fmt.Errorf("unable to cleanup resolver in %v time. %d requests remain", timeout, rc)
			}
			if glog.V(2) {
				glog.Infof("Waiting for resolver %d to finish %d remaining requests", r.ID, rc)
			}
			time.Sleep(cleanupSleepTime)
			continue
		}
		if glog.V(2) {
			glog.Infof("cleanupResolver[%d] handler table has %d entries", r.ID, len(table))
		}
		for _, he := range table {
			if he.closeOnCleanup && he.Handler != nil {
				msg := fmt.Sprintf("closing %s/%v", he.Name, he.Handler)
				err := he.Handler.Close()
				if err != nil {
					glog.Warningf("Error "+msg+": %s", err)
				} else {
					glog.Info(msg)
				}
			}
		}
		return nil
	}
}
