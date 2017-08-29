// Copyright 2017 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package circonus

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	cgm "github.com/circonus-labs/circonus-gometrics"

	"istio.io/mixer/adapter/circonus/config"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/adapter/test"
)

var validTestGauge = int64(123)
var validTestCounter = int64(1)
var validTestHistogramString = "H[1.4e+08]=1"
var validTestHistogramVal = 146 * time.Millisecond
var validTestHistogramIntVal = 3459
var validTestHistogramIntString = "H[3.4e+03]=1"

func testServer(t *testing.T) *httptest.Server {
	f := func(w http.ResponseWriter, r *http.Request) {

		switch r.URL.Path {
		case "/metrics_endpoint": // submit metrics
			switch r.Method {
			case "POST":
				fallthrough
			case "PUT":
				defer func() {
					err := r.Body.Close()
					if err != nil {
						panic(err)
					}
				}()

				b, err := ioutil.ReadAll(r.Body)
				if err != nil {
					panic(err)
				}
				var r interface{}
				err = json.Unmarshal(b, &r)
				if err != nil {
					panic(err)
				}

				payload, ok := r.(map[string]interface{})
				if !ok {
					t.Errorf("empty interface, no payload: %v", b)
				}
			TestLoop:
				for key, value := range payload {

					switch key {
					case "test_counter":

						valFloat := value.(map[string]interface{})["_value"].(float64)
						if int64(valFloat) != validTestCounter {
							t.Errorf("Got test counter val %v, expected %v", valFloat, validTestCounter)
							w.WriteHeader(500)
							w.Header().Set("Content-Type", "application/json")
							fmt.Fprintln(w, "mismatch in test counter values")
							break TestLoop
						}

					case "test_gauge":

						// type assert to string, then strconv to int. ugh
						valString := value.(map[string]interface{})["_value"].(string)
						valInt, err := strconv.Atoi(valString)
						if err != nil {
							t.Errorf("failed to convert %v to int", valString)
						}

						if int64(valInt) != validTestGauge {
							t.Errorf("Got test gauge val %v, expected %v", valInt, validTestGauge)
							w.WriteHeader(500)
							w.Header().Set("Content-Type", "application/json")
							fmt.Fprintln(w, "mismatch in test gauge values")
							break TestLoop
						}
					case "test_histogram":
						valInterfaces := value.(map[string]interface{})["_value"].([]interface{})

						var histoVals []string
						// only one test value, but need to index because of []interface
						// https://github.com/golang/go/wiki/InterfaceSlice
						for _, s := range valInterfaces {
							histoVals = append(histoVals, s.(string))
						}

						if histoVals[0] != validTestHistogramString {
							t.Errorf("Got test hist val %v, expected %v", histoVals, validTestHistogramString)
							w.WriteHeader(500)
							w.Header().Set("Content-Type", "application/json")
							fmt.Fprintln(w, "mismatch in test histogram values")
							break TestLoop
						}
					case "test_histogram_int":
						valInterfaces := value.(map[string]interface{})["_value"].([]interface{})

						var histoVals []string
						// only one test value, but need to index because of []interface
						// https://github.com/golang/go/wiki/InterfaceSlice
						for _, s := range valInterfaces {
							histoVals = append(histoVals, s.(string))
						}

						if histoVals[0] != validTestHistogramIntString {
							t.Errorf("Got test hist int val %v, expected %v", histoVals, validTestHistogramIntString)
							w.WriteHeader(500)
							w.Header().Set("Content-Type", "application/json")
							fmt.Fprintln(w, "mismatch in test histogram values")
							break TestLoop
						}
					}

				}

				w.WriteHeader(200)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintln(w, `{"stats":1}`)

			default:
				w.WriteHeader(500)
				fmt.Fprintln(w, "unsupported method")
			}
		default:
			msg := fmt.Sprintf("not found %s", r.URL.Path)
			w.WriteHeader(404)
			fmt.Fprintln(w, msg)
		}
	}
	return httptest.NewServer(http.HandlerFunc(f))
}

var (
	gaugeNoLabels = &adapter.MetricDefinition{
		Name:        "test_gauge",
		Description: "test of gauge metric",
		Kind:        adapter.Gauge,
		Labels:      map[string]adapter.LabelType{},
	}

	histogramNoLabels = &adapter.MetricDefinition{
		Name:        "test_histogram",
		Description: "test of histogram metric",
		Kind:        adapter.Distribution,
		Labels:      map[string]adapter.LabelType{},
	}

	histogramIntNoLabels = &adapter.MetricDefinition{
		Name:        "test_histogram_int",
		Description: "test of histogram int64 metric",
		Kind:        adapter.Distribution,
		Labels:      map[string]adapter.LabelType{},
	}

	counterNoLabels = &adapter.MetricDefinition{
		Name:        "test_counter",
		Description: "test of counter metric",
		Kind:        adapter.Counter,
		Labels:      map[string]adapter.LabelType{},
	}
)

func TestInvariants(t *testing.T) {
	test.AdapterInvariants(Register, t)
}

func TestNewMetricsAspect(t *testing.T) {
	conf := &config.Params{
		SubmissionUrl: "http://127.0.0.1:56104/test",
	}
	env := test.NewEnv(t)
	if _, err := newBuilder().NewMetricsAspect(env, conf, nil); err != nil {
		t.Errorf("b.NewMetrics(test.NewEnv(t), &config.Params{}) = %s, wanted no err", err)
	}

}

func TestRecord(t *testing.T) {

	validGauge := newGaugeVal(validTestGauge)
	invalidGauge := validGauge
	invalidGauge.MetricValue = "bar"

	validCounter := newCounterVal(validTestCounter)
	invalidCounter := validCounter
	invalidCounter.MetricValue = 1.0

	requestDuration := newHistogramVal(validTestHistogramVal)
	int64Distribution := newHistogramIntVal(int64(validTestHistogramIntVal))
	invalidDistribution := newHistogramVal("not good")

	cases := []struct {
		vals      []adapter.Value
		errString string
	}{
		{[]adapter.Value{}, ""},
		{[]adapter.Value{validGauge}, ""},
		{[]adapter.Value{validCounter}, ""},
		{[]adapter.Value{requestDuration}, ""},
		{[]adapter.Value{int64Distribution}, ""},
		{[]adapter.Value{validCounter, validGauge, int64Distribution}, ""},
		{[]adapter.Value{validCounter, validGauge}, "could not combine counter and gauge"},
		{[]adapter.Value{invalidCounter}, "could not record"},
		{[]adapter.Value{invalidGauge}, "could not record"},
		{[]adapter.Value{invalidDistribution}, "could not record"},
		{[]adapter.Value{validGauge, invalidGauge}, "could not record"},
	}

	testServer := testServer(t)
	defer testServer.Close()

	submissionURL := testServer.URL + "/metrics_endpoint"

	conf := &config.Params{
		SubmissionUrl: submissionURL,
	}

	// create a test circonus metrics instance
	cmc := &cgm.Config{}
	cmc.CheckManager.Check.SubmissionURL = submissionURL
	cmc.Debug = true
	cm, err := cgm.NewCirconusMetrics(cmc)
	if err != nil {
		t.Errorf("could not create new cgm %v", err)
	}

	for idx, c := range cases {

		b := newBuilder()

		m, err := b.NewMetricsAspect(test.NewEnv(t), conf, nil)
		if err != nil {
			t.Errorf("[%d] newBuilder().NewMetrics(test.NewEnv(t), conf) = _, %s; wanted no err", idx, err)
			continue
		}

		asp := m.(*aspect)
		asp.cm = *cm

		if err := m.Record(c.vals); err != nil {
			if c.errString == "" {
				t.Errorf("[%d] m.Record(c.vals) = %s; wanted no err", idx, err)
			}
			if !strings.Contains(err.Error(), c.errString) {
				t.Errorf("[%d] m.Record(c.vals) = %s; wanted err containing %s", idx, err.Error(), c.errString)
			}
		}

		cm.Flush() // flush the metric
		if err := m.Close(); err != nil {
			t.Errorf("[%d] m.Close() = %s; wanted no err", idx, err)
		}
		if c.errString != "" {
			continue
		}
	}
}

func newGaugeVal(val interface{}) adapter.Value {
	return adapter.Value{
		Definition:  gaugeNoLabels,
		Labels:      map[string]interface{}{},
		MetricValue: val,
	}
}

func newCounterVal(val interface{}) adapter.Value {
	return adapter.Value{
		Definition:  counterNoLabels,
		Labels:      map[string]interface{}{},
		MetricValue: val,
	}
}

func newHistogramVal(val interface{}) adapter.Value {
	return adapter.Value{
		Definition:  histogramNoLabels,
		Labels:      map[string]interface{}{},
		MetricValue: val,
	}
}

func newHistogramIntVal(val interface{}) adapter.Value {
	return adapter.Value{
		Definition:  histogramIntNoLabels,
		Labels:      map[string]interface{}{},
		MetricValue: val,
	}
}
