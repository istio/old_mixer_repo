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

// Package stdioLogger provides an implementation of the mixer logger aspect
// that writes logs (serialized as JSON) to a standard stream (stdout | stderr).
package stdioLogger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/golang/protobuf/proto"
	"istio.io/mixer/adapter/stdioLogger/config"
	"istio.io/mixer/pkg/aspect"
	"istio.io/mixer/pkg/aspect/logger"
	"istio.io/mixer/pkg/registry"

	me "github.com/hashicorp/go-multierror"
)

type (
	adapter    struct{}
	aspectImpl struct {
		logStream     io.Writer
		payloadFormat payloadFormat
		timestampFmt  string
	}

	payloadFormat int

	logEntry struct {
		Name          string                 `json:"logName,omitempty"`
		Timestamp     string                 `json:"timestamp,omitempty"`
		Severity      string                 `json:"severity,omitempty"`
		Labels        map[string]interface{} `json:"labels,omitempty"`
		TextPayload   string                 `json:"textPayload,omitempty"`
		StructPayload map[string]interface{} `json:"structPayload,omitempty"`
	}
)

const (
	textFmt   payloadFormat = iota
	structFmt payloadFormat = iota
)

// Register adds the stdioLogger adapter to the list of logger.Aspects known to
// mixer.
func Register(r registry.Registrar) error { return r.RegisterLogger(&adapter{}) }

func (a *adapter) Name() string { return "istio/stdioLogger" }
func (a *adapter) Description() string {
	return "Writes structured log entries to a standard I/O stream"
}
func (a *adapter) DefaultConfig() proto.Message                               { return &config.Params{} }
func (a *adapter) Close() error                                               { return nil }
func (a *adapter) ValidateConfig(cfg proto.Message) (ce *aspect.ConfigErrors) { return nil }
func (a *adapter) NewAspect(env aspect.Env, cfg proto.Message) (logger.Aspect, error) {
	c := cfg.(*config.Params)

	w := os.Stderr
	if c.LogStream == config.Params_STDOUT {
		w = os.Stdout
	}

	pFmt := textFmt
	if c.PayloadFormat == config.Params_STRUCTURED {
		pFmt = structFmt
	}

	tFmt := time.RFC3339
	if c.TimestampFormat != "" {
		// TODO: validation of timestamp format
		tFmt = c.TimestampFormat
	}

	return &aspectImpl{
		logStream:     w,
		payloadFormat: pFmt,
		timestampFmt:  tFmt,
	}, nil
}

func (a *aspectImpl) Close() error { return nil }
func (a *aspectImpl) Log(l []logger.Entry) error {
	var errors *me.Error
	entries, err := a.entries(l)
	if err != nil {
		errors = me.Append(errors, err)
	}
	for _, le := range entries {
		if err := writeJSON(a.logStream, le); err != nil {
			errors = me.Append(errors, err)
		}
	}

	return errors.ErrorOrNil()
}

func (a *aspectImpl) entries(le []logger.Entry) ([]logEntry, error) {
	var errors *me.Error
	entries := make([]logEntry, 0, len(le))
	for _, e := range le {
		entry, err := a.entry(e)
		if err != nil {
			errors = me.Append(errors, fmt.Errorf("failed to build logStream entry: %v", err))
			continue
		}
		entries = append(entries, entry)
	}
	return entries, errors.ErrorOrNil()
}

func (a *aspectImpl) entry(l logger.Entry) (logEntry, error) {
	e := logEntry{
		Name:      l.LogName,
		Severity:  l.Severity,
		Labels:    l.Labels,
		Timestamp: l.Timestamp.Format(a.timestampFmt),
	}

	if l.Payload != "" {
		switch a.payloadFormat {
		case structFmt:
			if err := json.Unmarshal([]byte(l.Payload), &e.StructPayload); err != nil {
				return logEntry{}, fmt.Errorf("could not unmarshal struct payload: %v", err)
			}
		default:
			e.TextPayload = l.Payload
		}
	}

	return e, nil
}

func writeJSON(w io.Writer, le logEntry) error {
	return json.NewEncoder(w).Encode(le)
}
