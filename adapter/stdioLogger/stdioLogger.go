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
		logStream    io.Writer
		timestampFmt string
	}

	logEntry struct {
		logger.Entry
		timestampFmt string `json:"-"`
	}
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

	tFmt := time.RFC3339
	if c.TimestampFormat != "" {
		// TODO: validation of timestamp format
		tFmt = c.TimestampFormat
	}

	return &aspectImpl{
		logStream:    w,
		timestampFmt: tFmt,
	}, nil
}

func (a *aspectImpl) Close() error { return nil }
func (a *aspectImpl) Log(l []logger.Entry) error {
	var errors *me.Error
	entries := a.entries(l)
	for _, le := range entries {
		if err := writeJSON(a.logStream, le); err != nil {
			errors = me.Append(errors, err)
		}
	}

	return errors.ErrorOrNil()
}

func (a *aspectImpl) entries(le []logger.Entry) []logEntry {
	entries := make([]logEntry, 0, len(le))
	for _, e := range le {
		entries = append(entries, logEntry{e, a.timestampFmt})
	}
	return entries
}

// NOTE: this is added to support control for timestamp serialization
func (l logEntry) MarshalJSON() ([]byte, error) {
	type Alias logEntry
	return json.Marshal(&struct {
		Timestamp string `json:"timestamp"`
		Alias
	}{
		Timestamp: l.Timestamp.Format(l.timestampFmt),
		Alias:     (Alias)(l),
	})
}

func writeJSON(w io.Writer, le logEntry) error {
	return json.NewEncoder(w).Encode(le)
}
