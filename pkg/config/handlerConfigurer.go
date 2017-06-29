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

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"

	pbd "istio.io/api/mixer/v1/config/descriptor"
	pb "istio.io/mixer/pkg/config/proto"
	"istio.io/mixer/pkg/expr"
	"istio.io/mixer/pkg/template"
)

type (
	handlerConfigurer struct {
		tmplRepo       template.Repository
		typeChecker    expr.TypeChecker
		attrDescFinder expr.AttributeDescriptorFinder
	}
	instancesByTemplate struct {
		instancesNamesByTemplate map[string][]string
	}
)

func (t *instancesByTemplate) insertInstance(tmplName string, instName string) {
	instsPerTmpl, exists := t.instancesNamesByTemplate[tmplName]
	if !exists {
		t.instancesNamesByTemplate[tmplName] = make([]string, 0)
	}

	if !contains(instsPerTmpl, instName) {
		t.instancesNamesByTemplate[tmplName] = append(t.instancesNamesByTemplate[tmplName], instName)
	}
}

func newInstancesByTemplate() instancesByTemplate {
	return instancesByTemplate{make(map[string][]string)}
}

func configureHandlers(actions []*pb.Action, constructors map[string]*pb.Constructor,
	handlers map[string]*HandlerBuilderInfo, tmplRepo template.Repository, expr expr.TypeChecker, df expr.AttributeDescriptorFinder) error {
	configurer := handlerConfigurer{tmplRepo: tmplRepo, typeChecker: expr, attrDescFinder: df}

	iTypes, err := configurer.inferTypes(constructors)
	if err != nil {
		return err
	}
	grpHandlers, err := configurer.groupHandlerInstancesByTemplate(actions, constructors, handlers)
	if err != nil {
		return err
	}

	return configurer.dispatchTypesToHandlers(iTypes, grpHandlers, handlers)
}

func (h *handlerConfigurer) dispatchTypesToHandlers(infrdTypes map[string]proto.Message,
	instsByTmpls map[string]instancesByTemplate, handlers map[string]*HandlerBuilderInfo) error {
	for hName, instsByTmpl := range instsByTmpls {
		hb, found := handlers[hName]
		if !found {
			// This should not happen, since the dispatchTypesToHandlers should get called after all verifications.
			return fmt.Errorf("handler %s is not registered", hName)
		}

		for tmplName, insts := range instsByTmpl.instancesNamesByTemplate {
			ti, found := h.tmplRepo.GetTemplateInfo(tmplName)
			if !found {
				// This should not happen, since the dispatchTypesToHandlers should get called after all verifications.
				return fmt.Errorf("template %s is not registered", tmplName)
			}

			typesToConfigure := make(map[string]proto.Message)
			for _, inst := range insts {
				v, found := infrdTypes[inst]
				if !found {
					// This should not happen, since the dispatchTypesToHandlers should get called after all
					// verifications.
					return fmt.Errorf("instance %s is not found in inferred types", inst)
				}
				typesToConfigure[inst] = v
			}
			err := ti.ConfigureTypeFn(typesToConfigure, hb.handlerBuilder)
			if err != nil {
				glog.Warningf("Cannot configure handler %s with types %v: %v", hName, typesToConfigure, err)
				return err
			}
		}
	}
	// TODO How to handle case where error in config/or adapter returns error, and we have done partial configuration.
	return nil
}

func (h *handlerConfigurer) groupHandlerInstancesByTemplate(actions []*pb.Action, constructors map[string]*pb.Constructor,
	handlers map[string]*HandlerBuilderInfo) (map[string]instancesByTemplate, error) {
	result := make(map[string]instancesByTemplate)

	for _, action := range actions {
		hName := action.GetHandler()
		if _, ok := handlers[hName]; !ok {
			return nil, fmt.Errorf("unable to find a configured handler with name '%s' referenced in action %v", hName, action)
		}

		instsByTmpl, exists := result[hName]
		if !exists {
			instsByTmpl = newInstancesByTemplate()
			result[hName] = instsByTmpl
		}

		for _, iName := range action.GetInstances() {
			cnstr, ok := constructors[iName]
			if !ok {
				return nil, fmt.Errorf("unable to find an a constructor with instance name '%s' "+
					"referenced in action %v", iName, action)
			}

			instsByTmpl.insertInstance(cnstr.GetTemplate(), iName)
		}
	}
	return result, nil
}

func (h *handlerConfigurer) inferTypes(constructors map[string]*pb.Constructor) (map[string]proto.Message, error) {
	result := make(map[string]proto.Message)
	for _, cnstr := range constructors {
		tmplInfo, found := h.tmplRepo.GetTemplateInfo(cnstr.GetTemplate())
		if !found {
			return nil, fmt.Errorf("template %s in constructor %v is not registered", cnstr.GetTemplate(), cnstr)
		}

		// TODO: The validation on the correctness of the expression is done here. I think it is fine, pls double check.
		inferredType, err := tmplInfo.InferTypeFn(cnstr.GetParams().(proto.Message), func(expr string) (pbd.ValueType, error) {
			return h.typeChecker.EvalType(expr, h.attrDescFinder)
		})
		if err != nil {
			return nil, fmt.Errorf("cannot infer type information from params %v in constructor %v", cnstr.Params, cnstr)
		}
		result[cnstr.GetInstanceName()] = inferredType
	}
	return result, nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
