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

// This file contains an implementation of an opentracing carrier built on top of Mixer's attribute bag. This allows
// clients to inject their span metadata into the set of attributes its reporting to Mixer, and lets Mixer extract
// span information propagated as attributes in a standard call to Mixer.

package tracing

import (
	"context"
	"fmt"

	"github.com/golang/glog"

	"istio.io/mixer/pkg/attribute"
)

const (
	headerAttributeName = "request.headers"
)

// AttributeFormat is the format marker to use when Injecting/Extracting attribute based metadata.
type AttributeFormat struct{}

// AttributeCarrier implements opentracing's TextMapWriter and TextMapReader interfaces using Istio's attributes.
// Today we read only the HTTP headers (as attributes); in the future we can support full attribute based trace propagation.
type AttributeCarrier struct {
	req  attribute.Bag
	resp *attribute.MutableBag
	keys map[string]bool // the header names we'll look for.
}

// NewCarrier initializes a carrier that can be used to modify the attributes in the current request context.
func NewCarrier(req attribute.Bag, resp *attribute.MutableBag, headerNames []string) *AttributeCarrier {
	keys := make(map[string]bool, len(headerNames))
	for _, header := range headerNames {
		keys[header] = true
	}
	return &AttributeCarrier{req: req, resp: resp, keys: keys}
}

type carrierKey struct{}

// NewContext annotates the provided context with the provided carrier and returns the updated context.
func NewContext(ctx context.Context, carrier *AttributeCarrier) context.Context {
	return context.WithValue(ctx, carrierKey{}, carrier)
}

// FromContext extracts the AttributeCarrier from the provided context, or returns nil if one is not found.
func FromContext(ctx context.Context) *AttributeCarrier {
	mc := ctx.Value(carrierKey{})
	if c, ok := mc.(*AttributeCarrier); ok {
		return c
	}
	return nil
}

// ForeachKey iterates over the request's headers (tucked into the request.headers attribute) looking for the keys
// that the carrier was constructed with. This uses the headers from the _request_ bag.
func (c *AttributeCarrier) ForeachKey(handler func(key, val string) error) error {
	headers, err := getHeaders(c.req)
	if err != nil {
		return err
	}
	for header, val := range headers {
		if c.keys[header] {
			if err := handler(header, val); err != nil {
				return err
			}
		}
	}
	return nil
}

// Set adds (key, val) to the request.headers attribute of the _response_ bag.
func (c *AttributeCarrier) Set(key, val string) {
	headers, err := getHeaders(c.resp)
	if err != nil {
		glog.Warningf("Tried to set key in attribute trace metadata carrier, but no request.headers attribute found: %v", err)
		return
	}
	headers[key] = val
	c.resp.Set(headerAttributeName, headers)
}

func getHeaders(b attribute.Bag) (map[string]string, error) {
	// TODO: is this func really even necessary? request.headers is a canonical attribute, so should we be paranoid about it?
	attrVal, found := b.Get(headerAttributeName)
	if !found {
		return nil, fmt.Errorf("no header attribute named '%s'", headerAttributeName)
	}
	headers, ok := attrVal.(map[string]string)
	if !ok {
		return nil, fmt.Errorf("expected headers to be a map[string]string, got something else: %#v", attrVal)
	}
	return headers, nil
}
