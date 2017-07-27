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

package attribute

import (
	mixerpb "istio.io/api/mixer/v1"
)

// ProtoBag implements the Bag interface on top of an Attributes proto.
type ProtoBag struct {
	proto          *mixerpb.Attributes
	globalDict     map[string]int32
	globalWordList []string
	messageDict    map[string]int32
}

func NewProtoBag(proto *mixerpb.Attributes, globalDict map[string]int32, globalWordList []string) *ProtoBag {
	return &ProtoBag{
		proto:          proto,
		globalDict:     globalDict,
		globalWordList: globalWordList,
	}
}

// Get returns an attribute value.
func (pb *ProtoBag) Get(name string) (interface{}, bool) {
	index, ok := pb.globalDict[name]
	if !ok {
		md := pb.getMessageDict()

		index, ok = md[name]
		if !ok {
			return nil, false
		}
	}

	var value interface{}

	value, ok = pb.proto.Strings[index]
	if ok {
		return value, true
	}

	value, ok = pb.proto.Int64S[index]
	if ok {
		return value, true
	}

	value, ok = pb.proto.Doubles[index]
	if ok {
		return value, true
	}

	value, ok = pb.proto.Bools[index]
	if ok {
		return value, true
	}

	value, ok = pb.proto.Timestamps[index]
	if ok {
		return value, true
	}

	value, ok = pb.proto.Durations[index]
	if ok {
		return value, true
	}

	value, ok = pb.proto.Bytes[index]
	if ok {
		return value, true
	}

	value, ok = pb.proto.StringMaps[index]
	if ok {
		return value, true
	}

	return nil, false
}

// Names return the names of all the attributes known to this bag.
func (pb *ProtoBag) Names() []string {
	var names []string
	for k := range pb.proto.Strings {
		name, _ := lookup(k, nil, pb.globalWordList, pb.proto.Words)
		names = append(names, name)
	}

	for k := range pb.proto.Int64S {
		name, _ := lookup(k, nil, pb.globalWordList, pb.proto.Words)
		names = append(names, name)
	}

	for k := range pb.proto.Doubles {
		name, _ := lookup(k, nil, pb.globalWordList, pb.proto.Words)
		names = append(names, name)
	}

	for k := range pb.proto.Bools {
		name, _ := lookup(k, nil, pb.globalWordList, pb.proto.Words)
		names = append(names, name)
	}

	for k := range pb.proto.Timestamps {
		name, _ := lookup(k, nil, pb.globalWordList, pb.proto.Words)
		names = append(names, name)
	}

	for k := range pb.proto.Durations {
		name, _ := lookup(k, nil, pb.globalWordList, pb.proto.Words)
		names = append(names, name)
	}

	for k := range pb.proto.Bytes {
		name, _ := lookup(k, nil, pb.globalWordList, pb.proto.Words)
		names = append(names, name)
	}

	for k := range pb.proto.StringMaps {
		name, _ := lookup(k, nil, pb.globalWordList, pb.proto.Words)
		names = append(names, name)
	}

	return names
}

func (pb *ProtoBag) Done() {
	// NOP
}

// Lazily produce the message-level dictionary
func (pb *ProtoBag) getMessageDict() map[string]int32 {
	if pb.messageDict == nil {
		// build the message-level dictionary

		d := make(map[string]int32, len(pb.proto.Words))
		for i, name := range pb.proto.Words {
			d[name] = -int32(i) - 1
		}

		// potentially racy update, but that's fine since all generate maps are equivalent
		pb.messageDict = d
	}

	return pb.messageDict
}
