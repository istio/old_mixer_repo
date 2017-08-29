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
	"github.com/gogo/protobuf/proto"
	"github.com/golang/glog"
)

// validator provides the default structural validation with delegating
// an external validator for the referential integrity.
type validator struct {
	externalValidator Validator
	kinds             map[string]proto.Message
}

// NewValidator creates a default validator which validates the structure through registered
// kinds and referential integrity through ev.
func NewValidator(ev Validator, kinds map[string]proto.Message) BackendValidator {
	return &validator{
		externalValidator: ev,
		kinds:             kinds,
	}
}

func (v *validator) Validate(t ChangeType, key Key, spec map[string]interface{}) error {
	pbSpecTmpl, ok := v.kinds[key.Kind]
	if !ok {
		// Pass unrecognized kinds -- they should be validated by somewhere else.
		glog.V(3).Infof("unrecognized kind %s is requested to validate", key.Kind)
		return nil
	}
	var pbSpec proto.Message
	if t == Update {
		pbSpec = proto.Clone(pbSpecTmpl)
		if err := convert(spec, pbSpec); err != nil {
			return err
		}
	}
	if v.externalValidator == nil {
		return nil
	}
	return v.externalValidator.Validate(t, key, pbSpec)
}
