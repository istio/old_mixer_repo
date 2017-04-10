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

package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	rpc "github.com/googleapis/googleapis/google/rpc"

	"istio.io/mixer/pkg/adapter"
)

func makeAPIRequest(handler http.Handler, method, url string, data []byte, t *testing.T) (int, []byte) {
	httpRequest, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	httpRequest.Header.Set("Content-Type", "application/json")
	if err != nil {
		t.Fatal(err)
	}
	httpWriter := httptest.NewRecorder()
	handler.ServeHTTP(httpWriter, httpRequest)
	result := httpWriter.Result()
	body, err := ioutil.ReadAll(result.Body)
	if err != nil {
		t.Fatal(err)
	}
	return result.StatusCode, body
}

func TestAPI_getRules(t *testing.T) {
	uri := "/scopes/scope/subjects/subject/rules"
	val := "Value"
	store := &fakeMemStore{
		data: map[string]string{
			uri: val,
		},
	}
	api := NewAPI("v1", 0, nil, nil,
		nil, nil, store)

	for _, ctx := range []struct {
		key    string
		val    string
		status int
	}{
		{uri, val, http.StatusOK},
		{"/scopes/scope/subjects/subject1/rules", val, http.StatusNotFound},
	} {
		t.Run(ctx.key, func(t *testing.T) {

			sc, body := makeAPIRequest(api.handler, "GET", "/api/v1"+ctx.key, []byte{}, t)
			if sc != ctx.status {
				t.Errorf("http status got %d\nwant %d", sc, ctx.status)
			}
			b := string(body)
			if sc == http.StatusOK {
				if !reflect.DeepEqual(ctx.val, b) {
					t.Errorf("incorrect value got [%#v]\nwant [%#v]", b, ctx.val)
				}
			}
		})
	}

}

func readBody(err string) readBodyFunc {
	return func(r io.Reader) (b []byte, e error) {
		if err != "" {
			e = errors.New(err)
		}
		return
	}
}

func validate(err string) validateFunc {
	return func(cfg map[string]string) (rt *Validated, ce *adapter.ConfigErrors) {
		if err != "" {
			ce = ce.Appendf("main", err)
		}
		return
	}
}

type errorPhase int

const (
	errNone errorPhase = iota
	errStoreRead
	errStoreWrite
	errReadyBody
	errValidate
)

func TestAPI_putRules(t *testing.T) {
	key := "/scopes/scope/subjects/subject/rules"
	val := "Value"

	for _, tst := range []struct {
		msg    string
		phase  errorPhase
		status int
	}{
		{"Created ", errNone, http.StatusOK},
		{"store write error", errStoreWrite, http.StatusInternalServerError},
		{"store read error", errStoreRead, http.StatusInternalServerError},
		{"request read error", errReadyBody, http.StatusInternalServerError},
		{"config validation error", errValidate, http.StatusPreconditionFailed},
	} {
		t.Run(tst.msg, func(t *testing.T) {

			store := &fakeMemStore{
				data: map[string]string{
					key: val,
				},
			}
			api := NewAPI("v1", 0, nil, nil,
				nil, nil, store)

			switch tst.phase {
			case errStoreRead:
				store.err = errors.New(tst.msg)
			case errStoreWrite:
				store.writeErr = errors.New(tst.msg)
			case errReadyBody:
				api.readBody = readBody(tst.msg)
			case errValidate:
				api.validate = validate(tst.msg)
			}
			sc, bbody := makeAPIRequest(api.handler, "PUT", "/api/v1"+key, []byte{}, t)
			if sc != tst.status {
				t.Errorf("http status got %d\nwant %d", sc, tst.status)
			}
			body := string(bbody)
			if !strings.Contains(body, tst.msg) {
				t.Errorf("got %s\nwant %s", body, tst.msg)
			}
		})
	}
}

// TODO define a new envelope message
// with a place for preparsed message?

func TestAPI_httpStatusToRPC(t *testing.T) {
	for _, tst := range []struct {
		http int
		code rpc.Code
	}{
		{http.StatusOK, rpc.OK},
		{http.StatusTeapot, rpc.UNKNOWN},
	} {
		t.Run(fmt.Sprintf("%s", tst.code), func(t *testing.T) {
			c := httpStatusToRPC(tst.http)
			if c != tst.code {
				t.Errorf("got %s\nwant %s", c, tst.code)
			}
		})
	}
}

type fakeresp struct {
	err   error
	value interface{}
}

// bin/linter.sh has a lint exclude for this.
func (f *fakeresp) WriteHeaderAndJson(status int, value interface{}, contentType string) error {
	f.value = value
	return f.err
}
func (f *fakeresp) Write(bytes []byte) (int, error) {
	f.value = bytes
	return 0, f.err
}

// Ensures that
func TestAPI_writeError(t *testing.T) {
	// ensures that logging is performed
	err := errors.New("always error")
	for _, tst := range []struct {
		http    int
		code    rpc.Code
		message string
	}{
		{http.StatusTeapot, rpc.UNKNOWN, "Teapot"},
		{http.StatusOK, rpc.OK, "OK"},
		{http.StatusNotFound, rpc.NOT_FOUND, "Not Found"},
	} {
		t.Run(fmt.Sprintf("%s", tst.code), func(t *testing.T) {
			resp := &fakeresp{err: err}
			writeResponse(tst.http, tst.message, resp)
			rpcStatus, ok := resp.value.(rpc.Status)
			if !ok {
				t.Error("failed to produce rpc.Status")
				return
			}
			if rpcStatus.Code != int32(tst.code) {
				t.Errorf("got %d\nwant %s", rpcStatus.Code, tst.code)
			}
		})
	}

	// ensure write is logged
	resp := &fakeresp{err: err}
	val := "new info"
	write(val, resp)
	if !reflect.DeepEqual([]byte(val), resp.value) {
		t.Errorf("got %v\nwant %v", resp.value, val)
	}
}

func TestAPI_Run(t *testing.T) {
	api := NewAPI("v1", 9094, nil, nil, nil, nil, nil)
	go api.Run()
	err := api.Server.Close()
	if err != nil {
		t.Errorf("unexpected failure while closing %s", err)
	}
}
