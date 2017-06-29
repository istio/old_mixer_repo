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
