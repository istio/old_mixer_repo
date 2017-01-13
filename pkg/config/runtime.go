// Copyright 2017 Google Inc.
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
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/expr"
)

type (
	// Runtime Represents the runtime view of the config
	// It is prevalidated and immutable.
	// It can be safely dispatched
	Runtime struct {
		Validated
		// used to evaluate selectors
		evaluator expr.PredicateEvaluator
	}
	// Combined config is given to aspect managers
	Combined struct {
		Adapter *Adapter
		Aspect  *Aspect
	}

	// AspectSet is a set of aspects. ex: Check call will result in {"listChecker", "iam"}
	// Runtime should only return aspects matching a certain type
	AspectSet map[string]bool
)

// NewRuntime returns a Runtime object given a validated config and a predicate evaluator
func NewRuntime(v *Validated, evaluator expr.PredicateEvaluator) *Runtime {
	return &Runtime{
		Validated: *v,
		evaluator: evaluator,
	}
}

// Resolve returns a list of CombinedConfig  given an attribute bag
// It will only return config from the requested set of aspects
// For example the Check handler and Report handler will request
// a disjoint set of aspects check: {iplistChecker, iam}, report: {Log, metrics}
func (p *Runtime) Resolve(bag attribute.Bag, aspectSet AspectSet) ([]*Combined, error) {
	dlist := make([]*Combined, 0, p.numAspects)
	err := p.resolveRules(bag, aspectSet, p.serviceConfig.GetRules(), "/", &dlist)
	return dlist, err
}

func (p *Runtime) resolveRules(bag attribute.Bag, aspectSet AspectSet, rules []*AspectRule, path string, dlist *[]*Combined) (err error) {

	var selected bool

	for _, rule := range rules {
		if selected, err = p.evaluator.EvalPredicate(rule.GetSelector(), bag); err != nil {
			return err
		}

		if !selected {
			continue
		}
		path = path + "/" + rule.GetSelector()
		for _, aa := range rule.GetAspects() {
			var adp *Adapter
			if aspectSet[aa.GetKind()] {
				// find matching adapter
				// assume that config references are correct
				if aa.GetAdapter() != "" {
					adp = p.adapterByName[aa.GetAdapter()]
				} else {
					adp = p.adapterByKind[aa.GetKind()][0]
				}
				*dlist = append(*dlist, &Combined{
					Adapter: adp,
					Aspect:  aa,
				})
			}
		}
		rs := rule.GetRules()
		if len(rs) == 0 {
			continue
		}
		if err = p.resolveRules(bag, aspectSet, rs, path, dlist); err != nil {
			return err
		}
	}
	return nil
}
