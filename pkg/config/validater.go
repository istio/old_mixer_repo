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
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v2"

	"istio.io/mixer/pkg/aspect"
	"istio.io/mixer/pkg/expr"
)

type (
	// ValidaterFinder is used to find specific underlying validaters
	// Manager registry and adapter registry should implement this interface
	// so ConfigValidaters can be uniformly accessed
	ValidaterFinder interface {
		Find(name string) (aspect.ConfigValidater, bool)
	}
)

// NewValidater returns a validater given component validaters
func NewValidater(managerFinder ValidaterFinder, adapterFinder ValidaterFinder, strict bool, exprValidater expr.Validater) *Validater {
	return &Validater{
		managerFinder: managerFinder,
		adapterFinder: adapterFinder,
		strict:        strict,
		exprValidater: exprValidater,
		validated:     &Validated{},
	}
}

type (
	// Validater is the Configuration validator
	Validater struct {
		managerFinder ValidaterFinder
		adapterFinder ValidaterFinder
		strict        bool
		exprValidater expr.Validater
		validated     *Validated
	}

	// Validated store validated configuration
	// It has been validated as internally consistent and correct
	Validated struct {
		// Names are given to specific adapter configurations: unique
		adapterByName map[string]*Adapter
		// This is more often used to match adapters
		adapterByKind map[string][]*Adapter
		serviceConfig *ServiceConfig
		numAspects    int
	}
)

// ValidateAdapterConfig consumes a yml config string with adapter config
// It is validated in presence of
func (p *Validater) validateGlobalConfig(cfg string) (ce *aspect.ConfigErrors) {
	m := &GlobalConfig{}
	err := yaml.Unmarshal([]byte(cfg), m)
	if err != nil {
		ce = ce.Append("AdapterConfig", err)
		return
	}
	p.validated.adapterByKind = make(map[string][]*Adapter)
	p.validated.adapterByName = make(map[string]*Adapter)
	var acfg proto.Message
	var aArr []*Adapter
	var found bool
	for _, aa := range m.GetAdapters() {
		if acfg, err = ConvertParams(p.adapterFinder, aa.Impl, aa.Params, p.strict); err != nil {
			ce = ce.Append("Adapter: "+aa.Impl, err)
			continue
		}
		aa.Params = acfg

		if aArr, found = p.validated.adapterByKind[aa.GetKind()]; !found {
			aArr = []*Adapter{}
		}
		aArr = append(aArr, aa)
		p.validated.adapterByKind[aa.GetKind()] = aArr
		if aa.GetName() != "" {
			p.validated.adapterByName[aa.GetName()] = aa
		}

	}
	return
}

// ValidateSelector ensures that the selector is valid per expression language
func (p *Validater) validateSelector(selector string) (err error) {
	if len(selector) == 0 {
		return fmt.Errorf("empty selector not allowed")
	}
	return p.exprValidater.Validate(selector)
}

// ValidateRules  processes Aspect Rules
func (p *Validater) ValidateRules(rules []*AspectRule, path string, validatePresence bool) (ce *aspect.ConfigErrors) {
	var acfg proto.Message
	var err error
	for _, rule := range rules {
		if err = p.validateSelector(rule.GetSelector()); err != nil {
			ce = ce.Append(path+":Selector "+rule.GetSelector(), err)
		}
		path = path + "/" + rule.GetSelector()
		for idx, aa := range rule.GetAspects() {
			if acfg, err = ConvertParams(p.managerFinder, aa.GetKind(), aa.GetParams(), p.strict); err != nil {
				ce = ce.Append(fmt.Sprintf("%s:%s[%d]", path, aa.Kind, idx), err)
				continue
			}
			aa.Params = acfg
			p.validated.numAspects++
			if validatePresence {
				// ensure that aa.Kind has a registered adapter
				if p.validated.adapterByKind[aa.GetKind()] == nil {
					ce = ce.Appendf("Kind", "No adapter for Kind=%s is available", aa.GetKind())
				}
				if aa.GetAdapter() != "" && p.validated.adapterByName[aa.GetAdapter()] == nil {
					ce = ce.Appendf("NamedAdapter", "No adapter by name %s Available", aa.GetAdapter())
				}
			}
		}
		rs := rule.GetRules()
		if len(rs) == 0 {
			continue
		}
		if verr := p.ValidateRules(rs, path, validatePresence); verr != nil {
			ce = ce.Extend(verr)
		}
	}
	return ce
}

// Validate a single serviceConfig and globalConfig together
// returns a fully validated runtimeConfig if no errors are found
func (p *Validater) Validate(serviceCfg string, globalCfg string) (rt *Validated, ce *aspect.ConfigErrors) {

	if re := p.validateGlobalConfig(globalCfg); re != nil {
		return rt, ce.Append("GlobalConfig", re)
	}
	// The order is important here, because serviceConfig refers to global config

	if re := p.validateServiceConfig(serviceCfg, true); re != nil {
		return rt, ce.Append("ServiceConfig", re)
	}

	return p.validated, nil
}

// ValidateServiceConfig validates service config
// if validatePresence is true it will ensure that the named adapter and Kinds
// have an available and configured adapter
func (p *Validater) validateServiceConfig(cfg string, validatePresence bool) (ce *aspect.ConfigErrors) {
	m := &ServiceConfig{}
	err := yaml.Unmarshal([]byte(cfg), m)
	if err != nil {
		ce = ce.Append("ServiceConfig", err)
		return
	}
	if ce = p.ValidateRules(m.GetRules(), "", validatePresence); ce != nil {
		return ce
	}
	p.validated.serviceConfig = m
	return
}

// UnknownValidater returns error for the given name
func UnknownValidater(name string) error {
	return fmt.Errorf("unknown type [%s]", name)
}

// ConvertParams converts returns a typed protomessage based on available Validater
func ConvertParams(finder ValidaterFinder, name string, params interface{}, strict bool) (proto.Message, error) {
	avl, found := finder.Find(name)
	if !found {
		return nil, UnknownValidater(name)
	}

	acfg := avl.DefaultConfig()
	if err := Decode(params, acfg, strict); err != nil {
		return nil, err
	}
	if verr := avl.ValidateConfig(acfg); verr != nil {
		return nil, verr
	}
	return acfg, nil
}

// Decode interprets src interface{} as the specified proto message
func Decode(src interface{}, dest proto.Message, strict bool) (err error) {
	var md mapstructure.Metadata
	mcfg := mapstructure.DecoderConfig{
		Metadata: &md,
		Result:   dest,
	}
	var decoder *mapstructure.Decoder
	if decoder, err = mapstructure.NewDecoder(&mcfg); err != nil {
		return err
	}
	if err = decoder.Decode(src); err == nil && strict && len(md.Unused) > 0 {
		return fmt.Errorf("unused fields while parsing %s", md.Unused)
	}

	return err
}
