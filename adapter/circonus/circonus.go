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

package circonus

import (
	"fmt"

	cgm "github.com/circonus-labs/circonus-gometrics"
	multierror "github.com/hashicorp/go-multierror"

	"istio.io/mixer/adapter/circonus/config"
	"istio.io/mixer/pkg/adapter"
)

type (
	builder struct {
		adapter.DefaultBuilder
	}

	aspect struct {
		cm cgm.CirconusMetrics
	}
)

var (
	name        = "circonus"
	desc        = "Pushes Circonus metrics"
	defaultConf = &config.Params{
		SubmissionUrl: "",
	}
)

// Register records the builders exposed by this adapter.
func Register(r adapter.Registrar) {
	r.RegisterMetricsBuilder(newBuilder())
}

func newBuilder() *builder {
	return &builder{adapter.NewDefaultBuilder(name, desc, defaultConf)}
}

func (*builder) NewMetricsAspect(env adapter.Env, cfg adapter.Config, metrics map[string]*adapter.MetricDefinition) (adapter.MetricsAspect, error) {

	params := cfg.(*config.Params)
	cmc := &cgm.Config{}
	cmc.CheckManager.Check.SubmissionURL = params.SubmissionUrl
	cm, err := cgm.NewCirconusMetrics(cmc)
	if err != nil {
		err = env.Logger().Errorf("Could not create NewCirconusMetrics: %v", err)
		return nil, err
	}

	return &aspect{*cm}, nil
}

func (a *aspect) Record(values []adapter.Value) error {
	var result *multierror.Error

	for _, v := range values {
		if err := a.record(v); err != nil {
			result = multierror.Append(result, err)
		}
	}
	return result.ErrorOrNil()
}

func (a *aspect) record(value adapter.Value) error {
	mname := value.Definition.Name

	switch value.Definition.Kind {

	case adapter.Gauge:
		v, err := value.Int64()
		if err != nil {
			return fmt.Errorf("could not record gauge '%s': %v", mname, err)
		}
		a.cm.Gauge(mname, v)

	case adapter.Counter:
		_, err := value.Int64()
		if err != nil {
			return fmt.Errorf("could not record counter '%s': %v", mname, err)
		}
		a.cm.Increment(mname)

	case adapter.Distribution:
		v, err := value.Duration()
		if err == nil {
			a.cm.Timing(mname, float64(v))
			return nil
		}
		vint, err := value.Int64()
		if err == nil {
			a.cm.Timing(mname, float64(vint))
			return nil
		}
		return fmt.Errorf("could not record distribution '%s': %v", mname, err)

	}

	return nil
}

func (a *aspect) Close() error { return nil }
