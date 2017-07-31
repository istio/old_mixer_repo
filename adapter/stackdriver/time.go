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

// This file contains the logic for munging timestamps when reporting DELTA metrics. Stackdriver represents DELTAs
// as cumulative metrics where no points have overlapping time intervals. This must be true for each metric stream,
// which is described by a Metric _and_ Monitored Resource pair.
// Note that Stackdriver stores times at microsecond precision, so our time comparisons are performed at that precision.
//
// For a set of TimeSeries we're pushing to Stackdriver we:
// - extract all DELTA TimeSeries
// - group them by (metric, monitored resource)
// - for each group we:
// 	* set time interval end time = time interval start time (we're reporting a point in time)
// 	* sort them by start time
// 	* walk the group in sorted order, ensuring no two have the same start time. If they do, we increment the second
//    point by a microsecond (smallest possible unit), resort the remaining points, and continue.

import (
	"sort"
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/genproto/googleapis/monitoring/v3"
)

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
func massageTimes(series []*monitoring.TimeSeries) []*monitoring.TimeSeries {
	out := make([]*monitoring.TimeSeries, 0, len(series))
	byMetric := make(map[key][]*monitoring.TimeSeries)
	for _, ts := range series {
		if ts.MetricKind == metricpb.MetricDescriptor_DELTA {
			k := key{ts.Metric, ts.Resource}
			byMetric[k] = append(byMetric[k], ts)
		} else {
			out = append(out, ts)
		}
	}

	// TODO: evaluate alternate impl: merge adjacent (EndTime[i] == StartTime[i+1]) points together, adding their `Value`s.
	// Results in less data to SD overall, may behave better too.
	for _, ts := range byMetric {
		sort.Sort(byStartTimeUSec(ts))

		// Ensure the first TimeSeries is a single point in time; we take care of the rest in the loop.
		// We don't need to assert that at least a single TimeSeries exists because we wouldn't be in the loop body if there was not at least one.
		ts[0].Points[0].Interval.EndTime = ts[0].Points[0].Interval.StartTime
		for i := 1; i < len(ts); i++ {
			// We always have exactly one point, no need to check.
			a := ts[i-1].Points[0].Interval
			b := ts[i].Points[0].Interval
			// a > b can never happen because the input is sorted in ascending order; a < b is our normal happy path.
			// When a == b, we need to massage the time slightly to avoid overlap.
			if compareUSec(a.StartTime, b.StartTime) == 0 {
				// same time, we need to adjust the second slightly (smallest unit we can) to avoid overlap
				b.StartTime.Nanos += int32(1 * time.Microsecond)
				// Changing the time could invalidate the previous sorting
				sort.Sort(byStartTimeUSec(ts[i:]))
			}
			// Ensure our TimeSeries is always a single point in time, to avoid having to deal with overlap.
			b.EndTime = b.StartTime
		}
		out = append(out, ts...)
	}
	return out
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
