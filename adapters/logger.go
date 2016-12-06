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

package adapters

import (
	"istio.io/mixer"
)

// Logger is the interface for adapters that will handle logs data within
// the mixer.
type Logger interface {
	Adapter

	// Log directs a backend adapter to process a batch of facts derived
	// from potentially several Report() calls.
	Log([]mixer.FactValues) error

	// Flush instructs a backed adapter to flush its current state.
	//
	// Flush will be called by the mixer before an adapter is removed.
	// Implementations should use Flush to guarantee all logs have been
	// appropriately handled.
	Flush()
}
