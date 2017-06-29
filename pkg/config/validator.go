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

// Package config handles configuration ingestion and processing.
// validator
// 1. Accepts new configuration from user
// 2. Validates configuration
// 3. Produces a "ValidatedConfig"
// runtime
// 1. It is validated and actionable configuration
// 2. It resolves the configuration to a list of Combined {aspect, adapter} configs
//    given an attribute.Bag.
// 3. Combined config has complete information needed to dispatch aspect
package config

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"

	dpb "istio.io/api/mixer/v1/config/descriptor"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/adapter/config"
	"istio.io/mixer/pkg/config/descriptor"
	pb "istio.io/mixer/pkg/config/proto"
	"istio.io/mixer/pkg/expr"
	"istio.io/mixer/pkg/template"
)

type (
	// AspectParams describes configuration parameters for an aspect.
	AspectParams proto.Message

	// AspectValidator describes a type that is able to validate Aspect configuration.
	AspectValidator interface {
		// DefaultConfig returns a default configuration struct for this
		// adapter. This will be used by the configuration system to establish
		// the shape of the block of configuration state passed to the NewAspect method.
		DefaultConfig() (c AspectParams)

		// ValidateConfig determines whether the given configuration meets all correctness requirements.
		ValidateConfig(c AspectParams, validator expr.TypeChecker, finder descriptor.Finder) *adapter.ConfigErrors
	}

	// BuilderValidatorFinder is used to find specific underlying validators.
	// Manager registry and adapter registry should implement this interface
	// so ConfigValidators can be uniformly accessed.
	BuilderValidatorFinder func(name string) (adapter.ConfigValidator, bool)

	// AspectValidatorFinder is used to find specific underlying validators.
	// Manager registry and adapter registry should implement this interface
	// so ConfigValidators can be uniformly accessed.
	AspectValidatorFinder func(kind Kind) (AspectValidator, bool)

	// AdapterToAspectMapper returns the set of aspect kinds implemented by
	// the given builder.
	AdapterToAspectMapper func(builder string) KindSet

	// BuilderInfoFinder is used to find specific handlers BuilderInfo for configuration.
	BuilderInfoFinder func(name string) (*adapter.BuilderInfo, bool)

	// ConfigureHandler is used to configure handler implementation with Types associated with all the templates that
	// it supports.
	ConfigureHandler func(actions []*pb.Action, constructors map[string]*pb.Constructor,
		handlers map[string]*HandlerBuilderInfo, tmplRepo template.Repository, expr expr.TypeChecker, df expr.AttributeDescriptorFinder) error
)

// newValidator returns a validator given component validators.
func newValidator(managerFinder AspectValidatorFinder, adapterFinder BuilderValidatorFinder,
	builderInfoFinder BuilderInfoFinder, configureHandler ConfigureHandler, templateRepo template.Repository,
	findAspects AdapterToAspectMapper, strict bool, typeChecker expr.TypeChecker) *validator {
	return &validator{
		managerFinder:        managerFinder,
		adapterFinder:        adapterFinder,
		builderInfoFinder:    builderInfoFinder,
		configureHandler:     configureHandler,
		templateRepo:         templateRepo,
		findAspects:          findAspects,
		constructorByName:    make(map[string]*pb.Constructor),
		actions:              make([]*pb.Action, 0),
		strict:               strict,
		typeChecker:          typeChecker,
		handlerBuilderByName: make(map[string]*HandlerBuilderInfo),
		validated: &Validated{
			adapterByName: make(map[adapterKey]*pb.Adapter),
			handlerByName: make(map[string]*HandlerInfo),
			rule:          make(map[rulesKey]*pb.ServiceConfig),
			adapter:       make(map[string]*pb.GlobalConfig),
			descriptor:    make(map[string]*pb.GlobalConfig),
			shas:          make(map[string][sha1.Size]byte),
		},
	}
}

type (
	// validator is the Configuration validator.
	validator struct {
		managerFinder        AspectValidatorFinder
		adapterFinder        BuilderValidatorFinder
		builderInfoFinder    BuilderInfoFinder
		configureHandler     ConfigureHandler
		templateRepo         template.Repository
		findAspects          AdapterToAspectMapper
		descriptorFinder     descriptor.Finder
		handlerBuilderByName map[string]*HandlerBuilderInfo
		constructorByName    map[string]*pb.Constructor
		actions              []*pb.Action
		strict               bool
		typeChecker          expr.TypeChecker
		validated            *Validated
	}

	adapterKey struct {
		kind Kind
		name string
	}

	// rulesKey is used to lookup the combined rules document.
	rulesKey struct {
		// Scope of the rules document.
		Scope string
		// Subject of the rules document.
		Subject string
	}

	// Validated store validated configuration.
	// It has been validated as internally consistent and correct.
	Validated struct {
		adapterByName map[adapterKey]*pb.Adapter
		// descriptors and adapters are only allowed in global scope
		adapter       map[string]*pb.GlobalConfig
		handlerByName map[string]*HandlerInfo
		descriptor    map[string]*pb.GlobalConfig
		rule          map[rulesKey]*pb.ServiceConfig
		shas          map[string][sha1.Size]byte
		numAspects    int
	}

	// HandlerBuilderInfo stores validated HandlerBuilders..
	HandlerBuilderInfo struct {
		handlerBuilder     *config.HandlerBuilder
		handlerCnfg        *pb.Handler
		supportedTemplates []adapter.SupportedTemplates
	}

	// HandlerInfo stores validated and configured Handlers.
	HandlerInfo struct {
		handlerInstance    *config.Handler
		adapterName        string
		supportedTemplates []adapter.SupportedTemplates
	}
)

func copyDescriptors(m map[string]*pb.GlobalConfig) map[string]*pb.GlobalConfig {
	d := map[string]*pb.GlobalConfig{}
	for k, a := range m {
		d[k] = a
	}
	return d
}

// Clone makes a clone of validated config
func (v *Validated) Clone() *Validated {
	aa := map[adapterKey]*pb.Adapter{}
	for k, a := range v.adapterByName {
		aa[k] = a
	}

	hh := map[string]*HandlerInfo{}
	for k, a := range v.handlerByName {
		hh[k] = a
	}

	rule := map[rulesKey]*pb.ServiceConfig{}
	for k, a := range v.rule {
		rule[k] = a
	}

	shas := map[string][sha1.Size]byte{}
	for k, a := range v.shas {
		shas[k] = a
	}

	return &Validated{
		adapterByName: aa,
		handlerByName: hh,
		rule:          rule,
		adapter:       copyDescriptors(v.adapter),
		descriptor:    copyDescriptors(v.descriptor),
		numAspects:    v.numAspects,
		shas:          shas,
	}
}

const (
	global       = "global"
	scopes       = "scopes"
	subjects     = "subjects"
	rules        = "rules"
	constructors = "constructors"
	actionRules  = "action_rules"
	adapters     = "adapters"
	handlers     = "handlers"
	descriptors  = "descriptors"

	keyAdapters            = "/scopes/global/adapters"
	keyHandlers            = "/scopes/global/handlers"
	keyDescriptors         = "/scopes/global/descriptors"
	keyGlobalServiceConfig = "/scopes/global/subjects/global/rules"
	keyConstructorsConfig  = "/scopes/global/subjects/global/constructors"
	keyActionsConfig       = "/scopes/global/subjects/global/action_rules"
)

// String string representation of a Key
func (p rulesKey) String() string {
	return fmt.Sprintf("%s/%s", p.Scope, p.Subject)
}

// /scopes/global/subjects/global/rules --> global / global
func parseRulesKey(key string) (k *rulesKey) {
	comps := strings.Split(key, "/")
	if len(comps) < 6 {
		return nil
	}
	if comps[1] != scopes || comps[3] != subjects {
		return nil
	}
	k = &rulesKey{comps[2], comps[4]}
	return k
}

func (a adapterKey) String() string {
	return fmt.Sprintf("%s//%s", a.kind, a.name)
}

// FIXME post alpha
// create new messages of type
// message MetricList {
//   repeated metrics = 1;
// }
// One for each type of descriptor
// Those messages can be parsed directly using proto.jsonp.
// At present globalConfig.Adapters contains `struct` that prevents us from using proto.jsonp

// compatfilterConfig
// given a yaml file, filter specific keys from it
// globalConfig contains descriptors and adapters which will be split shortly.
func compatfilterConfig(cfg string, shouldSelect func(string) bool) ([]byte, map[string]interface{}, error) {
	//data []byte, m map[string]interface{}, err error
	var m map[string]interface{}
	var data []byte
	var err error

	if err = yaml.Unmarshal([]byte(cfg), &m); err != nil {
		return data, nil, err
	}

	for k := range m {
		if !shouldSelect(k) {
			delete(m, k)
		}
	}
	data, err = json.Marshal(m)
	return data, m, err
}

// validateDescriptors
//
// Enums as struct fields can be symbolic names.
// However enums inside maps *cannot* be symbolic names.
// TODO add validation beyond proto parse
func (p *validator) validateDescriptors(key string, cfg string) (ce *adapter.ConfigErrors) {
	m, ce := descriptor.Parse(cfg)
	if m == nil || ce != nil {
		return
	}

	for _, desc := range m.Metrics {
		ce = ce.Extend(descriptor.ValidateMetric(desc))
	}
	for _, desc := range m.Logs {
		ce = ce.Extend(descriptor.ValidateLogEntry(desc))
	}
	for _, desc := range m.Quotas {
		ce = ce.Extend(descriptor.ValidateQuota(desc))
	}
	for _, desc := range m.MonitoredResources {
		ce = ce.Extend(descriptor.ValidateMonitoredResource(desc))
	}
	for _, desc := range m.Principals {
		ce = ce.Extend(descriptor.ValidatePrincipal(desc))
	}

	p.validated.descriptor[key] = m
	return ce
}

// validateAdapters consumes a yml config string with adapter config.
// It is validated in the presence of validators.
func (p *validator) validateAdapters(key string, cfg string) (ce *adapter.ConfigErrors) {
	var ferr error
	var data []byte

	if data, _, ferr = compatfilterConfig(cfg, func(s string) bool {
		return s == "adapters"
	}); ferr != nil {
		return ce.Appendf("adapterConfig", "failed to unmarshal config into proto with err: %v", ferr)
	}

	var m = &pb.GlobalConfig{}
	if err := yaml.Unmarshal(data, m); err != nil {
		return ce.Appendf("adapterConfig", "failed to unmarshal config into proto: %v", err)
	}

	var acfg adapter.Config
	var err *adapter.ConfigErrors
	// FIXME update this when we start supporting adapters defined in multiple scopes
	p.validated.adapterByName = make(map[adapterKey]*pb.Adapter)
	for _, aa := range m.GetAdapters() {
		if acfg, err = convertAdapterParams(p.adapterFinder, aa.Impl, aa.Params, p.strict); err != nil {
			ce = ce.Appendf("Adapter: "+aa.Impl, "failed to convert aspect params to proto: %v", err)
			continue
		}
		aa.Params = acfg
		// check which kinds aa.Impl provides
		// Then register it for all of them.
		kinds := p.findAspects(aa.Impl)
		for kind := Kind(0); kind < NumKinds; kind++ {
			if kinds.IsSet(kind) {
				p.validated.adapterByName[adapterKey{kind, aa.Name}] = aa
			}
		}
	}
	p.validated.adapter[key] = m
	return
}

// ValidateSelector ensures that the selector is valid per expression language.
func (p *validator) validateSelector(selector string, df expr.AttributeDescriptorFinder) (err error) {
	// empty selector always selects
	if len(selector) == 0 {
		return nil
	}
	return p.typeChecker.AssertType(selector, df, dpb.BOOL)
}

// validateAspectRules validates the recursive configuration data structure.
// It is primarily used by validate ServiceConfig.
func (p *validator) validateAspectRules(rules []*pb.AspectRule, path string, validatePresence bool) (numAspects int, ce *adapter.ConfigErrors) {
	var acfg adapter.Config
	for _, rule := range rules {
		if err := p.validateSelector(rule.GetSelector(), p.descriptorFinder); err != nil {
			ce = ce.Append(path+":Selector "+rule.GetSelector(), err)
		}
		var err *adapter.ConfigErrors
		path = path + "/" + rule.GetSelector()
		for idx, aa := range rule.GetAspects() {
			if acfg, err = convertAspectParams(p.managerFinder, aa.Kind, aa.GetParams(), p.strict, p.descriptorFinder); err != nil {
				ce = ce.Appendf(fmt.Sprintf("%s:%s[%d]", path, aa.Kind, idx), "failed to parse params: %v", err)
				continue
			}
			aa.Params = acfg
			numAspects++
			if validatePresence {
				if aa.Adapter == "" {
					aa.Adapter = "default"
				}
				// ensure that aa.Kind has a registered adapter
				k, ok := ParseKind(aa.Kind)
				if !ok {
					ce = ce.Appendf("kind", "%s is not a valid kind", aa.Kind)
				} else {
					ak := adapterKey{k, aa.Adapter}
					if p.validated.adapterByName[ak] == nil {
						ce = ce.Appendf("namedAdapter", "%s not available", ak)
					}
				}
			}
		}
		rs := rule.GetRules()
		if len(rs) == 0 {
			continue
		}
		if na, verr := p.validateAspectRules(rs, path, validatePresence); verr != nil {
			ce = ce.Extend(verr)
		} else {
			numAspects += na
		}
	}
	return numAspects, ce
}

func (p *validator) validateRules(rules []*pb.Rule, path string, cnstrByName map[string]*pb.Constructor,
	hdlrByName map[string]*HandlerBuilderInfo) (ce *adapter.ConfigErrors) {
	for _, rule := range rules {
		if err := p.validateSelector(rule.GetSelector(), p.descriptorFinder); err != nil {
			ce = ce.Append(path+":Selector "+rule.GetSelector(), err)
		}

		path = path + "/" + rule.GetSelector()
		for idx, aa := range rule.GetActions() {
			hasError := false
			if aa.GetHandler() == "" || hdlrByName[aa.GetHandler()] == nil {
				hasError = true
				ce = ce.Appendf(fmt.Sprintf("%s[%d]", path, idx), "handler not specified or is invalid")
			}
			for _, instanceName := range aa.GetInstances() {
				if cnstrByName[instanceName] == nil {
					hasError = true
					ce = ce.Appendf(fmt.Sprintf("%s[%d]", path, idx), "instance '%s' is not defined.", instanceName)
				}
				// TODO Validate if the cnstr's template is something that the handler supports.
			}
			if !hasError {
				p.actions = append(p.actions, aa)
			}
		}
		rs := rule.GetRules()
		if len(rs) == 0 {
			continue
		}
		if verr := p.validateRules(rs, path, cnstrByName, hdlrByName); verr != nil {
			ce = ce.Extend(verr)
		}
	}
	return ce
}

// validateConstructors validates the constructors in the service configuration.
func (p *validator) validateConstructors(constructors []*pb.Constructor) (ce *adapter.ConfigErrors) {
	for _, cnstr := range constructors {
		if ccfg, err := convertConstructorParam(p.templateRepo, cnstr.GetTemplate(), cnstr.GetParams(), p.strict); err != nil {
			ce = ce.Appendf(fmt.Sprintf("constructor:%s", cnstr.GetInstanceName()), "failed to parse params: %v", err)
			continue
		} else {
			cnstr.Params = ccfg
			p.constructorByName[cnstr.GetInstanceName()] = cnstr
		}
	}
	return ce
}

// classifyKeys classifies keys of cfg into rules, adapters, and descriptors.
func classifyKeys(cfg map[string]string) map[string][]string {
	keymap := map[string][]string{}
	for key := range cfg {
		kk := strings.Split(key, "/")
		var k string
		switch kk[len(kk)-1] {
		case rules:
			k = rules
		case constructors:
			k = constructors
		case actionRules:
			k = actionRules
		case adapters:
			k = adapters
		case handlers:
			k = handlers
		case descriptors:
			k = descriptors
		default:
			if glog.V(4) {
				glog.Infoln("unknown key", key)
			}
			continue
		}
		keymap[k] = append(keymap[k], key)
	}

	return keymap
}

func descriptorKey(scope string) string {
	return fmt.Sprintf("/scopes/%s/%s", scope, descriptors)
}

// validate validates a single serviceConfig and globalConfig together.
// It returns a fully validated Config if no errors are found.
func (p *validator) validate(cfg map[string]string) (rt *Validated, ce *adapter.ConfigErrors) {
	keymap := classifyKeys(cfg)

	for _, kk := range keymap[descriptors] {
		if re := p.validateDescriptors(kk, cfg[kk]); re != nil {
			return rt, ce.Appendf("descriptorConfig", "failed validation").Extend(re)
		}
	}

	for _, kk := range keymap[adapters] {
		if re := p.validateAdapters(kk, cfg[kk]); re != nil {
			return rt, ce.Appendf("adapterConfig", "failed validation").Extend(re)
		}
	}

	for _, kk := range keymap[handlers] {
		if re := p.validateHandlers(cfg[kk]); re != nil {
			return rt, ce.Appendf("handlerConfig", "failed validation").Extend(re)
		}
	}

	// The order is important here, because serviceConfig refers to adapters and descriptors
	p.descriptorFinder = descriptor.NewFinder(p.validated.descriptor[descriptorKey(global)])
	for _, kk := range keymap[rules] {
		ck := parseRulesKey(kk)
		if ck == nil {
			continue
		}
		if re := p.validateServiceConfig(*ck, cfg[kk], true); re != nil {
			return rt, ce.Appendf("serviceConfig", "failed validation").Extend(re)
		}
	}

	for _, kk := range keymap[constructors] {
		ck := parseRulesKey(kk)
		if ck == nil {
			continue
		}
		if re := p.validateConstructorConfigs(*ck, cfg[kk]); re != nil {
			return rt, ce.Appendf("serviceConfig", "failed validation").Extend(re)
		}
	}

	for _, kk := range keymap[actionRules] {
		ck := parseRulesKey(kk)
		if ck == nil {
			continue
		}
		if re := p.validateRulesConfig(*ck, cfg[kk], p.constructorByName, p.handlerBuilderByName); re != nil {
			return rt, ce.Appendf("serviceConfig", "failed validation").Extend(re)
		}
	}

	// everything is validated we can configure the handlers.
	if re := p.buildHandlers(); re != nil {
		return rt, ce.Extend(re)
	}

	return p.validated, nil
}

func (p *validator) buildHandlers() (ce *adapter.ConfigErrors) {
	if err := p.configureHandler(p.actions, p.constructorByName, p.handlerBuilderByName, p.templateRepo, p.typeChecker, p.descriptorFinder); err != nil {
		return ce.Appendf("handlerConfig", "failed to configure handler: %v", err)
	}

	for name, handlerBuilder := range p.handlerBuilderByName {
		handlerInstance, err := (*handlerBuilder.handlerBuilder).Build(handlerBuilder.handlerCnfg.Params.(proto.Message))
		// TODO Add validation to ensure handlerInstance support all the templates it claims to support.
		if err != nil {
			return ce.Appendf("handlerConfig: "+name, "failed to build a handler instance: %v", err)
		}
		p.validated.handlerByName[name] = &HandlerInfo{
			adapterName:        handlerBuilder.handlerCnfg.GetAdapter(),
			handlerInstance:    &handlerInstance,
			supportedTemplates: handlerBuilder.supportedTemplates,
		}
	}
	return nil
}

// ValidateServiceConfig validates service config.
// if validatePresence is true it will ensure that the named adapter and Kinds
// have an available and configured adapter.
func (p *validator) validateServiceConfig(pk rulesKey, cfg string, validatePresence bool) (ce *adapter.ConfigErrors) {
	var err error
	m := &pb.ServiceConfig{}
	var numAspects int
	if err = yaml.Unmarshal([]byte(cfg), m); err != nil {
		return ce.Appendf("serviceConfig", "failed to unmarshal config into proto: %v", err)
	}

	if numAspects, ce = p.validateAspectRules(m.GetRules(), "", validatePresence); ce != nil {
		return ce
	}
	p.validated.rule[pk] = m
	p.validated.numAspects += numAspects

	return nil
}

func (p *validator) validateRulesConfig(pk rulesKey, cfg string, cnstrByName map[string]*pb.Constructor,
	hdlrByName map[string]*HandlerBuilderInfo) (ce *adapter.ConfigErrors) {
	var err error
	m := &pb.ServiceConfig{}
	if err = yaml.Unmarshal([]byte(cfg), m); err != nil {
		return ce.Appendf("serviceConfig", "failed to unmarshal config into proto: %v", err)
	}

	if ce = p.validateRules(m.GetActionRules(), "", cnstrByName, hdlrByName); ce != nil {
		return ce
	}

	return nil
}

func (p *validator) validateConstructorConfigs(pk rulesKey, cfg string) (ce *adapter.ConfigErrors) {
	var err error
	m := &pb.ServiceConfig{}
	if err = yaml.Unmarshal([]byte(cfg), m); err != nil {
		return ce.Appendf("serviceConfig", "failed to unmarshal config into proto: %v", err)
	}

	if ce = p.validateConstructors(m.GetConstructors()); ce != nil {
		return ce
	}

	return nil
}

func (p *validator) validateHandlers(cfg string) (ce *adapter.ConfigErrors) {
	var ferr error
	var data []byte

	if data, _, ferr = compatfilterConfig(cfg, func(s string) bool {
		return s == "handlers"
	}); ferr != nil {
		return ce.Appendf("handlerConfig", "failed to unmarshal config into proto with err: %v", ferr)
	}

	var m = &pb.GlobalConfig{}
	if err := yaml.Unmarshal(data, m); err != nil {
		return ce.Appendf("handlerConfig", "failed to unmarshal config into proto: %v", err)
	}

	var hcfg proto.Message
	var err *adapter.ConfigErrors

	for _, hh := range m.GetHandlers() {
		bi, found := p.builderInfoFinder(hh.Adapter)
		if !found {
			ce = ce.Appendf("handlerConfig", "Adapter %s referenced in Handler %s is not found", hh.GetAdapter(), hh.GetName())
			continue
		}
		if hcfg, err = convertHandlerParams(bi, hh.GetName(), hh.Params, p.strict); err != nil {
			ce = ce.Appendf("Handler: "+hh.Adapter, "failed to convert handler params to proto: %v", err)
			continue
		}

		hh.Params = hcfg
		hb := bi.CreateHandlerBuilderFn()
		p.handlerBuilderByName[hh.GetName()] = &HandlerBuilderInfo{handlerCnfg: hh, handlerBuilder: &hb, supportedTemplates: bi.SupportedTemplates}
	}
	return
}

func convertHandlerParams(bi *adapter.BuilderInfo, name string, params interface{}, strict bool) (hc proto.Message, ce *adapter.ConfigErrors) {
	hc = bi.DefaultConfig
	if err := decode(params, hc, strict); err != nil {
		return nil, ce.Appendf(name, "failed to decode handler params: %v", err)
	}
	if err := bi.ValidateConfig(hc); err != nil {
		return nil, ce.Appendf(name, "handler validation failed: %v", err)
	}
	return hc, nil
}

// unknownValidator returns error for the given name.
func unknownValidator(name string) error {
	return fmt.Errorf("unknown type [%s]", name)
}

// unknownKind returns error for the given name.
func unknownKind(name string) error {
	return fmt.Errorf("unknown aspect kind [%s]", name)
}

// convertAdapterParams converts returns a typed proto message based on available validator.
func convertAdapterParams(f BuilderValidatorFinder, name string, params interface{}, strict bool) (ac adapter.Config, ce *adapter.ConfigErrors) {
	var avl adapter.ConfigValidator
	var found bool

	if avl, found = f(name); !found {
		return nil, ce.Append(name, unknownValidator(name))
	}

	ac = avl.DefaultConfig()
	if err := decode(params, ac, strict); err != nil {
		return nil, ce.Appendf(name, "failed to decode adapter params: %v", err)
	}
	if err := avl.ValidateConfig(ac); err != nil {
		return nil, ce.Appendf(name, "adapter validation failed: %v", err)
	}
	return ac, nil
}

// convertConstructorParam converts and returns a typed proto message based on available templates.
func convertConstructorParam(tf template.Repository, templateName string, params interface{},
	strict bool) (cp proto.Message, ce *adapter.ConfigErrors) {

	var found bool
	var tmplInfo template.Info
	if tmplInfo, found = tf.GetTemplateInfo(templateName); !found {
		return nil, ce.Appendf("template", "'%s' is not a registered", templateName)
	}
	cp = tmplInfo.CnstrDefConfig
	if err := decode(params, cp, strict); err != nil {
		return nil, ce.Appendf(templateName, "failed to decode constructor params: %v", err)
	}
	return cp, nil
}

// convertAspectParams converts returns a typed proto message based on available validator.
func convertAspectParams(f AspectValidatorFinder, name string, params interface{}, strict bool, df descriptor.Finder) (AspectParams, *adapter.ConfigErrors) {
	var ce *adapter.ConfigErrors
	var avl AspectValidator
	var found bool
	var k Kind

	if k, found = ParseKind(name); !found {
		return nil, ce.Append(name, unknownKind(name))
	}

	if avl, found = f(k); !found {
		return nil, ce.Append(name, unknownValidator(name))
	}

	ap := avl.DefaultConfig()
	if err := decode(params, ap, strict); err != nil {
		return nil, ce.Appendf(name, "failed to decode aspect params: %v", err)
	}
	eval, err := expr.NewCEXLEvaluator(expr.DefaultCacheSize)
	if err != nil {
		return nil, ce.Appendf(name, "failed to create expression evaluator: %v", err)
	}
	if err := avl.ValidateConfig(ap, eval, df); err != nil {
		return nil, ce.Appendf(name, "aspect validation failed: %v", err)
	}
	return ap, nil
}

// decode interprets src interface{} as the specified proto message.
// if strict is true returns error on unknown fields.
// TODO dst at message CnstParam {Value string} fails to decode value: 1. Seems weird, investigate
// The constructorParams are all stringified for all ValueType items, so if a value field is meant to be
// of ValueType, it's expression can be anything and we are suppose to infer type from it. To parse the
// field we need a proto, which is our synthesized ConstructorParam from each template. Due to the limitation
// of non string fields cannot parsed into strings, we might have a problem.. Investigate.
func decode(src interface{}, dst proto.Message, strict bool) error {
	ba, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("failed to marshal config into json: %v", err)
	}
	um := jsonpb.Unmarshaler{AllowUnknownFields: !strict}
	if err := um.Unmarshal(bytes.NewReader(ba), dst); err != nil {
		b2, _ := json.Marshal(dst)
		return fmt.Errorf("failed to unmarshal config <%s> into proto: %v %s", string(ba), err, string(b2))
	}
	return nil
}
