// Copyright 2017 the Istio Authors.
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

package stackdriver

import (
	"context"
	"fmt"
	"sync"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3"
	"github.com/golang/glog"
	"github.com/golang/protobuf/ptypes"
	"github.com/googleapis/gax-go"
	xcontext "golang.org/x/net/context"
	gapiopts "google.golang.org/api/option"
	labelpb "google.golang.org/genproto/googleapis/api/label"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"

	"istio.io/mixer/adapter/stackdriver/config"
	"istio.io/mixer/pkg/adapter"
)

// TODO: implement adapter validation
// TODO: change batching to be size aware: right now we batch and send data to stackdriver based on only a ticker.
// Ideally we'd also size our buffer and send data whenever we hit the size limit or config.push_interval time has passed
// since the last push.

type (

	// Abstracts the creation of the stackdriver client to enable network-less testing.
	createClientFunc func(*config.Params) (*monitoring.MetricClient, error)

	factory struct {
		adapter.DefaultBuilder
		createClient createClientFunc
	}

	sdinfo struct {
		ttype string
		kind  metricpb.MetricDescriptor_MetricKind
		value metricpb.MetricDescriptor_ValueType
	}

	// Abstracts over client.CreateTimeSeries for testing
	pushFunc func(ctx xcontext.Context, req *monitoringpb.CreateTimeSeriesRequest, opts ...gax.CallOption) error

	sd struct {
		// TODO: remove when env is request scoped
		env adapter.Env // used for logging

		projectID   string
		metricInfo  map[string]sdinfo
		client      *monitoring.MetricClient
		pushMetrics pushFunc
		// Stackdriver's SDK doesn't perform batching, though their API supports it.
		// We'll roll our own batching by aggregating timeseries and periodically sending them to Stackdriver.
		t      *time.Ticker // we hold on to a ref so we can stop it during Close()
		m      sync.Mutex   // guards toSend
		toSend []*monitoringpb.TimeSeries
	}
)

const (
	adapterName = "stackdriver"
	adapterDesc = "Publishes StackDriver metricInfo, logs, and traces."

	// From https://github.com/GoogleCloudPlatform/golang-samples/blob/master/monitoring/custommetric/custommetric.go
	customMetricPrefix = "custom.googleapis.com/"
)

var (
	// TODO: evaluate how we actually want to do this mapping - this first stab w/ everything as String probably
	// isn't what we really want.
	// The better path forward is probably to constrain the input types and err on bad combos.
	labelMap = map[adapter.LabelType]labelpb.LabelDescriptor_ValueType{
		adapter.String:       labelpb.LabelDescriptor_STRING,
		adapter.Int64:        labelpb.LabelDescriptor_INT64,
		adapter.Float64:      labelpb.LabelDescriptor_INT64,
		adapter.Bool:         labelpb.LabelDescriptor_BOOL,
		adapter.Time:         labelpb.LabelDescriptor_STRING,
		adapter.Duration:     labelpb.LabelDescriptor_STRING,
		adapter.IPAddress:    labelpb.LabelDescriptor_STRING,
		adapter.EmailAddress: labelpb.LabelDescriptor_STRING,
		adapter.URI:          labelpb.LabelDescriptor_STRING,
		adapter.DNSName:      labelpb.LabelDescriptor_STRING,
		adapter.StringMap:    labelpb.LabelDescriptor_STRING,
	}
)

// Register records the builders exposed by this adapter.
func Register(r adapter.Registrar) {
	r.RegisterMetricsBuilder(newFactory(createClient))
}

func newFactory(createClient createClientFunc) factory {
	return factory{DefaultBuilder: adapter.NewDefaultBuilder(adapterName, adapterDesc, &config.Params{}), createClient: createClient}
}

func (f factory) Close() error { return nil }

func createClient(cfg *config.Params) (*monitoring.MetricClient, error) {
	return monitoring.NewMetricClient(context.Background(), toOpts(cfg)...)
}

// We keep this function separate from createClient to enable easy testing
func toOpts(cfg *config.Params) (opts []gapiopts.ClientOption) {
	switch cfg.Creds.(type) {
	case *config.Params_ApiKey:
		opts = append(opts, gapiopts.WithAPIKey(cfg.GetApiKey()))
	case *config.Params_ServiceAccountPath:
		opts = append(opts, gapiopts.WithServiceAccountFile(cfg.GetServiceAccountPath()))
	case *config.Params_AppCredentials:
		// When using default app credentials the SDK handles everything for us.
	}
	if cfg.Endpoint != "" {
		opts = append(opts, gapiopts.WithEndpoint(cfg.Endpoint))
	}
	return
}

// NewMetricsAspect provides an implementation for adapter.MetricsBuilder.
func (f factory) NewMetricsAspect(env adapter.Env, c adapter.Config, metrics map[string]*adapter.MetricDefinition) (adapter.MetricsAspect, error) {
	cfg := c.(*config.Params)
	types := make(map[string]sdinfo, len(metrics))
	for name, def := range metrics {
		info, found := cfg.MetricInfo[name]
		if !found {
			env.Logger().Warningf("No stackdriver info found for metric %s, skipping it", name)
			continue
		}
		// TODO: do we want to make sure that the definition conforms to stackdrvier requirements? Really that needs to happen during config validation
		types[name] = sdinfo{
			ttype: metricType(def.Name),
			kind:  info.Kind,
			value: info.Value,
		}
	}

	// Per the documentation on config.proto, if push_interval is zero we'll default to a 1s push interval
	if cfg.PushInterval == time.Duration(0) {
		cfg.PushInterval = 1 * time.Second
	}

	var err error
	var client *monitoring.MetricClient
	// TODO: in theory this client could live in the factory and be shared amongst many adapter instances
	if client, err = f.createClient(cfg); err != nil {
		return nil, env.Logger().Errorf("Failed to construct stackdriver client: %v", err)
	}

	s := &sd{
		env:         env,
		projectID:   cfg.ProjectId,
		client:      client,
		pushMetrics: client.CreateTimeSeries,
		metricInfo:  types,
		t:           time.NewTicker(cfg.PushInterval),
		m:           sync.Mutex{},
	}
	s.startTicker(env)
	return s, nil
}

// Extracted from NewMetricsAspect for testing - it shouldn't be possible to get an Env any other time.
func (s *sd) startTicker(env adapter.Env) {
	env.ScheduleDaemon(func() {
		for range s.t.C {
			s.pushData()
		}
	})
}

func (s *sd) Record(vals []adapter.Value) error {
	glog.Info("stackdriver.Record called with %d vals", len(vals))

	// TODO: len(vals) is constant for config lifetime, consider pooling
	data := make([]*monitoringpb.TimeSeries, 0, len(vals))
	for _, val := range vals {
		info, found := s.metricInfo[val.Definition.Name]
		if !found {
			// We weren't configured with stackdriver data about this metric, so we don't know how to publish it.
			glog.Infof("Skipping metric %s due to not being configured with stackdriver info about it.", val.Definition.Name)
			continue
		}

		end, _ := ptypes.TimestampProto(val.EndTime)
		data = append(data, &monitoringpb.TimeSeries{
			Metric: &metricpb.Metric{
				Type:   info.ttype,
				Labels: toStringMap(val.Labels),
			},
			// TODO: handle MRs; today we publish all metrics to SD's global MR because it's easy.
			Resource: &monitoredres.MonitoredResource{
				Type: "global",
				Labels: map[string]string{
					"project_id": s.projectID,
				},
			},
			MetricKind: info.kind,
			ValueType:  info.value,
			// Since we're sending a `CreateTimeSeries` request we can only populate a single point, see
			// the documentation on the `points` field: https://cloud.google.com/monitoring/api/ref_v3/rest/v3/TimeSeries
			Points: []*monitoringpb.Point{{
				Interval: &monitoringpb.TimeInterval{
					// StartTime is only used for DELTA metrics; all of our metrics are custom so
					// start time can only cause errors. Plus the API defaults to setting StartTime = EndTime
					// if no StartTime is provided: https://cloud.google.com/monitoring/api/ref_v3/rest/v3/TimeInterval
					EndTime: end,
				},
				Value: toTypedVal(val.MetricValue, val.Definition.Value)},
			},
		})
	}
	s.m.Lock()
	s.toSend = append(s.toSend, data...)
	s.m.Unlock()
	return nil
}

func (s *sd) pushData() {
	l := s.env.Logger()

	l.Infof("Pushing data to Stackdriver")
	s.m.Lock()
	if len(s.toSend) == 0 {
		s.m.Unlock()
		l.Infof("No data to send to Stackdriver")
		return
	}
	// Take the ref to the data we're pushing and create a new one to be written in to. We assume it'll be similarly
	// sized to the last one.
	// TODO: evaluate just swapping between two arrays (old, new) rather than creating new ones. Could run into
	// problems if latency is high (> cfg.PushInterval) due to sending a lot of data in one go. Otherwise, maybe pool things
	toSend := s.toSend
	s.toSend = make([]*monitoringpb.TimeSeries, 0, len(toSend))
	s.m.Unlock()
	l.Infof("Pushing %d timeseries to stackdriver", len(toSend))

	err := s.pushMetrics(context.Background(),
		&monitoringpb.CreateTimeSeriesRequest{
			Name:       monitoring.MetricProjectPath(s.projectID),
			TimeSeries: toSend,
		})

	// TODO: this is executed in a daemon, so we can't get out info about errors other than logging.
	// We need to build framework level support for these kinds of async tasks. Perhaps a generic batching adapter
	// can handle some of this complexity?
	l.Infof("Stackdriver returned: %v", err)
}

func (s *sd) Close() error {
	s.t.Stop()
	return s.client.Close()
}

func toStringMap(in map[string]interface{}) map[string]string {
	out := make(map[string]string, len(in))
	for key, val := range in {
		out[key] = fmt.Sprintf("%v", val)
	}
	return out
}

func toTypedVal(val interface{}, t adapter.LabelType) *monitoringpb.TypedValue {
	switch labelMap[t] {
	case labelpb.LabelDescriptor_BOOL:
		return &monitoringpb.TypedValue{&monitoringpb.TypedValue_BoolValue{BoolValue: val.(bool)}}
	case labelpb.LabelDescriptor_INT64:
		return &monitoringpb.TypedValue{&monitoringpb.TypedValue_Int64Value{Int64Value: val.(int64)}}
	case labelpb.LabelDescriptor_STRING:
		fallthrough
	default:
		return &monitoringpb.TypedValue{&monitoringpb.TypedValue_StringValue{StringValue: fmt.Sprintf("%v", val)}}
	}
}

func metricType(name string) string {
	// TODO: figure out what, if anything, we need to do to sanitize these.
	return customMetricPrefix + name
}
