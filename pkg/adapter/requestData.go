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

package adapter

import "context"

// Service defines the service specific details.
type Service struct {
	// Fully qualified name of the service.
	FullName string
}

// RequestData defines information about a request, for example details about the destination service.
//
// This data is delivered to the adapters through the passed in context object.
// Adapter can retrieve this data from the context by invoking `RequestDataFromContext`.
type RequestData struct {
	// Details about the destination service.
	Destination Service
}

// RequestDataKey is the name of the key that holds the RequestData information within the context.
const RequestDataKey = "requestdata"

// RequestDataFromContext retrieves the RequestData object contained inside the given context.
// Returns false if the given context does not contains a valid RequestData object.
func RequestDataFromContext(ctx context.Context) (RequestData, bool) {
	reqData, ok := ctx.Value(RequestDataKey).(RequestData)
	return reqData, ok
}
