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

package stackdriver

// This file contains the logic for munging timeseries when reporting DELTA metrics. Stackdriver represents DELTAs
// as cumulative metrics where no points have overlapping time intervals. This must be true for each metric stream,
// which is described by a Metric _and_ Monitored Resource pair.
// Note that Stackdriver stores times at microsecond precision, so our time comparisons are performed at that precision.
//
// For a set of TimeSeries we're pushing to Stackdriver we:
// - extract all DELTA TimeSeries
// - group them by (metric, monitored resource)
// - for each group we:
// 	* sort them by start time
// 	* walk the group in sorted order, merging all timeseries with overlapping time intervals.

import (
	"fmt"
	"sort"
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/genproto/googleapis/monitoring/v3"

	"istio.io/mixer/pkg/adapter"
)

const usec int32 = int32(1 * time.Microsecond)

// Type used to implement sorting of TimeSeries by start time, using microseconds as the unit of precision instead of nanos.
type byStartTimeUSec []*monitoring.TimeSeries

func (t byStartTimeUSec) Len() int      { return len(t) }
func (t byStartTimeUSec) Swap(i, j int) { t[i], t[j] = t[j], t[i] }
func (t byStartTimeUSec) Less(i, j int) bool {
	t1 := t[i].Points[0].Interval.StartTime
	t2 := t[j].Points[0].Interval.StartTime
	if t1.Seconds == t2.Seconds {
		return toUSec(t1.Nanos) < toUSec(t2.Nanos)
	}
	return t1.Seconds < t2.Seconds
}

// Used as our key to group TimeSeries
type key struct {
	metric *metricpb.Metric
	mr     *monitoredres.MonitoredResource
}

// For DELTA metrics, no two can have duplicate StartTimes. This func groups the TimeSeries by metric, and touches the times
// to ensure no two have duplicate start times. This assumes that all TimeSeries have exactly 1 point, since we're using
// them with a call to CreateTimeSeries (which has the same requirement).
//
// TODO: if we're really paranoid, we could maintain a map[metric]time.Time of the last (greatest) time sent for each metric.
// We could then reject any writes with time < the stored time, as a safeguard against metrics in different batches
// stomping on each other's times. This obvious doesn't scale to multiple Mixer instances.
func coalesce(series []*monitoring.TimeSeries, logger adapter.Logger) []*monitoring.TimeSeries {
	out := make([]*monitoring.TimeSeries, 0, len(series))
	byMetric := make(map[key][]*monitoring.TimeSeries)
	for _, ts := range series {
		if ts.MetricKind == metricpb.MetricDescriptor_DELTA {
			k := key{ts.Metric, ts.Resource}
			// Stackdriver does not support custom metrics of kinda DELTA, but if you send cumulative metrics with no
			// overlapping intervals then DELTA based queries work over the stream. We change the kind here to allow users
			// to create "DELTA" metrics in config, but have it actually work at the API level.
			ts.MetricKind = metricpb.MetricDescriptor_CUMULATIVE
			// Further, DELTAs cannot have the same start and end time, so we massage the data a bit.
			if compareUSec(ts.Points[0].Interval.StartTime, ts.Points[0].Interval.EndTime) == 0 {
				ts.Points[0].Interval.EndTime.Nanos += usec
			}
			byMetric[k] = append(byMetric[k], ts)
		} else {
			out = append(out, ts)
		}
	}

	for _, ts := range byMetric {
		logger.Infof("ts pre sort = %v", ts)
		sort.Sort(byStartTimeUSec(ts))
		logger.Infof("ts post sort = %v", ts)
		// now we walk the list, combining runs of timeseries with overlapping intervals into a single point.
		current := ts[0]
		logger.Infof("current = %v", current)
		for i := 1; i < len(ts); i++ {
			logger.Infof("comparing to %v", ts[i])
			if compareUSec(current.Points[0].Interval.EndTime, ts[i].Points[0].Interval.StartTime) >= 0 {
				// merge the two; if there's an error then both params are unchanged.
				var err error
				if current, err = merge(current, ts[i]); err != nil {
					logger.Infof("failed to merge timeseries: %v", err)
				}
			} else {
				// non-overlapping, move along
				out = append(out, current)
				current = ts[i]
			}
		}
		// Get the last one
		out = append(out, current)
	}
	return out
}

// Attempts to merge two timeseries; if they are not of the same type we return an error and a, unchanged, as the resulting timeseries.
// Given the way that stackdriver metrics work, and our grouping by (istio) metric name, metric, and monitored resource, we should
// never see two timeseries merged with different value types.
func merge(a, b *monitoring.TimeSeries) (*monitoring.TimeSeries, error) {
	var ok bool
	switch av := a.Points[0].Value.Value.(type) {
	case *monitoring.TypedValue_Int64Value:
		var bv *monitoring.TypedValue_Int64Value
		if bv, ok = b.Points[0].Value.Value.(*monitoring.TypedValue_Int64Value); !ok {
			return a, fmt.Errorf("can't merge two timeseries with different value types; a has int64 value, b does not: %#v", b.Points[0].Value)
		}
		a.Points[0].Value = &monitoring.TypedValue{&monitoring.TypedValue_Int64Value{av.Int64Value + bv.Int64Value}}
	case *monitoring.TypedValue_DoubleValue:
		var bv *monitoring.TypedValue_DoubleValue
		if bv, ok = b.Points[0].Value.Value.(*monitoring.TypedValue_DoubleValue); !ok {
			return a, fmt.Errorf("can't merge two timeseries with different value types; a has double value, b does not: %#v", b.Points[0].Value)
		}
		a.Points[0].Value = &monitoring.TypedValue{&monitoring.TypedValue_DoubleValue{av.DoubleValue + bv.DoubleValue}}
	case *monitoring.TypedValue_DistributionValue:
		if _, ok = b.Points[0].Value.Value.(*monitoring.TypedValue_DistributionValue); !ok {
			return a, fmt.Errorf("can't merge two timeseries with different value types; a is a distribution, b is not: %#v", b.Points[0].Value)
		}
		// TODO: combining distributions is hard. We know that they have the same buckets since they're all for the same metric, so that part is fine.
		// But we need to update counts of each bucket, as well as the mean, sum of squared deviation, and the range. This is non-trivial.
	default:
		// illegal anyway, since we can't have DELTA/CUMULATIVE metrics on anything else
		return a, fmt.Errorf("invalid type for DELTA metric: %v", a.Points[0].Value)
	}
	a.Points[0].Interval.EndTime = b.Points[0].Interval.EndTime
	return a, nil
}

// Compare returns < 0 if a < b, > 0 if a > b, and 0 if a == b.
// Note that Stackdriver stores times at microsecond precision, so our comparison is performed at that precision too.
func compareUSec(a, b *timestamp.Timestamp) int64 {
	if a.Seconds == b.Seconds {
		return int64(toUSec(a.Nanos) - toUSec(b.Nanos))
	}
	return a.Seconds - b.Seconds
}

func toUSec(nanos int32) int32 {
	return nanos / int32(time.Microsecond)
}
