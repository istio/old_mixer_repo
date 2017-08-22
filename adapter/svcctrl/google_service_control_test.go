package svcctrl

import (
	"reflect"
	"time"
	"testing"

	sc "google.golang.org/api/servicecontrol/v1"

	"github.com/davecgh/go-spew/spew"
)

func TestHandleMetric(t *testing.T) {

	timeNow := time.Now().Format(time.RFC3339Nano)
	request, err := handleMetric(timeNow, "")

	if err != nil {
		t.Fatalf("handleMetric() failed with:%v", err)
	}

	metric_value := int64(1)
	expected_req := &sc.ReportRequest{
		Operations: []*sc.Operation{
			{
				OperationName: "reportMetrics",
				StartTime: timeNow,
				EndTime:   timeNow,
				Labels: map[string]string{
					"cloud.googleapis.com/location": "global",
				},
				MetricValueSets: []*sc.MetricValueSet{
					{
						MetricName: "serviceruntime.googleapis.com/api/producer/request_count",
						MetricValues: []*sc.MetricValue{
							{
								StartTime:  timeNow,
								EndTime:    timeNow,
								Int64Value: &metric_value,
							},
						},
					},
				},
			},
		},
	}

	cfg := spew.NewDefaultConfig()
	cfg.DisablePointerAddresses = true
	if !reflect.DeepEqual(*expected_req, *request) {
		t.Errorf("expect op1 == op2, but op1 = %v, op2 = %v", cfg.Sdump(*expected_req), cfg.Sdump(*request))
	}
}

