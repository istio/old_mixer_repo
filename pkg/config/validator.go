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
	pb "istio.io/mixer/pkg/config/proto"
	"istio.io/mixer/pkg/expr"
)

type (
	// ValidatorFinder is used to find specific underlying validators.
	// Manager registry and adapter registry should implement this interface
	// so ConfigValidators can be uniformly accessed.
	ValidatorFinder interface {
		Find(name string) (aspect.ConfigValidator, bool)
	}
)

// NewValidator returns a validator given component validators.
func NewValidator(managerFinder ValidatorFinder, adapterFinder ValidatorFinder, strict bool, exprValidator expr.Validator) *Validator {
	return &Validator{
		managerFinder: managerFinder,
		adapterFinder: adapterFinder,
		strict:        strict,
		exprValidator: exprValidator,
		validated:     &Validated{},
	}
}

type (
	// Validator is the Configuration validator.
	Validator struct {
		managerFinder ValidatorFinder
		adapterFinder ValidatorFinder
		strict        bool
		exprValidator expr.Validator
		validated     *Validated
	}

	// Validated store validated configuration.
	// It has been validated as internally consistent and correct.
	Validated struct {
		// Names are given to specific adapter configurations: unique
		adapterByName map[string]*pb.Adapter
		// This is more often used to match adapters
		adapterByKind map[string][]*pb.Adapter
		serviceConfig *pb.ServiceConfig
		numAspects    int
	}
)

// validateAdapterConfig consumes a yml config string with adapter config.
// It is validated in presence of validators.
func (p *Validator) validateGlobalConfig(cfg string) (ce *aspect.ConfigErrors) {
	m := &pb.GlobalConfig{}
	err := yaml.Unmarshal([]byte(cfg), m)
	if err != nil {
		ce = ce.Append("AdapterConfig", err)
		return
	}
	p.validated.adapterByKind = make(map[string][]*pb.Adapter)
	p.validated.adapterByName = make(map[string]*pb.Adapter)
	var acfg proto.Message
	var aArr []*pb.Adapter
	var found bool
	for _, aa := range m.GetAdapters() {
		if acfg, err = ConvertParams(p.adapterFinder, aa.Impl, aa.Params, p.strict); err != nil {
			ce = ce.Append("Adapter: "+aa.Impl, err)
			continue
		}
		aa.Params = acfg

		if aArr, found = p.validated.adapterByKind[aa.GetKind()]; !found {
			aArr = []*pb.Adapter{}
		}
		aArr = append(aArr, aa)
		p.validated.adapterByKind[aa.GetKind()] = aArr
		if aa.GetName() != "" {
			p.validated.adapterByName[aa.GetName()] = aa
		}

	}
	return
}

// ValidateSelector ensures that the selector is valid per expression language.
func (p *Validator) validateSelector(selector string) (err error) {
	// empty selector always selects
	if len(selector) == 0 {
		return nil
	}
	return p.exprValidator.Validate(selector)
}

// validateAspectRules validates the recursive configuration data structure.
// It is primarily used by validate ServiceConfig.
func (p *Validator) validateAspectRules(rules []*pb.AspectRule, path string, validatePresence bool) (ce *aspect.ConfigErrors) {
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
					ce = ce.Appendf("Kind", "adapter for Kind=%s is not available", aa.GetKind())
				}
				if aa.GetAdapter() != "" && p.validated.adapterByName[aa.GetAdapter()] == nil {
					ce = ce.Appendf("NamedAdapter", "adapter by name %s not available", aa.GetAdapter())
				}
			}
		}
		rs := rule.GetRules()
		if len(rs) == 0 {
			continue
		}
		if verr := p.validateAspectRules(rs, path, validatePresence); verr != nil {
			ce = ce.Extend(verr)
		}
	}
	return ce
}

// Validate validates a single serviceConfig and globalConfig together.
// It returns a fully validated Config if no errors are found.
func (p *Validator) Validate(serviceCfg string, globalCfg string) (rt *Validated, ce *aspect.ConfigErrors) {
	var cerr *aspect.ConfigErrors
	if re := p.validateGlobalConfig(globalCfg); re != nil {
		cerr = ce.Appendf("GlobalConfig", "failed validation")
		return rt, cerr.Extend(re)
	}
	// The order is important here, because serviceConfig refers to global config

	if re := p.validateServiceConfig(serviceCfg, true); re != nil {
		cerr = ce.Appendf("ServiceConfig", "failed validation")
		return rt, cerr.Extend(re)
	}

	return p.validated, nil
}

// ValidateServiceConfig validates service config.
// if validatePresence is true it will ensure that the named adapter and Kinds
// have an available and configured adapter.
func (p *Validator) validateServiceConfig(cfg string, validatePresence bool) (ce *aspect.ConfigErrors) {
	m := &pb.ServiceConfig{}
	err := yaml.Unmarshal([]byte(cfg), m)
	if err != nil {
		ce = ce.Append("ServiceConfig", err)
		return
	}
	if ce = p.validateAspectRules(m.GetRules(), "", validatePresence); ce != nil {
		return ce
	}
	p.validated.serviceConfig = m
	return
}

// UnknownValidator returns error for the given name.
func UnknownValidator(name string) error {
	return fmt.Errorf("unknown type [%s]", name)
}

// ConvertParams converts returns a typed proto message based on available Validator.
func ConvertParams(finder ValidatorFinder, name string, params interface{}, strict bool) (proto.Message, error) {
	avl, found := finder.Find(name)
	if !found {
		return nil, UnknownValidator(name)
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

// Decode interprets src interface{} as the specified proto message.
func Decode(src interface{}, dest proto.Message, strict bool) (err error) {
	var md mapstructure.Metadata
	var decoder *mapstructure.Decoder
	if decoder, err = mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata: &md,
		Result:   dest,
	}); err != nil {
		return err
	}

	if err = decoder.Decode(src); err == nil && strict && len(md.Unused) > 0 {
		return fmt.Errorf("unused fields while parsing %s", md.Unused)
	}
	return err
}
