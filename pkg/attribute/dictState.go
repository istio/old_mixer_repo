// Copyright 2016 Istio Authors
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

type dictState struct {
	globalDict  map[string]int32
	messageDict map[string]int32
}

func newDictState(globalDict map[string]int32) *dictState {
	return &dictState{
		globalDict: globalDict,
	}
}

func (ds *dictState) getIndex(word string) int32 {
	if index, ok := ds.globalDict[word]; ok {
		return index
	}

	if ds.messageDict == nil {
		ds.messageDict = make(map[string]int32)
	} else if index, ok := ds.messageDict[word]; ok {
		return index
	}

	index := -int32(len(ds.messageDict)) - 1
	ds.messageDict[word] = index
	return index
}

func (ds *dictState) getMessageWordList() []string {
	if len(ds.messageDict) == 0 {
		return nil
	}

	words := make([]string, len(ds.messageDict))
	for k, v := range ds.messageDict {
		words[-v-1] = k
	}

	return words
}
