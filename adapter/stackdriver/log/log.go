// Copyright 2017 the Istio Authors.
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

package log

import (
	"context"
	"fmt"
	"io"
	"text/template"
	"time"

	"cloud.google.com/go/logging"
	xctx "golang.org/x/net/context"
	"google.golang.org/api/option"

	"istio.io/mixer/adapter/stackdriver/config"
	"istio.io/mixer/adapter/stackdriver/helper"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/pool"
	"istio.io/mixer/template/logentry"
)

type (
	makeClientFn func(ctx xctx.Context, projectID string, opts ...option.ClientOption) (*logging.Client, error)

	logFn func(logging.Entry)

	builder struct {
		makeClient makeClientFn
		types      map[string]*logentry.Type
	}

	info struct {
		labels []string
		tmpl   *template.Template
		req    *config.Params_LogInfo_HttpRequestMapping
	}

	handler struct {
		now func() time.Time // used for testing

		logger adapter.Logger
		client io.Closer
		log    logFn
		info   map[string]info
	}
)

var (
	_ logentry.HandlerBuilder = &builder{}
	_ logentry.Handler        = &handler{}
)

// NewBuilder returns a builder implementing the logentry.HandlerBuilder interface.
func NewBuilder() logentry.HandlerBuilder {
	return &builder{makeClient: logging.NewClient}
}

func (b *builder) ConfigureLogEntryHandler(types map[string]*logentry.Type) error {
	b.types = types
	return nil
}

func (b *builder) Build(c adapter.Config, env adapter.Env) (adapter.Handler, error) {
	logger := env.Logger()
	cfg := c.(*config.Params)
	infos := make(map[string]info)
	for name, log := range cfg.LogInfo {
		_, found := b.types[name]
		if !found {
			logger.Infof("configured with log info about %s which is not an Istio metric", name)
			continue
		}
		tmpl, err := template.New(name).Parse(log.PayloadTemplate)
		if err != nil {
			_ = logger.Errorf("failed to evaluate template for log %s, skipping: %v", name, err)
			continue
		}
		infos[name] = info{
			labels: log.LabelNames,
			tmpl:   tmpl,
			req:    log.HttpMapping,
		}
	}

	client, err := b.makeClient(context.Background(), cfg.ProjectId, helper.ToOpts(cfg)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create stackdriver logging client: %v", err)
	}
	return &handler{log: client.Logger("istio-mixer").Log, client: client, now: time.Now, logger: logger}, nil
}

func (h *handler) HandleLogEntry(_ context.Context, values []*logentry.Instance) error {
	for _, v := range values {
		linfo, found := h.info[v.Name]
		if !found {
			h.logger.Warningf("got instance for unknown log '%s', skipping", v.Name)
			continue
		}

		buf := pool.GetBuffer()
		if err := linfo.tmpl.Execute(buf, v.Variables); err != nil {
			// We'll just continue on with an empty payload for this entry - we could still be populating the HTTP req with valuable info, for example.
			_ = h.logger.Errorf("failed to execute template for log '%s': %v", v.Name, err)
		}
		payload := buf.String()
		pool.PutBuffer(buf)

		h.log(logging.Entry{
			Timestamp:   h.now(), // TODO: use timestamp on Instance when timestamps work
			Severity:    logging.ParseSeverity(v.Severity),
			LogName:     v.Name,
			Labels:      toLabelMap(linfo.labels, v.Variables),
			Payload:     payload,
			HTTPRequest: toReq(linfo.req, v.Variables),
		})
	}
	return nil
}

func (h *handler) Close() error {
	return h.client.Close()
}

func toLabelMap(names []string, variables map[string]interface{}) map[string]string {
	out := make(map[string]string, len(names))
	for _, name := range names {
		v := variables[name]
		switch vt := v.(type) {
		case string:
			out[name] = vt
		default:
			out[name] = fmt.Sprintf("%v", v)
		}
	}
	return out
}

func toReq(mapping *config.Params_LogInfo_HttpRequestMapping, variables map[string]interface{}) *logging.HTTPRequest {
	req := &logging.HTTPRequest{}
	if mapping != nil {
		reqs := variables[mapping.RequestSize]
		reqsize, ok := toInt(reqs)
		if ok {
			req.RequestSize = reqsize
		}

		resps := variables[mapping.ResponseSize]
		respsize, ok := toInt(resps)
		if ok {
			req.ResponseSize = respsize
		}

		code := variables[mapping.Status]
		status, ok := code.(int)
		if ok {
			req.Status = status
		}

		l := variables[mapping.Latency]
		latency, ok := l.(time.Duration)
		if ok {
			req.Latency = latency
		}

		lip := variables[mapping.LocalIp]
		localip, ok := lip.(string)
		if ok {
			req.LocalIP = localip
		}

		rip := variables[mapping.RemoteIp]
		remoteip, ok := rip.(string)
		if ok {
			req.RemoteIP = remoteip
		}
	}
	return req
}

func toInt(v interface{}) (int64, bool) {
	// case int, int8, int16, ...: return int64(i), true
	// does not compile because go can't handle the type conversion for multiple types in a single branch.
	switch i := v.(type) {
	case int:
		return int64(i), true
	case int64:
		return i, true
	default:
		return 0, false
	}
}
