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

	multierror "github.com/hashicorp/go-multierror"
	pb "istio.io/mixer/pkg/config/proto"
)

type (
	// Runtime represents the runtime view of the config.
	// It is pre-validated and immutable.
	// It can be safely used concurrently.
	Runtime struct {
		Validated
		// used to evaluate selectors
		eval expr.PredicateEvaluator
	}
	// Combined config is given to aspect managers.
	Combined struct {
		Adapter *pb.Adapter
		Aspect  *pb.Aspect
	}

	// AspectSet is a set of aspects. ex: Check call will result in {"listChecker", "iam"}
	// Runtime should only return aspects matching a certain type.
	AspectSet map[string]bool
)

// NewRuntime returns a Runtime object given a validated config and a predicate eval.
func NewRuntime(v *Validated, evaluator expr.PredicateEvaluator) *Runtime {
	return &Runtime{
		Validated: *v,
		eval:      evaluator,
	}
}

// Resolve returns a list of CombinedConfig given an attribute bag.
// It will only return config from the requested set of aspects.
// For example the Check handler and Report handler will request
// a disjoint set of aspects check: {iplistChecker, iam}, report: {Log, metrics}
func (p *Runtime) Resolve(bag attribute.Bag, aspectSet AspectSet) ([]*Combined, error) {
	dlist := make([]*Combined, 0, p.numAspects)
	err := p.resolveRules(bag, aspectSet, p.serviceConfig.GetRules(), "/", &dlist)
	return dlist, err
}

func (p *Runtime) resolveRules(bag attribute.Bag, aspectSet AspectSet, rules []*pb.AspectRule, path string, dlist *[]*Combined) (err error) {

	var selected bool
	var lerr error

	for _, rule := range rules {
		if selected, lerr = p.eval.EvalPredicate(rule.GetSelector(), bag); lerr != nil {
			err = multierror.Append(err, lerr)
			continue
		}

		if !selected {
			continue
		}
		path = path + "/" + rule.GetSelector()
		for _, aa := range rule.GetAspects() {
			var adp *pb.Adapter
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
		if lerr = p.resolveRules(bag, aspectSet, rs, path, dlist); lerr != nil {
			err = multierror.Append(err, lerr)
			continue
		}
	}
	return err
}
