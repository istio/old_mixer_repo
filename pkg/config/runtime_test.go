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
	"testing"

	"istio.io/mixer/pkg/attribute"

	"fmt"

	mixerpb "istio.io/api/mixer/v1"
)

type trueEval struct {
	err    error
	ncalls int
	ret    bool
}

func (t *trueEval) EvalPredicate(expression string, attrs attribute.Bag) (bool, error) {
	if t.ncalls == 0 {
		return t.ret, t.err
	}
	t.ncalls--
	return true, nil
}

type ttable struct {
	err    error
	ncalls int
	ret    bool
	nlen   int
	nerr   int
	asp    []string
}

func TestRuntime(t *testing.T) {
	table := []*ttable{
		{nil, 0, true, 4, 0, []string{"listChecker"}},
		{nil, 1, false, 2, 0, []string{"listChecker"}},
		{fmt.Errorf("Predicate Error"), 1, false, 2, 0, []string{"listChecker"}},
		{nil, 0, true, 0, 0, []string{}},
		{fmt.Errorf("Predicate Error"), 0, true, 0, 0, []string{"listChecker"}},
	}

	LC := "listChecker"
	a1 := &Adapter{
		Name: "a1",
		Kind: LC,
	}
	a2 := &Adapter{
		Name: "a2",
		Kind: LC,
	}

	v := &Validated{
		adapterByName: map[string]*Adapter{
			"a1": a1, "a2": a2,
		},
		adapterByKind: map[string][]*Adapter{
			LC: {a1, a2},
		},
		serviceConfig: &ServiceConfig{
			Rules: []*AspectRule{
				{
					Selector: "ok",
					Aspects: []*Aspect{
						{
							Kind: LC,
						},
						{
							Adapter: "a2",
							Kind:    LC,
						},
					},
					Rules: []*AspectRule{
						{
							Selector: "ok",
							Aspects: []*Aspect{
								{
									Kind: LC,
								},
								{
									Adapter: "a2",
									Kind:    LC,
								},
							},
						},
					},
				},
			},
		},
		numAspects: 1,
	}

	bag, err := attribute.NewManager().NewTracker().StartRequest(&mixerpb.Attributes{})
	if err != nil {
		t.Error("Unable to get attribute bag")
	}

	for idx, tt := range table {
		fe := &trueEval{tt.err, tt.ncalls, tt.ret}
		aspects := make(map[string]bool)
		for _, a := range tt.asp {
			aspects[a] = true
		}
		rt := NewRuntime(v, fe)

		al, err := rt.Resolve(bag, aspects)
		if err != tt.err {
			t.Error(idx, err)
		}

		if len(al) != tt.nlen {
			t.Errorf("%d Expected %d resolve got %d", idx, tt.nlen, len(al))
		}
	}
}
