// Copyright 2017 Istio Authors.
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

// Package noop is an empty adapter implementing every aspect.
// WARNING: Not intended for actual use. This is a stand-in adapter used in benchmarking Mixer's adapter framework.
package noop

import (
	rpc "github.com/googleapis/googleapis/google/rpc"

	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/status"
)

type (
	AccessLogsBuilder struct{ adapter.Builder }
	AppLogsBuilder    struct{ adapter.Builder }
	AttrBuilder       struct{ adapter.Builder }
	DenialsBuilder    struct{ adapter.Builder }
	ListBuilder       struct{ adapter.Builder }
	MetricBuilder     struct{ adapter.Builder }
	QuotaBuilder      struct{ adapter.Builder }

	accessLogsAspect struct{}
	appLogsAspect    struct{}
	attrAspect       struct{}
	denialsAspect    struct{}
	listAspect       struct{}
	metricAspect     struct{}
	quotaAspect      struct{}
)

// Register registers the no-op adapter as every aspect.
func Register(r adapter.Registrar) {
	r.RegisterAccessLogsBuilder(AccessLogsBuilder{})
	r.RegisterApplicationLogsBuilder(AppLogsBuilder{})
	r.RegisterAttributesGeneratorBuilder(AttrBuilder{})
	r.RegisterDenialsBuilder(DenialsBuilder{})
	r.RegisterListsBuilder(ListBuilder{})
	r.RegisterMetricsBuilder(MetricBuilder{})
	r.RegisterQuotasBuilder(QuotaBuilder{})
}

func (AttrBuilder) BuildAttributesGenerator(adapter.Env, adapter.Config) (adapter.AttributesGenerator, error) {
	return &attrAspect{}, nil
}
func (attrAspect) Generate(map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}
func (attrAspect) Close() error { return nil }

func (AccessLogsBuilder) NewAccessLogsAspect(adapter.Env, adapter.Config) (adapter.AccessLogsAspect, error) {
	return &accessLogsAspect{}, nil
}
func (accessLogsAspect) LogAccess([]adapter.LogEntry) error { return nil }
func (accessLogsAspect) Close() error                       { return nil }

func (AppLogsBuilder) NewApplicationLogsAspect(adapter.Env, adapter.Config) (adapter.ApplicationLogsAspect, error) {
	return &appLogsAspect{}, nil
}
func (appLogsAspect) Log([]adapter.LogEntry) error { return nil }
func (appLogsAspect) Close() error                 { return nil }

func (DenialsBuilder) NewDenialsAspect(adapter.Env, adapter.Config) (adapter.DenialsAspect, error) {
	return &denialsAspect{}, nil
}
func (denialsAspect) Deny() rpc.Status { return status.New(rpc.FAILED_PRECONDITION) }
func (denialsAspect) Close() error     { return nil }

func (ListBuilder) NewListsAspect(adapter.Env, adapter.Config) (adapter.ListsAspect, error) {
	return &listAspect{}, nil
}
func (listAspect) CheckList(symbol string) (bool, error) { return false, nil }
func (listAspect) Close() error                          { return nil }

func (MetricBuilder) NewMetricsAspect(adapter.Env, adapter.Config, map[string]*adapter.MetricDefinition) (adapter.MetricsAspect, error) {
	return &metricAspect{}, nil
}
func (metricAspect) Record([]adapter.Value) error { return nil }
func (metricAspect) Close() error                 { return nil }

func (QuotaBuilder) NewQuotasAspect(env adapter.Env, c adapter.Config, quotas map[string]*adapter.QuotaDefinition) (adapter.QuotasAspect, error) {
	return &quotaAspect{}, nil
}
func (quotaAspect) Alloc(adapter.QuotaArgs) (adapter.QuotaResult, error) {
	return adapter.QuotaResult{}, nil
}
func (quotaAspect) AllocBestEffort(adapter.QuotaArgs) (adapter.QuotaResult, error) {
	return adapter.QuotaResult{}, nil
}
func (quotaAspect) ReleaseBestEffort(adapter.QuotaArgs) (int64, error) { return 0, nil }
func (quotaAspect) Close() error                                       { return nil }
