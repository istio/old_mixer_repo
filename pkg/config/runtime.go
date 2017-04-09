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
	"fmt"
	"strings"

	"github.com/golang/glog"
	multierror "github.com/hashicorp/go-multierror"

	"istio.io/mixer/pkg/attribute"
	pb "istio.io/mixer/pkg/config/proto"
	"istio.io/mixer/pkg/expr"
)

type (
	// runtime represents the runtime view of the config.
	// It is pre-validated and immutable.
	// It can be safely used concurrently.
	runtime struct {
		Validated
		// used to evaluate selectors
		eval expr.PredicateEvaluator
	}
)

// newRuntime returns a runtime object given a validated config and a predicate eval.
func newRuntime(v *Validated, evaluator expr.PredicateEvaluator) *runtime {
	return &runtime{
		Validated: *v,
		eval:      evaluator,
	}
}

// GetScopes returns configuration scopes that apply given a target service
func GetScopes(target string) ([]string, error) {
	return getK8sScopes(target)
}

// K8S_MAX_SPLIT how many components does a k8s svc have
const k8sMaSplit = 2

// getK8sScopes return k8s scopes
// my-svc.my-namespace.svc.cluster.local --> [ global, my-namespace, my-svc ]
// This is k8s specific, it should be made configurable
func getK8sScopes(target string) ([]string, error) {
	comps := strings.SplitN(target, ".", k8sMaSplit)
	if len(comps) < 2 {
		return nil, fmt.Errorf("target not valid %s, must be of type my-svc.my-namespace.svc.cluster.local", target)
	}
	svc, ns := comps[0], comps[1]
	return []string{"global", ns, svc + "." + ns}, nil
}

// resolve returns a list of CombinedConfig given an attribute bag.
// It will only return config from the requested set of aspects.
// For example the Check handler and Report handler will request
// a disjoint set of aspects check: {iplistChecker, iam}, report: {Log, metrics}
func (r *runtime) Resolve(bag attribute.Bag, set KindSet) (dlist []*pb.Combined, err error) {
	if glog.V(2) {
		glog.Infof("resolving for kinds: %s", set)
		defer func() { glog.Infof("resolved configs (err=%v): %s", err, dlist) }()
	}
	return resolve(bag, set, r.policy, r.resolveRules, false /* conditional full resolve */)
}

// ResolveUnconditional returns the list of CombinedConfigs for the supplied
// attributes and kindset based on resolution of unconditional rules. That is,
// it only attempts to find aspects in rules that have an empty selector. This
// method is primarily used for pre-process aspect configuration retrieval.
func (r *runtime) ResolveUnconditional(bag attribute.Bag, set KindSet) (out []*pb.Combined, err error) {
	return resolve(bag, set, r.policy, r.resolveRules, true /* unconditional resolve */)
}

const ktargetService = "target.service"

// Make this a reasonable number so that we don't reallocate slices often.
const resolveSize = 50

func resolve(bag attribute.Bag, kindSet KindSet, policy map[Key]*pb.ServiceConfig, resolveRules resolveRulesFunc, onlyEmptySelectors bool) (dlist []*pb.Combined, err error) {
	var scopes []string
	if glog.V(2) {
		glog.Infof("unconditionally resolving for kinds: %s", kindSet)
		defer func() { glog.Infof("unconditionally resolved configs (err=%v): %s", err, dlist) }()
	}

	target, _ := bag.Get(ktargetService)
	if target == nil {
		return nil, fmt.Errorf("%s attribute not found", ktargetService)
	}

	if scopes, err = GetScopes(target.(string)); err != nil {
		return nil, err
	}

	dlist = make([]*pb.Combined, 0, resolveSize)
	dlistout := make([]*pb.Combined, 0, resolveSize)

	for idx := 0; idx < len(scopes); idx++ {
		scope := scopes[idx]
		amap := make(map[string][]*pb.Combined)

		for j := idx; j < len(scopes); j++ {
			subject := scopes[j]
			key := Key{scope, subject}
			pol := policy[key]
			if pol == nil {
				continue
			}
			// empty the slice, do not re allocate
			dlist = dlist[:0]
			if dlist, err = resolveRules(bag, kindSet, pol.GetRules(), "/", dlist, onlyEmptySelectors); err != nil {
				return dlist, err
			}

			aamap := make(map[string][]*pb.Combined)

			for _, d := range dlist {
				aamap[d.Aspect.Kind] = append(aamap[d.Aspect.Kind], d)
			}

			// more specific subject replaces
			for k, v := range aamap {
				amap[k] = v
			}
		}
		// collapse from amap
		for _, v := range amap {
			dlistout = append(dlistout, v...)
		}
	}

	return dlistout, nil
}

func (r *runtime) evalPredicate(selector string, bag attribute.Bag) (bool, error) {
	// empty selector always selects
	if selector == "" {
		return true, nil
	}
	return r.eval.EvalPredicate(selector, bag)
}

type resolveRulesFunc func(bag attribute.Bag, kindSet KindSet, rules []*pb.AspectRule, path string, dlist []*pb.Combined, onlyEmptySelectors bool) ([]*pb.Combined, error)

// resolveRules recurses through the config struct and returns a list of combined aspects
func (r *runtime) resolveRules(bag attribute.Bag, kindSet KindSet, rules []*pb.AspectRule, path string, dlist []*pb.Combined, onlyEmptySelectors bool) ([]*pb.Combined, error) {
	var selected bool
	var lerr error
	var err error

	for _, rule := range rules {
		// We write detailed logs about a single rule into a string rather than printing as we go to ensure
		// they're all in a single log line and not interleaved with logs from other requests.
		logMsg := ""
		if glog.V(3) {
			glog.Infof("resolveRules (%v) ==> %v ", rule, path)
		}

		sel := rule.GetSelector()
		if sel != "" && onlyEmptySelectors {
			continue
		}
		if selected, lerr = r.evalPredicate(sel, bag); lerr != nil {
			err = multierror.Append(err, lerr)
			continue
		}
		if !selected {
			continue
		}
		if glog.V(3) {
			logMsg = fmt.Sprintf("Rule (%v) applies, using aspect kinds %v:", rule.GetSelector(), kindSet)
		}

		path = path + "/" + sel
		for _, aa := range rule.GetAspects() {
			k, ok := ParseKind(aa.Kind)
			if !ok || !kindSet.IsSet(k) {
				if glog.V(3) {
					logMsg = fmt.Sprintf("%s\n- did not select aspect kind %s named '%s', wrong kind", logMsg, aa.Kind, aa.Adapter)
				}
				continue
			}
			adp := r.adapterByName[adapterKey{k, aa.Adapter}]
			if glog.V(3) {
				logMsg = fmt.Sprintf("%s\n- selected aspect kind %s with config: %v", logMsg, aa.Kind, adp)
			}
			dlist = append(dlist, &pb.Combined{Builder: adp, Aspect: aa})
		}

		if glog.V(3) {
			glog.Info(logMsg)
		}

		rs := rule.GetRules()
		if len(rs) == 0 {
			continue
		}
		if dlist, lerr = r.resolveRules(bag, kindSet, rs, path, dlist, onlyEmptySelectors); lerr != nil {
			err = multierror.Append(err, lerr)
		}
	}
	return dlist, err
}
