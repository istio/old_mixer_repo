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
	"errors"
	"strings"
	"testing"

	"github.com/gogo/protobuf/proto"

	cfg "istio.io/mixer/pkg/config/proto"
)

type fakeValidator struct {
	err error
}

func (v *fakeValidator) Validate(t ChangeType, key Key, spec proto.Message) error {
	return v.err
}

func TestValidate(t *testing.T) {
	for _, c := range []struct {
		title         string
		kinds         map[string]proto.Message
		externalError error
		typ           ChangeType
		key           Key
		in            map[string]interface{}
		want          error
	}{
		{
			"update",
			map[string]proto.Message{
				"Handler": &cfg.Handler{},
			},
			nil,
			Update,
			Key{Kind: "Handler", Namespace: "ns", Name: "foo"},
			map[string]interface{}{"adapter": "noop", "name": "default"},
			nil,
		},
		{
			"delete",
			map[string]proto.Message{
				"Handler": &cfg.Handler{},
			},
			nil,
			Delete,
			Key{Kind: "Handler", Namespace: "ns", Name: "foo"},
			nil,
			nil,
		},
		{
			"unknown kinds",
			map[string]proto.Message{},
			errors.New("fail"),
			Update,
			Key{Kind: "Unknown", Namespace: "ns", Name: "foo"},
			map[string]interface{}{"foo": "bar"},
			nil,
		},
		{
			"external validator failures",
			map[string]proto.Message{"Handler": &cfg.Handler{}},
			errors.New("external validator failure"),
			Update,
			Key{Kind: "Handler", Namespace: "ns", Name: "foo"},
			map[string]interface{}{"adapter": "noop", "name": "default"},
			errors.New("external validator failure"),
		},
		{
			"external validator failures on delete",
			map[string]proto.Message{"Handler": &cfg.Handler{}},
			errors.New("external validator failure"),
			Delete,
			Key{Kind: "Handler", Namespace: "ns", Name: "foo"},
			nil,
			errors.New("external validator failure"),
		},
	} {
		t.Run(c.title, func(tt *testing.T) {
			v := &validator{
				externalValidator: &fakeValidator{c.externalError},
				kinds:             c.kinds,
			}
			err := v.Validate(c.typ, c.key, c.in)
			if c.want == nil && err != nil {
				tt.Errorf("Got %v, Want nil", err)
			} else if c.want != nil && !strings.Contains(err.Error(), c.want.Error()) {
				tt.Errorf("Got %v, Want to contain %s", err, c.want.Error())
			}
		})
	}
}
