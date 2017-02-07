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

package aspect

import (
	"fmt"
	"time"

	"google.golang.org/genproto/googleapis/rpc/code"

	"github.com/hashicorp/go-multierror"
	dpb "istio.io/api/mixer/v1/config/descriptor"
	"istio.io/mixer/pkg/adapter"
	aconfig "istio.io/mixer/pkg/aspect/config"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/config"
	"istio.io/mixer/pkg/expr"
)

type (
	metricsManager struct {
	}

	metricInfo struct {
		metricKind adapter.MetricKind
		value      string
		labels     map[string]string
	}

	metricsWrapper struct {
		aspect   adapter.MetricsAspect
		metadata map[string]*metricInfo // metric name -> info
	}
)

// NewMetricsManager returns a manager for the metric aspect.
func NewMetricsManager() Manager {
	return &metricsManager{}
}

// NewAspect creates a metric aspect.
func (m *metricsManager) NewAspect(c *config.Combined, a adapter.Builder, env adapter.Env) (Wrapper, error) {
	params := c.Aspect.Params.(*aconfig.MetricsParams)

	// TODO: get descriptors from config
	desc := []*dpb.MetricDescriptor{
		{
			Name:  "api_responses",
			Kind:  dpb.COUNTER,
			Value: dpb.INT64,
			Labels: []*dpb.LabelDescriptor{
				{Name: "api_method", ValueType: dpb.STRING},
				{Name: "response_code", ValueType: dpb.INT64},
			},
		},
	}

	metadata := make(map[string]*metricInfo)
	defs := make([]adapter.MetricDefinition, len(desc))
	for i, d := range desc {
		def := toDefinition(d)
		metric, found := find(params.Metrics, def.Name)
		if !found {
			env.Logger().Warningf("No metric found for descriptor %s, skipping it", def.Name)
			continue
		}

		defs[i] = def
		metadata[def.Name] = &metricInfo{
			metricKind: def.Kind,
			value:      metric.Value,
			labels:     metric.Labels,
		}
	}

	asp, err := a.(adapter.MetricsBuilder).NewMetricsAspect(env, c.Builder.Params.(adapter.AspectConfig), defs)
	if err != nil {
		return nil, fmt.Errorf("failed to construct metrics aspect with config '%v' and err: %s", c, err)
	}
	return &metricsWrapper{asp, metadata}, nil
}

func (*metricsManager) Kind() string                        { return MetricKind }
func (*metricsManager) DefaultConfig() adapter.AspectConfig { return &aconfig.MetricsParams{} }

func (*metricsManager) ValidateConfig(adapter.AspectConfig) (ce *adapter.ConfigErrors) {
	// TODO: we need to be provided the metric descriptors in addition to the metrics themselves here, so we can do type assertions.
	// We also need some way to assert the type of the result of evaluating an expression, but we don't have any attributes or an
	// evaluator on hand.
	return
}

func (w *metricsWrapper) Execute(attrs attribute.Bag, mapper expr.Evaluator) (*Output, error) {
	result := &multierror.Error{}
	var values []adapter.Value

metadataLoop:
	for name, md := range w.metadata {
		metricValue, err := mapper.Eval(md.value, attrs)
		if err != nil {
			result = multierror.Append(result, fmt.Errorf("failed to eval metric value for metric '%s' with err: %s", name, err))
			continue metadataLoop // we can't satisfy this metric because we have the wrong types, so skip to the next
		}

		labels := make(map[string]interface{}, len(md.labels))
		for label, texpr := range md.labels {
			val, err := mapper.Eval(texpr, attrs)
			if err != nil {
				result = multierror.Append(result, fmt.Errorf("failed to construct value for metric '%s' with err: %s", name, err))
				continue metadataLoop // we can't satisfy this metric because we have the wrong types, so skip to the next
			}
			labels[label] = val
		}

		// TODO: investigate either pooling these, or keeping a set around that has only its field's values updated.
		// we could keep a map[metric name]value, iterate over the it updating only the fields in each value
		values = append(values, adapter.Value{
			Name:   name,
			Kind:   md.metricKind,
			Labels: labels,
			// TODO: how do we get times?
			StartTime:   time.Now(),
			EndTime:     time.Now(),
			MetricValue: metricValue,
		})
	}

	if err := w.aspect.Record(values); err != nil {
		result = multierror.Append(result, fmt.Errorf("failed to record all values with err: %s", err))
	}

	// We may accumulate errors while still being able to record some metrics; if we have any values at all we'll
	// return an OK alongside our errors (since presumably we're able to record at least a few).
	var out *Output
	if len(values) > 0 {
		out = &Output{Code: code.Code_OK}
	}
	return out, result.ErrorOrNil()
}

func (w *metricsWrapper) Close() error {
	return w.aspect.Close()
}

func toDefinition(desc *dpb.MetricDescriptor) adapter.MetricDefinition {
	labels := make(map[string]adapter.LabelType, len(desc.Labels))
	for _, label := range desc.Labels {
		labels[label.Name] = adapter.FromPbType(label.ValueType)
	}
	return adapter.MetricDefinition{
		Name:   desc.Name,
		Kind:   adapter.FromPbMetricKind(desc.Kind),
		Labels: labels,
	}
}

func find(defs []*aconfig.MetricsParams_Metric, name string) (*aconfig.MetricsParams_Metric, bool) {
	for _, def := range defs {
		if def.Descriptor_ == name {
			return def, true
		}
	}
	return nil, false
}
