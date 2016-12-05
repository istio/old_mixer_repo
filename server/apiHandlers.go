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

package main

// This is what implements the per-request logic for each API method.

import (
	"github.com/golang/glog"

	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/genproto/googleapis/rpc/status"

	"istio.io/mixer/adapters"
	"istio.io/mixer/api/v1"
)

const (
	// ServiceName is a well-known name of a fact that is passed by the caller to specify
	// the particular config that should be used when handling a request.
	ServiceName = "serviceName"

	// PeerID is a well-known name of a fact that is passed by the caller to specify
	// the id of the client.
	PeerID = "peerId"
)

// APIHandlers holds pointers to the functions that implement
// request-level processing for incoming all public APIs.
type APIHandlers interface {
	// Check performs the configured set of precondition checks.
	// Note that the request parameter is immutable, while the response parameter is where
	// results are specified
	Check(adapters.FactTracker, *mixerpb.CheckRequest, *mixerpb.CheckResponse)

	// Report performs the requested set of reporting operations.
	// Note that the request parameter is immutable, while the response parameter is where
	// results are specified
	Report(adapters.FactTracker, *mixerpb.ReportRequest, *mixerpb.ReportResponse)

	// Quota increments, decrements, or queries the specified quotas.
	// Note that the request parameter is immutable, while the response parameter is where
	// results are specified
	Quota(adapters.FactTracker, *mixerpb.QuotaRequest, *mixerpb.QuotaResponse)
}

type apiHandlers struct {
	adapterManager *AdapterManager
	configManager  *ConfigManager
}

func newStatus(c code.Code) *status.Status {
	return &status.Status{Code: int32(c)}
}

func newQuotaError(c code.Code) *mixerpb.QuotaResponse_Error {
	return &mixerpb.QuotaResponse_Error{Error: newStatus(c)}
}

// NewAPIHandlers returns a canonical APIHandlers that implements all of the mixer's API surface
func NewAPIHandlers(adapterManager *AdapterManager, configManager *ConfigManager) APIHandlers {
	return &apiHandlers{
		adapterManager: adapterManager,
		configManager:  configManager,
	}
}

func (h *apiHandlers) Check(tracker adapters.FactTracker, request *mixerpb.CheckRequest, response *mixerpb.CheckResponse) {

	// Prepare common response fields.
	response.RequestIndex = request.RequestIndex

	tracker.UpdateFacts(request.GetFacts())

	serviceName := tracker.GetLabels()[ServiceName]
	peerID := tracker.GetLabels()[PeerID]

	allowed, err := h.checkLists(tracker, serviceName, peerID)
	if err != nil {
		glog.Warning("Unexpected check error: %v", err)
		response.Result = newStatus(code.Code_INTERNAL)
		return
	}

	if !allowed {
		response.Result = newStatus(code.Code_PERMISSION_DENIED)
		return
	}

	// No objections from any of the adapters
	response.Result = newStatus(code.Code_OK)
}

func (h *apiHandlers) checkLists(tracker adapters.FactTracker, serviceName string, peerID string) (bool, error) {
	listCheckerConfigs, err := h.configManager.GetListCheckerConfig(peerID, serviceName)
	if err != nil {
		return false, err
	}

	for _, listCheckerConfig := range listCheckerConfigs {
		listCheckerAdapter, err := h.adapterManager.GetListCheckerAdapter(peerID, serviceName, listCheckerConfig.AdapterConfig)
		if err != nil {
			return false, err
		}

		symbol, err := listCheckerConfig.EvaluateSymbolValue(tracker.GetLabels())
		if err != nil {
			return false, err
		}

		// TODO: This logic assumes that the result of CheckList will always have the same semantics
		// as a whitelist lookup. We need to clearly define the blacklist semantics and plumb it at some point.
		inList, err := listCheckerAdapter.CheckList(symbol)
		if err != nil {
			return false, err
		}

		if !inList {
			return false, nil
		}
	}

	return true, nil
}

func (h *apiHandlers) Report(tracker adapters.FactTracker, request *mixerpb.ReportRequest, response *mixerpb.ReportResponse) {
	tracker.UpdateFacts(request.GetFacts())
	response.RequestIndex = request.RequestIndex
	response.Result = newStatus(code.Code_UNIMPLEMENTED)
}

func (h *apiHandlers) Quota(tracker adapters.FactTracker, request *mixerpb.QuotaRequest, response *mixerpb.QuotaResponse) {
	tracker.UpdateFacts(request.GetFacts())
	response.RequestIndex = request.RequestIndex
	response.Result = newQuotaError(code.Code_UNIMPLEMENTED)
}
