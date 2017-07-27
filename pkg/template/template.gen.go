package template

import (
	"github.com/golang/protobuf/proto"

	adptConfig "istio.io/mixer/pkg/adapter/config"
)

var (
	SupportedTmplInfo = map[string]Info{

		istio_mixer_adapter_sample_check.TemplateName: {
			CtrCfg:   &istio_mixer_adapter_sample_check.ConstructorParam{},
			BldrName: "istio.io/mixer/template/sample/check.SampleProcessorBuilder",
			SupportsTemplate: func(hndlrBuilder adptConfig.HandlerBuilder) bool {
				_, ok := hndlrBuilder.(istio_mixer_adapter_sample_check.SampleProcessorBuilder)
				return ok
			},
			InferType: func(cp proto.Message, tEvalFn TypeEvalFn) (proto.Message, error) {
				var err error = nil
				cpb := cp.(*istio_mixer_adapter_sample_check.ConstructorParam)
				infrdType := &istio_mixer_adapter_sample_check.Type{}

				infrdType.CheckExpression = istio_mixer_v1_config_descriptor.STRING

				_ = cpb
				return infrdType, err
			},
			ConfigureType: func(types map[string]proto.Message, builder *adptConfig.HandlerBuilder) error {

				castedBuilder := (*builder).(istio_mixer_adapter_sample_check.SampleProcessorBuilder)
				castedTypes := make(map[string]*istio_mixer_adapter_sample_check.Type)
				for k, v := range types {

					v1 := v.(*istio_mixer_adapter_sample_check.Type)
					castedTypes[k] = v1
				}
				return castedBuilder.ConfigureSample(castedTypes)
			},
		},

		istio_mixer_adapter_sample_quota.TemplateName: {
			CtrCfg:   &istio_mixer_adapter_sample_quota.ConstructorParam{},
			BldrName: "istio.io/mixer/template/sample/quota.QuotaProcessorBuilder",
			SupportsTemplate: func(hndlrBuilder adptConfig.HandlerBuilder) bool {
				_, ok := hndlrBuilder.(istio_mixer_adapter_sample_quota.QuotaProcessorBuilder)
				return ok
			},
			InferType: func(cp proto.Message, tEvalFn TypeEvalFn) (proto.Message, error) {
				var err error = nil
				cpb := cp.(*istio_mixer_adapter_sample_quota.ConstructorParam)
				infrdType := &istio_mixer_adapter_sample_quota.Type{}

				infrdType.Dimensions = make(map[string]istio_mixer_v1_config_descriptor.ValueType)
				for k, v := range cpb.Dimensions {
					if infrdType.Dimensions[k], err = tEvalFn(v); err != nil {
						return nil, err
					}
				}

				_ = cpb
				return infrdType, err
			},
			ConfigureType: func(types map[string]proto.Message, builder *adptConfig.HandlerBuilder) error {

				castedBuilder := (*builder).(istio_mixer_adapter_sample_quota.QuotaProcessorBuilder)
				castedTypes := make(map[string]*istio_mixer_adapter_sample_quota.Type)
				for k, v := range types {

					v1 := v.(*istio_mixer_adapter_sample_quota.Type)
					castedTypes[k] = v1
				}
				return castedBuilder.ConfigureQuota(castedTypes)
			},
		},

		istio_mixer_adapter_sample_report.TemplateName: {
			CtrCfg:   &istio_mixer_adapter_sample_report.ConstructorParam{},
			BldrName: "istio.io/mixer/template/sample/report.SampleProcessorBuilder",
			SupportsTemplate: func(hndlrBuilder adptConfig.HandlerBuilder) bool {
				_, ok := hndlrBuilder.(istio_mixer_adapter_sample_report.SampleProcessorBuilder)
				return ok
			},
			InferType: func(cp proto.Message, tEvalFn TypeEvalFn) (proto.Message, error) {
				var err error = nil
				cpb := cp.(*istio_mixer_adapter_sample_report.ConstructorParam)
				infrdType := &istio_mixer_adapter_sample_report.Type{}

				if infrdType.Value, err = tEvalFn(cpb.Value); err != nil {
					return nil, err
				}

				infrdType.Dimensions = make(map[string]istio_mixer_v1_config_descriptor.ValueType)
				for k, v := range cpb.Dimensions {
					if infrdType.Dimensions[k], err = tEvalFn(v); err != nil {
						return nil, err
					}
				}

				_ = cpb
				return infrdType, err
			},
			ConfigureType: func(types map[string]proto.Message, builder *adptConfig.HandlerBuilder) error {

				castedBuilder := (*builder).(istio_mixer_adapter_sample_report.SampleProcessorBuilder)
				castedTypes := make(map[string]*istio_mixer_adapter_sample_report.Type)
				for k, v := range types {

					v1 := v.(*istio_mixer_adapter_sample_report.Type)
					castedTypes[k] = v1
				}
				return castedBuilder.ConfigureSample(castedTypes)
			},
		},
	}
)
