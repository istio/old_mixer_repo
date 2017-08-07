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

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/genproto/googleapis/monitoring/v3"

	"istio.io/mixer/pkg/adapter/test"
)

// shorthand to save us some chars in test cases
type ts []*monitoring.TimeSeries

func makeTS(m *metric.Metric, mr *monitoredres.MonitoredResource, seconds int64, micros int32) *monitoring.TimeSeries {
	return makeTSFull(m, mr, seconds, micros, 1, metric.MetricDescriptor_DELTA)
}

func makeTSDelta(m *metric.Metric, mr *monitoredres.MonitoredResource, seconds int64, micros int32, val int64) *monitoring.TimeSeries {
	return makeTSFull(m, mr, seconds, micros, val, metric.MetricDescriptor_DELTA)
}

func makeTSCumulative(m *metric.Metric, mr *monitoredres.MonitoredResource, seconds int64, micros int32, val int64) *monitoring.TimeSeries {
	return makeTSFull(m, mr, seconds, micros, val, metric.MetricDescriptor_CUMULATIVE)
}

func makeTSFull(m *metric.Metric, mr *monitoredres.MonitoredResource, seconds int64, micros int32, value int64,
	kind metric.MetricDescriptor_MetricKind) *monitoring.TimeSeries {

	return &monitoring.TimeSeries{
		Metric:     m,
		Resource:   mr,
		MetricKind: kind,
		Points: []*monitoring.Point{{
			Value: &monitoring.TypedValue{&monitoring.TypedValue_Int64Value{value}},
			Interval: &monitoring.TimeInterval{
				StartTime: &timestamp.Timestamp{Seconds: seconds, Nanos: micros * usec},
				EndTime:   &timestamp.Timestamp{Seconds: seconds, Nanos: (micros * usec) + usec},
			},
		}},
	}
}

func TestToUSec(t *testing.T) {
	now := time.Now()
	pbnow, _ := ptypes.TimestampProto(now)
	if int32(now.Nanosecond()/int(time.Microsecond)) != toUSec(pbnow.Nanos) {
		t.Fatalf("toUSec(%d) = %d, expected it to be equal to %v / time.Microsecond", pbnow.Nanos, toUSec(pbnow.Nanos), now.Nanosecond())
	}
}

func TestByStartTimeUSec(t *testing.T) {
	tests := []struct {
		name string
		in   ts
		out  ts
	}{
		{"empty", ts{}, ts{}},
		{"singleton",
			ts{makeTS(nil, nil, 1, 1)},
			ts{makeTS(nil, nil, 1, 1)},
		},
		{"reverse order s",
			ts{makeTS(nil, nil, 2, 1), makeTS(nil, nil, 1, 1)},
			ts{makeTS(nil, nil, 1, 1), makeTS(nil, nil, 2, 1)},
		},
		{"reverse order us",
			ts{makeTS(nil, nil, 1, 2), makeTS(nil, nil, 1, 1)},
			ts{makeTS(nil, nil, 1, 1), makeTS(nil, nil, 1, 2)},
		},
		{"overlapping",
			ts{makeTS(nil, nil, 1, 1), makeTS(nil, nil, 1, 1)},
			ts{makeTS(nil, nil, 1, 1), makeTS(nil, nil, 1, 1)},
		},
		{"out of order",
			ts{makeTS(nil, nil, 1, 3), makeTS(nil, nil, 2, 1), makeTS(nil, nil, 1, 1)},
			ts{makeTS(nil, nil, 1, 1), makeTS(nil, nil, 1, 3), makeTS(nil, nil, 2, 1)},
		},
	}

	for idx, tt := range tests {
		t.Run(fmt.Sprintf("[%d] %s", idx, tt.name), func(t *testing.T) {
			if len(tt.in) != len(tt.out) {
				t.Fatalf("Expected in and out to be the same size, got size %d and %d respective.", len(tt.in), len(tt.out))
			}
			sort.Sort(byStartTimeUSec(tt.in))
			for idx, expected := range tt.out {
				if !reflect.DeepEqual(tt.in[idx], expected) {
					t.Errorf("tt.in[%d] = %v, expected %v", idx, tt.in[idx], expected)
				}
			}
		})
	}
}

func TestCoalesce(t *testing.T) {
	m1 := &metric.Metric{
		Type:   "m1",
		Labels: map[string]string{},
	}
	mr1 := &monitoredres.MonitoredResource{
		Type:   "mr1",
		Labels: map[string]string{},
	}
	// TODO: we don't currently test multiple series (different metrics/MRs) as it complicates the test logic:
	// the order that they're returned can be random, where all the elements in the series are correctly ordered but the
	// series themselves are not the order we want in `tests.out`; this was causing flakes in the test.

	// This is based on the test input for the "cascading conflicts" test case; we merge several adjacent TSs and the
	// result is a time interval from (1s1ns, 1s4ns) which our helper functions makes hard to construct in line.
	cascadingConflictsOut := makeTSCumulative(m1, mr1, 1, 1, 10)
	cascadingConflictsOut.Points[0].Interval.EndTime.Nanos += 2 * usec

	doubleInput1 := makeTSDelta(m1, mr1, 1, 5, 456)
	doubleInput1.Points[0].Value = &monitoring.TypedValue{&monitoring.TypedValue_DoubleValue{4.5}}
	doubleInput2 := makeTSDelta(m1, mr1, 1, 5, 456)
	doubleInput2.Points[0].Value = &monitoring.TypedValue{&monitoring.TypedValue_DoubleValue{4.5}}
	doubleOutput := makeTSCumulative(m1, mr1, 1, 5, 456)
	doubleOutput.Points[0].Value = &monitoring.TypedValue{&monitoring.TypedValue_DoubleValue{9.0}}

	tests := []struct {
		name string
		in   ts
		out  ts // we assert that the output ordering matches out's ordering exactly.
	}{
		{"empty",
			ts{},
			ts{}},
		{"one",
			ts{makeTSDelta(m1, mr1, 1, 5, 456)},
			ts{makeTSCumulative(m1, mr1, 1, 5, 456)}},
		{"dupe",
			ts{makeTSDelta(m1, mr1, 1, 5, 1), makeTSDelta(m1, mr1, 1, 5, 1)},
			ts{makeTSCumulative(m1, mr1, 1, 5, 2)}},
		{"out of order",
			ts{makeTSDelta(m1, mr1, 2, 5, 1), makeTSDelta(m1, mr1, 1, 5, 1)},
			ts{makeTSCumulative(m1, mr1, 1, 5, 1), makeTSCumulative(m1, mr1, 2, 5, 1)},
		},
		{"reversed",
			ts{makeTSDelta(m1, mr1, 4, 1, 1), makeTSDelta(m1, mr1, 3, 1, 1), makeTSDelta(m1, mr1, 2, 1, 1), makeTSDelta(m1, mr1, 1, 1, 1)},
			ts{makeTSCumulative(m1, mr1, 1, 1, 1), makeTSCumulative(m1, mr1, 2, 1, 1), makeTSCumulative(m1, mr1, 3, 1, 1), makeTSCumulative(m1, mr1, 4, 1, 1)},
		},
		{"cascading conflicts",
			ts{makeTSDelta(m1, mr1, 1, 1, 1), makeTSDelta(m1, mr1, 1, 1, 2), makeTSDelta(m1, mr1, 1, 3, 3), makeTSDelta(m1, mr1, 1, 2, 4)},
			ts{cascadingConflictsOut},
		},
		{"conflicting and nonconflicting",
			ts{makeTSDelta(m1, mr1, 1, 1, 7), makeTSDelta(m1, mr1, 1, 1, 3), makeTSDelta(m1, mr1, 1, 7, 4896), makeTSDelta(m1, mr1, 1, 5, 9485367)},
			ts{makeTSCumulative(m1, mr1, 1, 1, 10), makeTSCumulative(m1, mr1, 1, 5, 9485367), makeTSCumulative(m1, mr1, 1, 7, 4896)},
		},
		{"double",
			ts{doubleInput1, doubleInput2},
			ts{doubleOutput},
		},
	}

	for idx, tt := range tests {
		t.Run(fmt.Sprintf("[%d] %s", idx, tt.name), func(t *testing.T) {
			out := coalesce(tt.in, test.NewEnv(t))
			if len(out) != len(tt.out) {
				t.Fatalf("coalesce(%v) = %v, len = %d, expectd len == %d", tt.in, out, len(out), len(tt.out))
			}
			for idx, expected := range tt.out {
				if !reflect.DeepEqual(out[idx], expected) {
					for i, ts := range out {
						t.Logf("out[%d] = %v", i, ts)
					}
					t.Errorf("out[%d] = %v, after coalescing expected %v", idx, out[idx], expected)
				}
			}
		})
	}
}
