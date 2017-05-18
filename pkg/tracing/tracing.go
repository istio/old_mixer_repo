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
	"fmt"

	ot "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
)

// Record attempts to record the operation and provided payload.
//
// TODO: opentracing supports richer logging types than this method's signature.
// Figure out how we want to handle things, and if strings are enough for now.
func Record(ctx context.Context, operation, format string, a ...interface{}) {
	span := ot.SpanFromContext(ctx)
	if span == nil {
		// tracing is disabled, since the interceptor didn't create a span for us.
		return
	}
	span.LogFields(log.String(operation, fmt.Sprintf(format, a...)))
}
