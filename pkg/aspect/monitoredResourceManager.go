// Copyright 2017 the Istio Authors.
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

package aspect

import (
	"fmt"

	dpb "istio.io/api/mixer/v1/config/descriptor"
	"istio.io/mixer/pkg/adapter"
	aconfig "istio.io/mixer/pkg/aspect/config"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/config"
	"istio.io/mixer/pkg/config/descriptor"
	"istio.io/mixer/pkg/expr"
)

type (
	// MonitoredResourceFinder describes the ability to produce a MonitoredResource given a name and the
	// ability to evaluate expressions in a context.
	MonitoredResourceFinder interface {
		Find(name string, attrs attribute.Bag, eval expr.Evaluator) (*adapter.MonitoredResource, error)
	}

	mrInfo struct {
		nameExpr string
		labels   map[string]string
	}

	mrProvider struct {
		info map[string]mrInfo
	}
)

// NewMRFinder produces a MonitoredResourceFinder given a set of aspect level monitored resources.
func NewMRFinder(params *aconfig.MonitoredResources) MonitoredResourceFinder {
	byName := make(map[string]mrInfo, len(params.Resources))
	for _, mr := range params.Resources {
		byName[mr.DescriptorName] = mrInfo{
			nameExpr: mr.NameExpression,
			labels:   mr.Labels,
		}
	}
	return &mrProvider{byName}
}

func (m *mrProvider) Find(name string, attrs attribute.Bag, eval expr.Evaluator) (*adapter.MonitoredResource, error) {
	info, found := m.info[name]
	if !found {
		return nil, fmt.Errorf("could not find unknown monitored resource descriptor '%s'", name)
	}
	concreteName, err := eval.Eval(info.nameExpr, attrs)
	if err != nil {
		return nil, err
	}
	// Validated ensures info.nameExpr has type STRING
	nameString, _ := concreteName.(string)

	labels, err := evalAll(info.labels, attrs, eval)
	if err != nil {
		return nil, err
	}
	return &adapter.MonitoredResource{
		Name:   nameString,
		Labels: labels,
	}, nil
}

// TODO: wire into pkg/config; we need to figure out how this will work since MRs are different than other aspect kinds.
func (*mrProvider) ValidateConfig(c config.AspectParams, v expr.Validator, df descriptor.Finder) (ce *adapter.ConfigErrors) {
	cfg := c.(*aconfig.MonitoredResources)
	for idx, mr := range cfg.Resources {
		if err := v.AssertType(mr.NameExpression, df, dpb.STRING); err != nil {
			ce = ce.Append(fmt.Sprintf("MonitoredResources.Resources[%d].NameExpression", idx), err)
		}

		desc := df.GetMonitoredResource(mr.DescriptorName)
		if desc == nil {
			ce = ce.Appendf(
				fmt.Sprintf("MonitoredResources.Resources[%d].DescriptorName", idx),
				"could not find a MonitoredResourceDescriptor named '%s'", mr.DescriptorName)
			continue
		}
		ce = ce.Extend(validateLabels(fmt.Sprintf("MonitoredResources.Resources[%d].Labels", idx), mr.Labels, desc.Labels, v, df))
	}
	return
}
