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

package tracing

import (
	"context"
	"testing"

	ot "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/mocktracer"
)

func TestRecord(t *testing.T) {
	ctx := context.Background()
	Record(ctx, "test op", "payload with no args")
	if span := ot.SpanFromContext(ctx); span != nil {
		t.Errorf("Expected no span in context, got %v", span)
	}

	span := mocktracer.New().StartSpan("test")
	ctx = ot.ContextWithSpan(ctx, span)
	Record(ctx, "operation", "log")

	mock := span.(*mocktracer.MockSpan)
	if len(mock.Logs()) < 1 {
		t.Error("Nothing logged after call to record, expected log entries")
	}
	if mock.Logs()[0].Fields[0].Key != "operation" {
		t.Errorf("Expected log field key to be 'operation', got %s", mock.Logs()[0].Fields[0].Key)
	}
}
