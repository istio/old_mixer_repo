// Copyright 2016 Istio Authors
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

package template

import (
	"fmt"

	"github.com/golang/protobuf/proto"

	pb "istio.io/api/mixer/v1/config/descriptor"
	adptConfig "istio.io/mixer/pkg/adapter/config"
	sample_report "istio.io/mixer/pkg/template/sample/report"
)

var (
	templateInfos = map[string]Info{
		sample_report.TemplateName: {
			InferTypeFn:     inferTypeForSampleReport,
			CnstrDefConfig:  &sample_report.ConstructorParam{},
			ConfigureTypeFn: configureTypeForSampleReport,
		},
	}
)

func inferTypeForSampleReport(cp proto.Message, tEvalFn TypeEvalFn) (proto.Message, error) {
	cpb := &sample_report.ConstructorParam{}
	var err error
	var ok bool

	if cpb, ok = cp.(*sample_report.ConstructorParam); !ok {
		return nil, fmt.Errorf("Constructor param %v is not of type %T", cp, cpb)
	}

	infrdType := &sample_report.Type{}
	if infrdType.Value, err = tEvalFn(cpb.Value); err != nil {
		return nil, err
	}

	infrdType.Dimensions = make(map[string]pb.ValueType)
	for k, v := range cpb.Dimensions {
		if infrdType.Dimensions[k], err = tEvalFn(v); err != nil {
			return nil, err
		}
	}

	return infrdType, nil
}

func configureTypeForSampleReport(types map[string]proto.Message, builder *adptConfig.HandlerBuilder) error {

	castedBuilder, ok := (*builder).(sample_report.SampleProcessorBuilder)
	if !ok {
		var x sample_report.SampleProcessorBuilder
		return fmt.Errorf("cannot cast %v into %T", builder, x)
	}
	castedTypes := make(map[string]*sample_report.Type)
	for k, v := range types {
		v1, ok := v.(*sample_report.Type)
		if !ok {
			return fmt.Errorf("cannot cast %v into %T", v1, sample_report.Type{})
		}
		castedTypes[k] = v1
	}

	return castedBuilder.ConfigureSample(castedTypes)
}
