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

func (v *fakeValidator) Validate([]*Event) error {
	return v.err
}

func backendEvent(t ChangeType, kind, namespace, name string, spec map[string]interface{}) *BackendEvent {
	return &BackendEvent{
		Type: t,
		Key:  Key{Kind: kind, Namespace: namespace, Name: name},
		Value: &BackEndResource{
			Metadata: ResourceMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: spec,
		},
	}
}

func TestValidate(t *testing.T) {
	for _, c := range []struct {
		title         string
		kinds         map[string]proto.Message
		externalError error
		evs           []*BackendEvent
		want          error
	}{
		{
			"update",
			map[string]proto.Message{
				"Handler": &cfg.Handler{},
			},
			nil,
			[]*BackendEvent{backendEvent(Update, "Handler", "ns", "foo", map[string]interface{}{"adapter": "noop", "name": "default"})},
			nil,
		},
		{
			"delete",
			map[string]proto.Message{
				"Handler": &cfg.Handler{},
			},
			nil,
			[]*BackendEvent{backendEvent(Delete, "Handler", "ns", "foo", nil)},
			nil,
		},
		{
			"unknown kinds",
			map[string]proto.Message{},
			errors.New("fail"),
			[]*BackendEvent{backendEvent(Update, "Unknown", "ns", "foo", map[string]interface{}{"foo": "bar"})},
			nil,
		},
		{
			"parrtially unknown",
			map[string]proto.Message{ruleKind: &cfg.Rule{}},
			errors.New("fail on known kind"),
			[]*BackendEvent{
				backendEvent(Update, "Unknown", "ns", "foo", map[string]interface{}{"foo": "bar"}),
				backendEvent(Update, ruleKind, "ns", "bar", map[string]interface{}{"match": "match rule"}),
			},
			errors.New("fail on known kind"),
		},
		{
			"external validator failures",
			map[string]proto.Message{"Handler": &cfg.Handler{}},
			errors.New("external validator failure"),
			[]*BackendEvent{backendEvent(Update, "Handler", "ns", "foo", map[string]interface{}{"adapter": "noop", "name": "default"})},
			errors.New("external validator failure"),
		},
		{
			"external validator failures on delete",
			map[string]proto.Message{"Handler": &cfg.Handler{}},
			errors.New("external validator failure"),
			[]*BackendEvent{backendEvent(Delete, "Handler", "ns", "foo", nil)},
			errors.New("external validator failure"),
		},
		{
			"deprecated field",
			map[string]proto.Message{ruleKind: &cfg.Rule{}},
			nil,
			[]*BackendEvent{backendEvent(Update, ruleKind, "ns", "foo", map[string]interface{}{"selector": "selector"})},
			errors.New("field 'selector' is deprecated"),
		},
	} {
		t.Run(c.title, func(tt *testing.T) {
			v := NewValidator(&fakeValidator{c.externalError}, c.kinds)
			err := v.Validate(c.evs)
			if c.want == nil && err != nil {
				tt.Errorf("Got %v, Want nil", err)
			} else if c.want != nil && !strings.Contains(err.Error(), c.want.Error()) {
				tt.Errorf("Got %v, Want to contain %s", err, c.want.Error())
			}
		})
	}
}
