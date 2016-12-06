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

package defaultLogger

import (
	"bytes"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"istio.io/mixer"
	"istio.io/mixer/adapters"
)

var (
	testReport = mixer.FactValues{
		StringValues: map[string]string{
			"service":           "bookstore.example.com",
			"timestamp":         "02/Jan/2006:15:04:05 -0700",
			"apiMethod":         "GetShelves",
			"responseCodeClass": "2xx",
			"user":              "testuser@example.com",
		},
		IntValues: map[string]int64{
			"responseCode": 200,
			"requestCount": 1,
			"requestBytes": 345,
		},
		FloatValues: map[string]float64{
			"requestDuration": 0.012334,
		},
		BoolValues: map[string]bool{
			"clusterInternal": false,
		},
	}
	altReport = mixer.FactValues{
		StringValues: map[string]string{
			"service":           "bookstore.example.com",
			"timestamp":         "02/Jan/2008:10:14:15 -0700",
			"apiMethod":         "ListBooks",
			"responseCodeClass": "4xx",
			"user":              "testuser@example.com",
		},
		IntValues: map[string]int64{
			"responseCode": 404,
			"requestCount": 1,
			"requestBytes": 13,
		},
		FloatValues: map[string]float64{
			"requestDuration": 0.05,
		},
		BoolValues: map[string]bool{
			"clusterInternal": true,
		},
	}
)

func TestLog(t *testing.T) {

	tests := []struct {
		name      string
		conf      Config
		reports   []mixer.FactValues
		want      []string
		expectErr bool
	}{
		{
			name:    "Single Report",
			conf:    Config{Template: "{{.service}} - {{.user}} [{{.timestamp}}] {{.apiMethod}} {{.responseCode}} {{.requestBytes}} ({{.requestDuration}}) {{.clusterInternal}}"},
			reports: []mixer.FactValues{testReport},
			want:    []string{"bookstore.example.com - testuser@example.com [02/Jan/2006:15:04:05 -0700] GetShelves 200 345 (0.012334) false"},
		},
		{
			name:    "Batch Reports",
			conf:    Config{Template: "{{.service}} - {{.user}} [{{.timestamp}}] {{.apiMethod}} {{.responseCode}} {{.requestBytes}} ({{.requestDuration}}) {{.clusterInternal}}"},
			reports: []mixer.FactValues{testReport, altReport},
			want: []string{
				"bookstore.example.com - testuser@example.com [02/Jan/2006:15:04:05 -0700] GetShelves 200 345 (0.012334) false",
				"bookstore.example.com - testuser@example.com [02/Jan/2008:10:14:15 -0700] ListBooks 404 13 (0.05) true",
			},
		},
		{
			name:      "Bad Config",
			conf:      Config{Template: "{{.service"},
			expectErr: true,
		},
	}

	b := NewBuilder()

	for _, v := range tests {
		a, err := b.NewAdapter(v.conf)
		if err != nil {
			if !v.expectErr {
				t.Errorf("%s - could not build adapter: %v", v.name, err)
			}
			continue
		}
		if err == nil && v.expectErr {
			t.Errorf("%s - expected error building adapter", v.name)
		}

		l := a.(adapters.Logger)

		old := os.Stdout // for restore
		r, w, _ := os.Pipe()
		os.Stdout = w // redirecting
		// copy over the output from stderr
		outC := make(chan string)
		go func() {
			var buf bytes.Buffer
			io.Copy(&buf, r)
			outC <- buf.String()
		}()

		l.Log(v.reports)

		// back to normal state
		w.Close()
		os.Stdout = old
		logs := <-outC

		got := strings.Split(strings.TrimSpace(logs), "\n")

		if !reflect.DeepEqual(got, v.want) {
			t.Errorf("%s - got %s, want %s", v.name, got, v.want)
		}
	}

}
