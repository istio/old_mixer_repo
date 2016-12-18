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

package attr

import (
	mixerpb "istio.io/mixer/api/v1"
)

// AttributeTracker is responsible for tracking a set of live attributes over time.
//
// An instance of this type is created for every gRPC stream incoming to the
// mixer. The instance tracks a current dictionary along with a set of
// attribute contexts.
type AttributeTracker interface {
	// Update refreshes the set of attributes tracked based on an incoming proto.
	//
	// This returns the AttributeContext that can be used to query the current
	// set of attributes.
	//
	// If this returns a non-nil error, it indicates there was a problem in the
	// supplied Attributes struct. When this happens, the state of the
	// AttributeContext is left unchanged.
	Update(attrs *mixerpb.Attributes) (AttributeContext, error)
}

type attributeTracker struct {
	dictionaries *dictionaries

	// all active contexts
	contexts map[int32]*attributeContext

	// the current live dictionary
	dictionary dictionary
}

func newTracker(dictionaries *dictionaries) AttributeTracker {
	return &attributeTracker{dictionaries, make(map[int32]*attributeContext), nil}
}

func (at *attributeTracker) Update(attrs *mixerpb.Attributes) (AttributeContext, error) {
	// replace the dictionary if requested
	if len(attrs.Dictionary) > 0 {
		at.dictionary = at.dictionaries.Intern(attrs.Dictionary)
	}

	// find the context and reset it if needed
	ac := at.contexts[attrs.AttributeContext]
	if ac == nil {
		ac = newAttributeContext()
		at.contexts[attrs.AttributeContext] = ac
	} else if attrs.ResetContext {
		ac.reset()
	}

	err := ac.update(at.dictionary, attrs)
	return ac, err
}
