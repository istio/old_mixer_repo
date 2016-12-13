// Copyright 2016 Google Inc.
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

package adapters

import "time"

const (
	// Unspecified is unknown category of metrics.
	Unspecified MetricKind = 0
	// Gauge is a metric that represents a single value measurement that can go
	// up or down arbitrarily.
	Gauge
	// Counter is a metric that represents a single value that only ever
	// increases.
	Counter
)

type (
	// MetricsReporter is the interface for adapters that will handle metrics
	// data within the mixer.
	MetricsReporter interface {
		Adapter

		// ReportMetrics directs a backend adapter to process a batch of
		// MetricValues derived from potentially several Report() calls.
		ReportMetrics([]MetricValue) error
	}

	// MetricKind represents the category of metric that is recorded in a metric
	// value.
	MetricKind int

	// MetricValue holds the value of a metric, along with dimensional and
	// other metadata.
	MetricValue struct {
		// Name is the unique name for the metric
		Name string

		// DisplayName is the short name for the metric
		DisplayName string

		// Description provides documentation on the metric
		Description string

		// Kind identifies the metric category of a MetricValue.
		Kind MetricKind

		// Unit identifies the unit of measurement for the metric.
		Unit string

		// StartTime identifies the start time of value collection.
		// TODO: document behavior if undefined
		StartTime time.Time

		// EndTime identifies the end time of value collection.
		EndTime time.Time

		// Labels are a set of key-value pairs for dimensional data.
		Labels map[string]string

		// BoolValue provides a boolean metric value.
		BoolValue bool

		// Int64Value provides an integer metric value.
		Int64Value int64

		// Float64Value provides a double-precision floating-point metric
		// value.
		Float64Value float64

		// StringValue provides a string-encoded metric value.
		StringValue string
	}
)
