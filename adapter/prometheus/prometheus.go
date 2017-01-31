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

// Package prometheus publishes metric values collected by the mixer for
// ingestion by prometheus.
package prometheus

import (
	"fmt"

	"istio.io/mixer/adapter/prometheus/config"
	"istio.io/mixer/pkg/adapter"
)

type (
	factory struct {
		adapter.DefaultBuilder

		srv server
	}

	prom struct{}
)

var (
	name = "prometheus"
	desc = "Publishes prometheus metrics"
	conf = &config.Params{}
)

// Register records the builders exposed by this adapter.
func Register(r adapter.Registrar) {
	r.RegisterMetrics(newFactory())
}

func newFactory() factory {
	srv := newServer(defaultAddr)
	if err := srv.Start(); err != nil {
		panic(fmt.Errorf("could not create prometheus adapter: %v", err))
	}
	return factory{adapter.NewDefaultBuilder(name, desc, conf), srv}
}

func (f factory) Close() error {
	return f.srv.Close()
}

func (factory) NewMetrics(env adapter.Env, cfg adapter.AspectConfig) (adapter.MetricsAspect, error) {
	return &prom{}, nil
}

func (*prom) Record([]adapter.Value) error {
	return nil
}

func (*prom) Close() error { return nil }
