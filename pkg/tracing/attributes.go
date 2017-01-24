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

// This file contains an implementation of an opentracing carrier built on top of the mixer's attribute bag. This allows
// clients to inject their span metadata into the set of attributes its reporting to the mixer, and lets the mixer extract
// span information propagated as attributes in a standard call to the mixer.

package tracing

import (
	"context"
	"strings"
	"sync"

	mixerpb "istio.io/api/mixer/v1"
)

const (
	prefix       = "istio-ot-"
	prefixLength = len(prefix)
)

// AttributeCarrier implements opentracing's TextMapWriter and TextMapReader interfaces using Istio's attribute system.
//
// TODO: this type is fundamentally flawed: when it operates on the set of attributes it must be threadsafe, but
// this type does not own the attribute set. This means the locks we use only provide protections against multiple
// threads using the carrier, not the underlying Attribute set we're pointing at. Either we need to guarantee this is
// only ever accessed by a single thread (unlikely given adapters should propagate span info and they're executed
// concurrently) or we need something that owns the attributes to take the lock (i.e. the attributes.Bag type should be
// extended so that this type can use it instead of using the raw attributes proto).
type AttributeCarrier struct {
	sync.RWMutex
	a *mixerpb.Attributes
}

// NewCarrier initializes a carrier that can be used to modify the attributes in the current request context.
func NewCarrier(attributes *mixerpb.Attributes) *AttributeCarrier {
	return &AttributeCarrier{a: attributes}
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

// ForeachKey iterates over the keys that correspond to string attributes, filtering keys for the prefix used by the
// AttributeCarrier to denote opentracing attributes. The AttributeCarrier specific prefix is stripped from the key
// before its passed to handler.
func (c *AttributeCarrier) ForeachKey(handler func(key, val string) error) error {
	// TODO: the only thing preventing us from implementing the carrier on top of attribute.Bag instead of attributes
	// directly is the lack of ability to iterate over keys in the bag for this method. If the bag interface was
	// updated to provide the set of keys it knows about, we'd have a substantially easier time.

	c.RLock()
	// Look through the dictionary for keys with our prefix, check that they refer to string attributes, then call handle
	for i, key := range c.a.Dictionary {
		if strings.HasPrefix(key, prefix) {
			if val, found := c.a.StringAttributes[i]; found {
				if err := handler(key[prefixLength:], val); err != nil {
					c.RUnlock()
					return err
				}
			}
		}
	}
	c.RUnlock()
	return nil
}

// Set adds (key, val) to the set of string attributes in the request scope. It prefixes the key with an istio-private
// string to avoid collisions with other attributes when arbitrary tracer implementations are used.
func (c *AttributeCarrier) Set(key, val string) {
	// We have to deal with the case that the key already exists in the attribute set, and the case that its new.
	// First we see if it exists in the attribute set already.
	keyIndex := int32(-1)
	prefixedKey := prefix + key
	c.RLock()
	for i, k := range c.a.Dictionary {
		if k == prefixedKey {
			keyIndex = i
			break
		}
	}
	c.RUnlock()

	// Key exists in the map already; update the value in the string attribute set.
	//
	// TODO: since we use a constant prefix private to this package, we assume that if the key is found it references
	// a string attribute (the AttributeCarrier has the invariant that all attributes it writes are stringly typed).
	// If we're being paranoid, we could verify that the key refers to a string attribute and take some other action
	// (panic, overwrite, etc) if it does not.
	if keyIndex > 0 {
		c.Lock()
		c.a.StringAttributes[keyIndex] = val
		c.Unlock()
		return
	}

	// Key does not exist in the map, add it
	c.Lock()
	index := largestKey(c.a.Dictionary) + 1
	c.a.Dictionary[index] = key
	c.a.StringAttributes[index] = val
	c.Unlock()
	return
}

// Note: the map should be read locked when this method is called. We don't take a lock in this method because
// the lock should also cover updates to the map using the key we found.
func largestKey(m map[int32]string) int32 {
	largest := int32(-1)
	for k := range m {
		if k > largest {
			largest = k
		}
	}
	return largest
}
