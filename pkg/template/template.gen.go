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

// THIS FILE IS AUTOMATICALLY GENERATED.

package template

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	rpc "github.com/googleapis/googleapis/google/rpc"
	"github.com/hashicorp/go-multierror"

	"istio.io/api/mixer/v1/config/descriptor"
	"istio.io/mixer/pkg/adapter"
	adptTmpl "istio.io/mixer/pkg/adapter/template"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/expr"
	"istio.io/mixer/pkg/status"

	"istio.io/mixer/template/sample/check"

	"istio.io/mixer/template/sample/quota"

	"istio.io/mixer/template/sample/report"
)

var (
	SupportedTmplInfo = map[string]Info{

		istio_mixer_adapter_sample_check.TemplateName: {
			CtrCfg:    &istio_mixer_adapter_sample_check.InstanceParam{},
			Variety:   adptTmpl.TEMPLATE_VARIETY_CHECK,
			BldrName:  "istio.io/mixer/template/sample/check.SampleHandlerBuilder",
			HndlrName: "istio.io/mixer/template/sample/check.SampleHandler",
			SupportsTemplate: func(hndlrBuilder adapter.HandlerBuilder) bool {
				_, ok := hndlrBuilder.(istio_mixer_adapter_sample_check.SampleHandlerBuilder)
				return ok
			},
			HandlerSupportsTemplate: func(hndlr adapter.Handler) bool {
				_, ok := hndlr.(istio_mixer_adapter_sample_check.SampleHandler)
				return ok
			},
			InferType: func(cp proto.Message, tEvalFn TypeEvalFn) (proto.Message, error) {
				var err error = nil
				cpb := cp.(*istio_mixer_adapter_sample_check.InstanceParam)
				infrdType := &istio_mixer_adapter_sample_check.Type{}

				_ = cpb
				return infrdType, err
			},
			ConfigureType: func(types map[string]proto.Message, builder *adapter.HandlerBuilder) error {
				// Mixer framework should have ensured the type safety.
				castedBuilder := (*builder).(istio_mixer_adapter_sample_check.SampleHandlerBuilder)
				castedTypes := make(map[string]*istio_mixer_adapter_sample_check.Type, len(types))
				for k, v := range types {
					// Mixer framework should have ensured the type safety.
					v1 := v.(*istio_mixer_adapter_sample_check.Type)
					castedTypes[k] = v1
				}
				return castedBuilder.ConfigureSampleHandler(castedTypes)
			},

			ProcessCheck: func(insts map[string]proto.Message, attrs attribute.Bag, mapper expr.Evaluator,
				handler adapter.Handler) (rpc.Status, adapter.CacheabilityInfo) {
				var found bool
				var err error

				var instances []*istio_mixer_adapter_sample_check.Instance
				castedInsts := make(map[string]*istio_mixer_adapter_sample_check.InstanceParam, len(insts))
				for k, v := range insts {
					v1 := v.(*istio_mixer_adapter_sample_check.InstanceParam)
					castedInsts[k] = v1
				}
				for name, md := range castedInsts {

					CheckExpression, err := mapper.Eval(md.CheckExpression, attrs)

					if err != nil {
						return status.WithError(err), adapter.CacheabilityInfo{}
					}

					instances = append(instances, &istio_mixer_adapter_sample_check.Instance{
						Name: name,

						CheckExpression: CheckExpression.(string),
					})
					_ = md
				}
				var cacheInfo adapter.CacheabilityInfo
				if found, cacheInfo, err = handler.(istio_mixer_adapter_sample_check.SampleHandler).HandleSample(instances); err != nil {
					return status.WithError(err), adapter.CacheabilityInfo{}
				}

				if found {
					return status.OK, cacheInfo
				}

				return status.WithPermissionDenied(fmt.Sprintf("%s rejected", instances)), adapter.CacheabilityInfo{}
			},
			ProcessReport: nil,
			ProcessQuota:  nil,
		},

		istio_mixer_adapter_sample_quota.TemplateName: {
			CtrCfg:    &istio_mixer_adapter_sample_quota.InstanceParam{},
			Variety:   adptTmpl.TEMPLATE_VARIETY_QUOTA,
			BldrName:  "istio.io/mixer/template/sample/quota.QuotaHandlerBuilder",
			HndlrName: "istio.io/mixer/template/sample/quota.QuotaHandler",
			SupportsTemplate: func(hndlrBuilder adapter.HandlerBuilder) bool {
				_, ok := hndlrBuilder.(istio_mixer_adapter_sample_quota.QuotaHandlerBuilder)
				return ok
			},
			HandlerSupportsTemplate: func(hndlr adapter.Handler) bool {
				_, ok := hndlr.(istio_mixer_adapter_sample_quota.QuotaHandler)
				return ok
			},
			InferType: func(cp proto.Message, tEvalFn TypeEvalFn) (proto.Message, error) {
				var err error = nil
				cpb := cp.(*istio_mixer_adapter_sample_quota.InstanceParam)
				infrdType := &istio_mixer_adapter_sample_quota.Type{}

				infrdType.Dimensions = make(map[string]istio_mixer_v1_config_descriptor.ValueType, len(cpb.Dimensions))
				for k, v := range cpb.Dimensions {
					if infrdType.Dimensions[k], err = tEvalFn(v); err != nil {
						return nil, err
					}
				}

				_ = cpb
				return infrdType, err
			},
			ConfigureType: func(types map[string]proto.Message, builder *adapter.HandlerBuilder) error {
				// Mixer framework should have ensured the type safety.
				castedBuilder := (*builder).(istio_mixer_adapter_sample_quota.QuotaHandlerBuilder)
				castedTypes := make(map[string]*istio_mixer_adapter_sample_quota.Type, len(types))
				for k, v := range types {
					// Mixer framework should have ensured the type safety.
					v1 := v.(*istio_mixer_adapter_sample_quota.Type)
					castedTypes[k] = v1
				}
				return castedBuilder.ConfigureQuotaHandler(castedTypes)
			},

			ProcessQuota: func(quotaName string, inst proto.Message, attrs attribute.Bag, mapper expr.Evaluator, handler adapter.Handler,
				qma adapter.QuotaRequestArgs) (rpc.Status, adapter.CacheabilityInfo, adapter.QuotaResult) {
				castedInst := inst.(*istio_mixer_adapter_sample_quota.InstanceParam)

				Dimensions, err := evalAll(castedInst.Dimensions, attrs, mapper)

				if err != nil {
					msg := fmt.Sprintf("failed to eval Dimensions for instance '%s': %v", quotaName, err)
					glog.Error(msg)
					return status.WithInvalidArgument(msg), adapter.CacheabilityInfo{}, adapter.QuotaResult{}
				}

				instance := &istio_mixer_adapter_sample_quota.Instance{
					Name: quotaName,

					Dimensions: Dimensions,
				}

				var qr adapter.QuotaResult
				var cacheInfo adapter.CacheabilityInfo
				if qr, cacheInfo, err = handler.(istio_mixer_adapter_sample_quota.QuotaHandler).HandleQuota(instance, qma); err != nil {
					glog.Errorf("Quota allocation failed: %v", err)
					return status.WithError(err), adapter.CacheabilityInfo{}, adapter.QuotaResult{}
				}
				if qr.Amount == 0 {
					msg := fmt.Sprintf("Unable to allocate %v units from quota %s", qma.QuotaAmount, quotaName)
					glog.Warning(msg)
					return status.WithResourceExhausted(msg), adapter.CacheabilityInfo{}, adapter.QuotaResult{}
				}
				if glog.V(2) {
					glog.Infof("Allocated %v units from quota %s", qma.QuotaAmount, quotaName)
				}
				return status.OK, cacheInfo, qr
			},
			ProcessReport: nil,
			ProcessCheck:  nil,
		},

		istio_mixer_adapter_sample_report.TemplateName: {
			CtrCfg:    &istio_mixer_adapter_sample_report.InstanceParam{},
			Variety:   adptTmpl.TEMPLATE_VARIETY_REPORT,
			BldrName:  "istio.io/mixer/template/sample/report.SampleHandlerBuilder",
			HndlrName: "istio.io/mixer/template/sample/report.SampleHandler",
			SupportsTemplate: func(hndlrBuilder adapter.HandlerBuilder) bool {
				_, ok := hndlrBuilder.(istio_mixer_adapter_sample_report.SampleHandlerBuilder)
				return ok
			},
			HandlerSupportsTemplate: func(hndlr adapter.Handler) bool {
				_, ok := hndlr.(istio_mixer_adapter_sample_report.SampleHandler)
				return ok
			},
			InferType: func(cp proto.Message, tEvalFn TypeEvalFn) (proto.Message, error) {
				var err error = nil
				cpb := cp.(*istio_mixer_adapter_sample_report.InstanceParam)
				infrdType := &istio_mixer_adapter_sample_report.Type{}

				if infrdType.Value, err = tEvalFn(cpb.Value); err != nil {
					return nil, err
				}

				infrdType.Dimensions = make(map[string]istio_mixer_v1_config_descriptor.ValueType, len(cpb.Dimensions))
				for k, v := range cpb.Dimensions {
					if infrdType.Dimensions[k], err = tEvalFn(v); err != nil {
						return nil, err
					}
				}

				_ = cpb
				return infrdType, err
			},
			ConfigureType: func(types map[string]proto.Message, builder *adapter.HandlerBuilder) error {
				// Mixer framework should have ensured the type safety.
				castedBuilder := (*builder).(istio_mixer_adapter_sample_report.SampleHandlerBuilder)
				castedTypes := make(map[string]*istio_mixer_adapter_sample_report.Type, len(types))
				for k, v := range types {
					// Mixer framework should have ensured the type safety.
					v1 := v.(*istio_mixer_adapter_sample_report.Type)
					castedTypes[k] = v1
				}
				return castedBuilder.ConfigureSampleHandler(castedTypes)
			},

			ProcessReport: func(insts map[string]proto.Message, attrs attribute.Bag, mapper expr.Evaluator, handler adapter.Handler) rpc.Status {
				result := &multierror.Error{}
				var instances []*istio_mixer_adapter_sample_report.Instance

				castedInsts := make(map[string]*istio_mixer_adapter_sample_report.InstanceParam, len(insts))
				for k, v := range insts {
					v1 := v.(*istio_mixer_adapter_sample_report.InstanceParam)
					castedInsts[k] = v1
				}
				for name, md := range castedInsts {

					Value, err := mapper.Eval(md.Value, attrs)

					if err != nil {
						result = multierror.Append(result, fmt.Errorf("failed to eval Value for instance '%s': %v", name, err))
						continue
					}

					Dimensions, err := evalAll(md.Dimensions, attrs, mapper)

					if err != nil {
						result = multierror.Append(result, fmt.Errorf("failed to eval Dimensions for instance '%s': %v", name, err))
						continue
					}

					Int64Primitive, err := mapper.Eval(md.Int64Primitive, attrs)

					if err != nil {
						result = multierror.Append(result, fmt.Errorf("failed to eval Int64Primitive for instance '%s': %v", name, err))
						continue
					}

					BoolPrimitive, err := mapper.Eval(md.BoolPrimitive, attrs)

					if err != nil {
						result = multierror.Append(result, fmt.Errorf("failed to eval BoolPrimitive for instance '%s': %v", name, err))
						continue
					}

					DoublePrimitive, err := mapper.Eval(md.DoublePrimitive, attrs)

					if err != nil {
						result = multierror.Append(result, fmt.Errorf("failed to eval DoublePrimitive for instance '%s': %v", name, err))
						continue
					}

					StringPrimitive, err := mapper.Eval(md.StringPrimitive, attrs)

					if err != nil {
						result = multierror.Append(result, fmt.Errorf("failed to eval StringPrimitive for instance '%s': %v", name, err))
						continue
					}

					instances = append(instances, &istio_mixer_adapter_sample_report.Instance{
						Name: name,

						Value: Value,

						Dimensions: Dimensions,

						Int64Primitive: Int64Primitive.(int64),

						BoolPrimitive: BoolPrimitive.(bool),

						DoublePrimitive: DoublePrimitive.(float64),

						StringPrimitive: StringPrimitive.(string),
					})
					_ = md
				}

				if err := handler.(istio_mixer_adapter_sample_report.SampleHandler).HandleSample(instances); err != nil {
					result = multierror.Append(result, fmt.Errorf("failed to report all values: %v", err))
				}

				err := result.ErrorOrNil()
				if err != nil {
					return status.WithError(err)
				}

				return status.OK
			},
			ProcessCheck: nil,
			ProcessQuota: nil,
		},
	}
)
