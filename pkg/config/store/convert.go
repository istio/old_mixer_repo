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

package store

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
)

func convertWithKind(spec map[string]interface{}, kind string, kinds map[string]proto.Message) (proto.Message, error) {
	base, ok := kinds[kind]
	if !ok {
		return nil, fmt.Errorf("unrecognized kind %s", kind)
	}
	pbSpec := proto.Clone(base)
	if err := convert(spec, pbSpec); err != nil {
		return nil, err
	}
	return pbSpec, nil
}

func convert(spec map[string]interface{}, target proto.Message) error {
	jsonData, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	return jsonpb.Unmarshal(bytes.NewReader(jsonData), target)
}
