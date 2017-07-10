// Copyright 2017 Istio Authors
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

package serviceControl

import (
	"reflect"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"google.golang.org/api/servicecontrol/v1"

	"istio.io/mixer/adapter/serviceControl/config"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/adapter/test"
)

const testServiceName = "gcp-service"

var (
	defaultParams = &config.Params{ServiceName: testServiceName}
	fakeService   = new(servicecontrol.Service)
)

type fakeClient struct {
}

func (*fakeClient) create(logger adapter.Logger) (*servicecontrol.Service, error) {
	return fakeService, nil
}

/*
type fakeServicesService struct {
	*servicecontrol.ServicesService
}

func (* fakeServicesService) Report(serviceName string, reportrequest *servicecontrol.ReportRequest) *servicecontrol.ServicesReportCall{
    fmt.Print(reportrequest)
	return nil
}

	func (r *fakeServicesService) AllocateQuota(serviceName string, allocatequotarequest *servicecontrol.AllocateQuotaRequest) *servicecontrol.ServicesAllocateQuotaCall{
		return nil
	}
		func (r *fakeServicesService) Check(serviceName string, checkrequest *servicecontrol.CheckRequest) *servicecontrol.ServicesCheckCall {
return nil
	}
		func (r *fakeServicesService) EndReconciliation(serviceName string, endreconciliationrequest *servicecontrol.EndReconciliationRequest) *servicecontrol.ServicesEndReconciliationCall {

			return nil
	}
		func (r *fakeServicesService) ReleaseQuota(serviceName string, releasequotarequest *servicecontrol.ReleaseQuotaRequest) *servicecontrol.ServicesReleaseQuotaCall {

			return nil
	}
		func (r *fakeServicesService) StartReconciliation(serviceName string, startreconciliationrequest *servicecontrol.StartReconciliationRequest) *servicecontrol.ServicesStartReconciliationCall {

			return nil
	}
*/
func TestAdapterInvariants(t *testing.T) {
	test.AdapterInvariants(Register, t)
}

func TestBuilder_NewAspect(t *testing.T) {
	e := test.NewEnv(t)
	b := builder{adapter.DefaultBuilder{}, new(fakeClient)}
	a, err := b.newAspect(e, defaultParams)
	if err != nil {
		t.Errorf("NewApplicationLogsAspect(env, %s) => unexpected error: %v", defaultParams, err)
	}
	if x := a.serviceName; x != testServiceName {
		t.Errorf("NewApplicationLogsAspect(env, %s) => service name actual: %s", defaultParams, x)
	}

	if a.service != fakeService {
		t.Errorf("NewApplicationLogsAspect(env, %s) => create service control client fail", defaultParams)
	}

	if x := a.logger; x != e.Logger() {
		t.Errorf("NewApplicationLogsAspect(env, %s) mismatching logger actual: %v", x)
	}
}

func TestLogger_Close(t *testing.T) {
	a := new(aspect)
	if err := a.Close(); err != nil {
		t.Errorf("Close() => unexpected error: %v", err)
	}
}

func TestLogger_Log(t *testing.T) {
	a := &aspect{
		testServiceName,
		new(servicecontrol.Service),
		test.NewEnv(t).Logger(),
	}

	l := adapter.LogEntry{LogName: "istio_log", TextPayload: "text payload", Timestamp: "2017-Jan-09", Severity: adapter.Info}

	tests := []struct {
		input []adapter.LogEntry
		want  servicecontrol.ReportRequest
	}{
		{[]adapter.LogEntry{l},
			servicecontrol.ReportRequest{
				Operations: []*servicecontrol.Operation{
					{
						OperationId:   "test_operation",
						OperationName: "reportLogs",
						StartTime:     "2017-07-01T10:10:05Z",
						EndTime:       "2017-07-01T10:10:05Z",
						LogEntries: []*servicecontrol.LogEntry{
							{
								Name:        "istio_log",
								Severity:    "INFO",
								TextPayload: "text payload",
								Timestamp:   "2017-Jan-09",
							},
						},
						Labels: map[string]string{"cloud.googleapis.com/location": "global"},
					},
				},
			},
		},
	}

	for _, v := range tests {
		r := a.log(v.input, time.Date(2017, time.July, 1, 10, 10, 5, 2, time.Local), "test_operation")
		if !reflect.DeepEqual(*r, v.want) {
			t.Errorf("log(%+v) => %v, want %v", v.input, spew.Sdump(r), spew.Sdump(v.want))
		}
	}
}

/*
func TestLogger_LogFailure(t *testing.T) {
	tw := &testWriter{errorOnWrite: true}
	textPayloadEntry := adapter.LogEntry{LogName: "istio_log", TextPayload: "text payload", Timestamp: "2017-Jan-09", Severity: adapter.Info}
	baseAspectImpl := &logger{tw}

	if err := baseAspectImpl.Log([]adapter.LogEntry{textPayloadEntry}); err == nil {
		t.Error("Log() should have produced error")
	}
}

func TestLogger_LogAccess(t *testing.T) {
	tw := &testWriter{lines: make([]string, 0)}

	noLabelsEntry := adapter.LogEntry{LogName: "access_log"}
	labelsEntry := adapter.LogEntry{LogName: "access_log", Labels: map[string]interface{}{"test": false, "val": 42}}
	labelsWithTextEntry := adapter.LogEntry{
		LogName:     "access_log",
		Labels:      map[string]interface{}{"test": false, "val": 42},
		TextPayload: "this is a log line",
	}

	baseLog := `{"logName":"access_log"}`
	labelsLog := `{"logName":"access_log","labels":{"test":false,"val":42}}`
	labelsWithTextLog := `{"logName":"access_log","labels":{"test":false,"val":42},"textPayload":"this is a log line"}`

	tests := []struct {
		input []adapter.LogEntry
		want  []string
	}{
		{[]adapter.LogEntry{}, []string{}},
		{[]adapter.LogEntry{noLabelsEntry}, []string{baseLog}},
		{[]adapter.LogEntry{labelsEntry}, []string{labelsLog}},
		{[]adapter.LogEntry{labelsWithTextEntry}, []string{labelsWithTextLog}},
	}

	for _, v := range tests {
		log := &logger{tw}
		if err := log.LogAccess(v.input); err != nil {
			t.Errorf("LogAccess(%v) => unexpected error: %v", v.input, err)
		}
		if !reflect.DeepEqual(tw.lines, v.want) {
			t.Errorf("LogAccess(%v) => %v, want %s", v.input, tw.lines, v.want)
		}
		tw.lines = make([]string, 0)
	}
}

func TestLogger_LogAccessFailure(t *testing.T) {
	tw := &testWriter{errorOnWrite: true}
	entry := adapter.LogEntry{LogName: "access_log"}
	l := &logger{tw}

	if err := l.LogAccess([]adapter.LogEntry{entry}); err == nil {
		t.Error("LogAccess() should have produced error")
	}
}


func (t *testWriter) Write(p []byte) (n int, err error) {
	if t.errorOnWrite {
		return 0, errors.New("write error")
	}
	t.count++
	t.lines = append(t.lines, strings.Trim(string(p), "\n"))
	return len(p), nil
}
*/
