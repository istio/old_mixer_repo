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

package test

import (
	"time"

	rpc "github.com/googleapis/googleapis/google/rpc"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/status"
)

type AttributesHandler interface {
	Check(attribute.Bag, *attribute.MutableBag) rpc.Status
	Quota(attribute.Bag, QuotaArgs) (QuotaResponse, rpc.Status)
	Report(attribute.Bag) rpc.Status
}

// ChannelsHandler sends received attribute.Bags (and other) information on the
// configured channels (with a timeout).
type ChannelsHandler struct {
	AttributesHandler

	CheckResponseBag *attribute.MutableBag
	QuotaResponse    QuotaResponse
	ReturnStatus     rpc.Status

	// CheckAttributes is the channel on which the attribute bag generated by
	// the server for Check() requests is sent.
	CheckAttributes chan attribute.Bag

	// QuotaDispatches is the channel on which the quota information generated by
	// the server for Check() requests is sent.
	QuotaDispatches chan QuotaDispatchInfo

	// ReportAttributes is the channel on which the attribute bag generated by
	// the server for Report() requests is sent.
	ReportAttributes chan attribute.Bag
}

func NewChannelsHandler() *ChannelsHandler {
	return &ChannelsHandler{
		CheckResponseBag: attribute.GetMutableBag(nil),
		QuotaResponse:    QuotaResponse{DefaultValidDuration, DefaultAmount},
		ReturnStatus:     status.OK,
		CheckAttributes:  make(chan attribute.Bag),
		QuotaDispatches:  make(chan QuotaDispatchInfo),
		ReportAttributes: make(chan attribute.Bag),
	}
}

func (c *ChannelsHandler) Check(req attribute.Bag, resp *attribute.MutableBag) rpc.Status {
	// avoid blocked go-routines in testing
	select {
	case c.CheckAttributes <- attribute.CopyBag(req):
		resp = c.CheckResponseBag
		return c.ReturnStatus
	case <-time.After(1 * time.Second):
		return status.WithDeadlineExceeded("timeout sending to Check channel")
	}
}

func (c *ChannelsHandler) Quota(req attribute.Bag, qma QuotaArgs) (QuotaResponse, rpc.Status) {
	// avoid blocked go-routines in testing
	select {
	case c.QuotaDispatches <- QuotaDispatchInfo{attribute.CopyBag(req), qma}:
		return c.QuotaResponse, c.ReturnStatus
	case <-time.After(1 * time.Second):
		return QuotaResponse{}, status.WithDeadlineExceeded("timeout sending to Quota channel")
	}
}

func (c *ChannelsHandler) Report(req attribute.Bag) rpc.Status {
	// avoid blocked go-routines in testing
	select {
	case c.ReportAttributes <- attribute.CopyBag(req):
		return c.ReturnStatus
	case <-time.After(1 * time.Second):
		return status.WithDeadlineExceeded("timeout sending to Report channel")
	}
}

// QuotaDispatchInfo contains both the attribute bag generated by the server
// for Quota dispatch, as well as the corresponding method arguments.
type QuotaDispatchInfo struct {
	Attributes attribute.Bag
	MethodArgs QuotaArgs
}

// QuotaArgs mirrors aspect.QuotaArgs. It allows tests to understand
// how their inputs map into quota calls within Mixer to validate the proper
// requests are being generated. It is NOT a contract for how a real Mixer would
// generate or dispatch Quota calls.
type QuotaArgs struct {
	// Unique ID for Quota operation.
	DeduplicationID string

	// The quota to allocate from.
	Quota string

	// The amount of quota to allocate.
	Amount int64

	// If true, allows a response to return less quota than requested. When
	// false, the exact requested amount is returned or 0 if not enough quota
	// was available.
	BestEffort bool
}

type QuotaResponse struct {
	// The amount of time until which the returned quota expires, this is 0 for non-expiring quotas.
	Expiration time.Duration

	// The total amount of quota returned, may be less than requested.
	Amount int64
}
