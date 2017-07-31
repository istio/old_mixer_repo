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
)

// shorthand to save us some chars in test cases
type ts []*monitoring.TimeSeries

func makeTS(m *metric.Metric, mr *monitoredres.MonitoredResource, seconds int64, micros int32) *monitoring.TimeSeries {
	return &monitoring.TimeSeries{
		Metric:     m,
		Resource:   mr,
		MetricKind: metric.MetricDescriptor_DELTA,
		Points: []*monitoring.Point{{
			Interval: &monitoring.TimeInterval{
				StartTime: &timestamp.Timestamp{Seconds: seconds, Nanos: micros * int32(time.Microsecond)},
				EndTime:   &timestamp.Timestamp{Seconds: seconds, Nanos: micros * int32(time.Microsecond)},
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

func TestMassageTimes(t *testing.T) {
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
	tests := []struct {
		name string
		in   ts
		out  ts // we assert that the output ordering matches out's ordering exactly.
	}{
		{"empty",
			ts{},
			ts{}},
		{"one",
			ts{makeTS(m1, mr1, 1, 5)},
			ts{makeTS(m1, mr1, 1, 5)}},
		{"dupe",
			ts{makeTS(m1, mr1, 1, 5), makeTS(m1, mr1, 1, 5)},
			ts{makeTS(m1, mr1, 1, 5), makeTS(m1, mr1, 1, 6)}},
		{"out of order",
			ts{makeTS(m1, mr1, 2, 5), makeTS(m1, mr1, 1, 5)},
			ts{makeTS(m1, mr1, 1, 5), makeTS(m1, mr1, 2, 5)},
		},
		{"reversed",
			ts{makeTS(m1, mr1, 4, 1), makeTS(m1, mr1, 3, 1), makeTS(m1, mr1, 2, 1), makeTS(m1, mr1, 1, 1)},
			ts{makeTS(m1, mr1, 1, 1), makeTS(m1, mr1, 2, 1), makeTS(m1, mr1, 3, 1), makeTS(m1, mr1, 4, 1)},
		},
		{"cascading conflicts",
			ts{makeTS(m1, mr1, 1, 1), makeTS(m1, mr1, 1, 1), makeTS(m1, mr1, 1, 2), makeTS(m1, mr1, 1, 3)},
			ts{makeTS(m1, mr1, 1, 1), makeTS(m1, mr1, 1, 2), makeTS(m1, mr1, 1, 3), makeTS(m1, mr1, 1, 4)},
		},
	}

	for idx, tt := range tests {
		t.Run(fmt.Sprintf("[%d] %s", idx, tt.name), func(t *testing.T) {
			if len(tt.in) != len(tt.out) {
				t.Fatalf("Expected in and out to be the same size, got size %d and %d respective.", len(tt.in), len(tt.out))
			}
			out := massageTimes(tt.in)
			for idx, expected := range tt.out {
				if !reflect.DeepEqual(out[idx], expected) {
					t.Errorf("out[%d] = %v, after massaging expected %v", idx, out[idx], expected)
				}
			}
		})
	}
}
