package googleServiceControl

import (
	"testing"
)

func TesthandleMetric(t *testing.T) {
	request, err := handleMetric()
	if err != nil {
		t.Fatalf("handleMetric() failed with:%v", err)
	}

	if len(request.Operations) != 1 {
		t.Error(`len(request.Operations()) != 1`)
	}

	operation := request.Operations[0]
	if operation.Labels["cloud.googleapis.com/location"] != "global" {
		t.Error(`operation.Labels["cloud.googleapis.com/location"] != "global"`)
	}

	if len(operation.MetricValueSets) != 1 {
		t.Fatal(`len(operation.MetricValueSets) != 1`)
	}
	metricValueSet := operation.MetricValueSets[0]

	if metricValueSet.MetricName != "serviceruntime.googleapis.com/api/producer/request_count" {
		t.Error(`metricValueSet.MetricName != "serviceruntime.googleapis.com/api/producer/request_count"`)
	}

	if len(metricValueSet.MetricValues) != 1 {
		t.Fatal(`len(metricValueSet.MetricValues) != 1`)
	}

	if *metricValueSet.MetricValues[0].Int64Value != 1 {
		t.Error(`*metricValueSet.MetricValues[0].Int64Value != 1`)
	}
}
