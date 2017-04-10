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

// Package config APIServer defines and implements the config API.
// The server constructs and uses a validator for validations
// The server uses KVStore to persists keys.

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"

	restful "github.com/emicklei/go-restful"
	"github.com/golang/glog"
	rpc "github.com/googleapis/googleapis/google/rpc"

	"istio.io/mixer/pkg/adapter"
	pb "istio.io/mixer/pkg/config/proto"
	"istio.io/mixer/pkg/expr"
	"istio.io/mixer/pkg/status"
)

type validateFunc func(cfg map[string]string) (rt *Validated, ce *adapter.ConfigErrors)

type readBodyFunc func(r io.Reader) ([]byte, error)

// API is the server wrapper that listens for incoming requests to the manager and processes them
type API struct {
	version  string
	rootPath string

	// used at the back end for validation and storage
	store    KVStore
	validate validateFunc

	// house keeping
	handler http.Handler
	Server  *http.Server

	// fault injection
	readBody readBodyFunc
}

// DefaultAPIPort default port exposed by the API server.
const DefaultAPIPort = 9094

// register routes
func (a *API) register(c *restful.Container) {
	ws := &restful.WebService{}
	ws.Consumes(restful.MIME_JSON, "application/yaml", "application/x-yaml")
	ws.Produces(restful.MIME_JSON)
	ws.Path(a.rootPath)

	ws.Route(ws.
		GET("/scopes/{scope}/subjects/{subject}/rules").
		To(a.getRules).
		Doc("Gets rules associated with the given scope and subject").
		Param(ws.PathParameter("scope", "scope").DataType("string")).
		Param(ws.PathParameter("subject", "subject").DataType("string")).
		Writes(pb.ServiceConfig{}))

	ws.Route(ws.
		PUT("/scopes/{scope}/subjects/{subject}/rules").
		To(a.putRules).
		Doc("Replaces rules associated with the given scope and subject").
		Param(ws.PathParameter("scope", "scope").DataType("string")).
		Param(ws.PathParameter("subject", "subject").DataType("string")).
		Writes(rpc.Status{}))
	c.Add(ws)
}

// NewAPI creates a new API server
func NewAPI(version string, port int, eval expr.Validator, aspectFinder AspectValidatorFinder,
	builderFinder BuilderValidatorFinder, findAspects AdapterToAspectMapper, store KVStore) *API {
	c := restful.NewContainer()
	a := &API{
		version:  version,
		rootPath: fmt.Sprintf("/api/%s", version),
		store:    store,
		readBody: ioutil.ReadAll,
		validate: func(cfg map[string]string) (rt *Validated, ce *adapter.ConfigErrors) {
			v := newValidator(aspectFinder, builderFinder, findAspects, true, eval)
			return v.validate(cfg)
		},
	}
	a.register(c)
	a.Server = &http.Server{Addr: ":" + strconv.Itoa(port), Handler: c}
	a.handler = c
	return a
}

// Run calls listen and serve on the API server
func (a *API) Run() {
	glog.Infof("Starting Config API Server at %v", a.Server.Addr)
	glog.Warning(a.Server.ListenAndServe())
}

// getRules returns the entire service config document for the scope and subject
// "/scopes/{scope}/subjects/{subject}/rules"
func (a *API) getRules(req *restful.Request, resp *restful.Response) {
	funcPath := req.Request.URL.Path[len(a.rootPath):]
	val, index, found := a.store.Get(funcPath)
	if !found {
		writeResponse(http.StatusNotFound, fmt.Sprintf("no rules for %s\n", funcPath), resp)
		return
	}

	// TODO send index back to the client
	_ = index
	resp.AddHeader("Content-Type", "application/yaml")
	write(val, resp)
}

// putRules replaces the entire service config document for the scope and subject
// "/scopes/{scope}/subjects/{subject}/rules"
func (a *API) putRules(req *restful.Request, resp *restful.Response) {
	funcPath := req.Request.URL.Path[len(a.rootPath):]

	// TODO optimize only read descriptors and adapters
	data, _, index, err := readdb(a.store, "/")
	if err != nil {
		writeResponse(http.StatusInternalServerError, err.Error(), resp)
		return
	}
	// TODO send index back to the client
	_ = index

	var bval []byte
	bval, err = a.readBody(req.Request.Body)
	if err != nil {
		writeResponse(http.StatusInternalServerError, err.Error(), resp)
		return
	}
	val := string(bval)
	data[funcPath] = val

	_, cerr := a.validate(data)
	if cerr != nil {
		glog.Warning(cerr.Error())
		glog.Warning(val)
		writeResponse(http.StatusPreconditionFailed, cerr.Error(), resp)
		return
	}
	index, err = a.store.Set(funcPath, val)
	if err != nil {
		writeResponse(http.StatusInternalServerError, err.Error(), resp)
		return
	}
	// TODO send index back to the client
	_ = index

	writeResponse(http.StatusOK, fmt.Sprintf("Created %s", funcPath), resp)
}

// a subset of restful.Response
type response interface {
	// WriteHeaderAndJson is a convenience method for writing the status and a value in Json with a given Content-Type.
	WriteHeaderAndJson(status int, value interface{}, contentType string) error
}

func write(contents string, resp io.Writer) {
	_, err := resp.Write([]byte(contents))
	if err != nil {
		glog.Warning(err)
	}
}

func writeResponse(httpStatus int, msg string, resp response) {
	if err := resp.WriteHeaderAndJson(
		httpStatus,
		status.WithMessage(
			httpStatusToRPC(httpStatus), msg),
		restful.MIME_JSON,
	); err != nil {
		glog.Warning(err)
	}
}

func httpStatusToRPC(httpStatus int) rpc.Code {
	code, ok := httpStatusToRPCMap[httpStatus]
	if !ok {
		code = rpc.UNKNOWN
	}
	return code
}

// httpStatusToRpc limited mapping from proto documentation.
var httpStatusToRPCMap = map[int]rpc.Code{
	http.StatusOK:                 rpc.OK,
	http.StatusNotFound:           rpc.NOT_FOUND,
	http.StatusConflict:           rpc.ALREADY_EXISTS,
	http.StatusForbidden:          rpc.PERMISSION_DENIED,
	http.StatusUnauthorized:       rpc.UNAUTHENTICATED,
	http.StatusPreconditionFailed: rpc.FAILED_PRECONDITION,
}
