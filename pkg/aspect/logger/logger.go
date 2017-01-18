// Copyright 2017 Google Inc.
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

package logger

import (
	"time"

	"github.com/golang/protobuf/proto"
	"istio.io/mixer/pkg/aspect"
)

type (
	// Aspect is the interface for adapters that will handle logs data
	// within the mixer.
	Aspect interface {
		aspect.Aspect

		// Log directs a backend adapter to process a batch of
		// log entries derived from potentially several Report() calls.
		Log([]Entry) error
	}

	// Entry is the set of data that together constitutes a log entry.
	Entry struct {
		// LogName is the name of the log in which to record this entry.
		LogName string `json:"logName",omitempty`
		// Labels are a set of metadata associated with this entry.
		// For instance, Labels can be used to describe the monitored
		// resource that corresponds to the log entry.
		Labels map[string]interface{} `json:"labels",omitempty`
		// Timestamp is the time value for the log entry
		Timestamp time.Time `json:"timestamp,omitempty"`
		// Severity indicates the log level for the log entry.
		// Example levels: "INFO", "WARNING", "500"
		Severity string `json:"severity",omitempty`
		// TextPayload is textual logs data for which the entry is being
		// generated.
		TextPayload string `json:"textPayload,omitempty"`
		// StructPayload is a structured set of data, extracted from
		// a serialized JSON Object, that represent the actual data
		// for which the entry is being generated.
		StructPayload map[string]interface{} `json:"structPayload,omitempty"`
	}

	// Adapter is the interface for building Aspect instances for mixer
	// logging backends.
	Adapter interface {
		aspect.Adapter

		// NewAspect returns a new Logger implementation, based on the
		// supplied Aspect configuration for the backend.
		NewAspect(env aspect.Env, config proto.Message) (Aspect, error)
	}
)
