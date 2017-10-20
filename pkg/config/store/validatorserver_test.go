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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gogo/protobuf/proto"
	multierror "github.com/hashicorp/go-multierror"
	cpb "istio.io/mixer/pkg/config/proto"
)

type fakeValidator struct {
	err error
}

func (v *fakeValidator) Validate([]*Event) error {
	return v.err
}

type countValidator int

func (v countValidator) Validate(evs []*Event) error {
	if len(evs) != int(v) {
		return fmt.Errorf("want %d, got %d", v, len(evs))
	}
	return nil
}

const sampleResource = `{"kind": "rule", "apiVersion": "config.istio.io/v1alpha2", "metadata": {"name": "foo", "namespace": "bar"}, "spec": {"match": "foo"}}`

func makeRequest(op string, reqs ...string) string {
	return fmt.Sprintf(`{"operation": "%s", "resources": [%s]}`, op, strings.Join(reqs, ","))
}

func TestParseRequest(t *testing.T) {
	v := &ValidatorServer{}
	for _, c := range []struct {
		title string
		in    string
		ctype string
		want  *validateRequest
	}{
		{
			"base",
			makeRequest("update", sampleResource),
			"application/json",
			&validateRequest{
				Operation: Update,
				Resources: []*BackEndResource{
					{
						Kind:       "rule",
						APIVersion: "config.istio.io/v1alpha2",
						Metadata:   ResourceMeta{Name: "foo", Namespace: "bar"},
						Spec:       map[string]interface{}{"match": "foo"},
					},
				},
			},
		},
		{
			"delete",
			makeRequest("delete", sampleResource),
			"application/json",
			&validateRequest{
				Operation: Delete,
				Resources: []*BackEndResource{
					{
						Kind:       "rule",
						APIVersion: "config.istio.io/v1alpha2",
						Metadata:   ResourceMeta{Name: "foo", Namespace: "bar"},
						Spec:       map[string]interface{}{"match": "foo"},
					},
				},
			},
		},
		{
			"multi",
			makeRequest("update", sampleResource, sampleResource),
			"application/json",
			&validateRequest{
				Operation: Update,
				Resources: []*BackEndResource{
					{
						Kind:       "rule",
						APIVersion: "config.istio.io/v1alpha2",
						Metadata:   ResourceMeta{Name: "foo", Namespace: "bar"},
						Spec:       map[string]interface{}{"match": "foo"},
					},
					{
						Kind:       "rule",
						APIVersion: "config.istio.io/v1alpha2",
						Metadata:   ResourceMeta{Name: "foo", Namespace: "bar"},
						Spec:       map[string]interface{}{"match": "foo"},
					},
				},
			},
		},
		{
			"invalid",
			"}",
			"application/json",
			nil,
		},
		{
			"unexpected-content-type",
			makeRequest("update", sampleResource, sampleResource),
			"text/plain",
			nil,
		},
	} {
		t.Run(c.title, func(tt *testing.T) {
			var body io.Reader
			if len(c.in) != 0 {
				body = strings.NewReader(c.in)
			}
			r := httptest.NewRequest("POST", "/", body)
			r.Header.Add("Content-Type", c.ctype)
			got, err := v.parseRequest(r)
			wantSuccess := c.want != nil
			gotSuccess := err == nil
			if wantSuccess != gotSuccess {
				tt.Errorf("got %v(%v), want %v", gotSuccess, err, wantSuccess)
			}
			if c.want != nil && !reflect.DeepEqual(got, c.want) {
				tt.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestResponse(t *testing.T) {
	for _, c := range []struct {
		in   error
		want validateResponse
	}{
		{nil, validateResponse{Allowed: true}},
		{errors.New("dummy"), validateResponse{Allowed: false, Details: []string{"dummy"}}},
		{
			multierror.Append(nil, errors.New("dummy1"), errors.New("dummy2")),
			validateResponse{Allowed: false, Details: []string{"dummy1", "dummy2"}},
		},
	} {
		resp := errorToResponse(c.in)
		if !reflect.DeepEqual(resp, c.want) {
			t.Errorf("got %+v, want %+v", resp, c.want)
		}
	}
}

func TestValidate(t *testing.T) {
	for _, c := range []struct {
		title     string
		targetNS  []string
		validator Validator
		in        *validateRequest
		ok        bool
	}{
		{
			"success",
			nil,
			countValidator(1),
			&validateRequest{
				Operation: Update,
				Resources: []*BackEndResource{
					{
						Kind:     ruleKind,
						Metadata: ResourceMeta{Name: "foo", Namespace: "bar"},
						Spec:     map[string]interface{}{"match": "foo"},
					},
				},
			},
			true,
		},
		{
			"multi",
			nil,
			countValidator(2),
			&validateRequest{
				Operation: Update,
				Resources: []*BackEndResource{
					{
						Kind:     ruleKind,
						Metadata: ResourceMeta{Name: "foo", Namespace: "bar"},
						Spec:     map[string]interface{}{"match": "foo"},
					},
					{
						Kind:     ruleKind,
						Metadata: ResourceMeta{Name: "foo", Namespace: "bazz"},
						Spec:     map[string]interface{}{"match": "foo"},
					},
				},
			},
			true,
		},
		{
			"conflict",
			nil,
			nil,
			&validateRequest{
				Operation: Update,
				Resources: []*BackEndResource{
					{
						Kind:     ruleKind,
						Metadata: ResourceMeta{Name: "foo", Namespace: "bar"},
						Spec:     map[string]interface{}{"match": "foo"},
					},
					{
						Kind:     ruleKind,
						Metadata: ResourceMeta{Name: "foo", Namespace: "bar"},
						Spec:     map[string]interface{}{"match": "bar"},
					},
				},
			},
			false,
		},
		{
			"multiple-deletes",
			nil,
			countValidator(1),
			&validateRequest{
				Operation: Delete,
				Resources: []*BackEndResource{
					{
						Kind:     ruleKind,
						Metadata: ResourceMeta{Name: "foo", Namespace: "bar"},
						Spec:     map[string]interface{}{"match": "foo"},
					},
					{
						Kind:     ruleKind,
						Metadata: ResourceMeta{Name: "foo", Namespace: "bar"},
						Spec:     map[string]interface{}{"match": "bar"},
					},
				},
			},
			true,
		},
		{
			"fail-on-update",
			nil,
			&fakeValidator{errors.New("dummy")},
			&validateRequest{
				Operation: Update,
				Resources: []*BackEndResource{
					{
						Kind:     ruleKind,
						Metadata: ResourceMeta{Name: "foo", Namespace: "bar"},
						Spec:     map[string]interface{}{"match": "foo"},
					},
				},
			},
			false,
		},
		{
			"fail-on-delete",
			nil,
			&fakeValidator{errors.New("dummy")},
			&validateRequest{
				Operation: Delete,
				Resources: []*BackEndResource{
					{
						Kind:     ruleKind,
						Metadata: ResourceMeta{Name: "foo", Namespace: "bar"},
						Spec:     nil,
					},
				},
			},
			false,
		},
		{
			"unrelated-ns",
			[]string{"not-bar"},
			&fakeValidator{errors.New("dummy")},
			&validateRequest{
				Operation: Update,
				Resources: []*BackEndResource{
					{
						Kind:     ruleKind,
						Metadata: ResourceMeta{Name: "foo", Namespace: "bar"},
						Spec:     map[string]interface{}{"match": "foo"},
					},
				},
			},
			true,
		},
		{
			"partially-unrelated",
			[]string{"bar"},
			countValidator(1),
			&validateRequest{
				Operation: Update,
				Resources: []*BackEndResource{
					{
						Kind:     ruleKind,
						Metadata: ResourceMeta{Name: "foo", Namespace: "bar"},
						Spec:     map[string]interface{}{"match": "foo"},
					},
					{
						Kind:     ruleKind,
						Metadata: ResourceMeta{Name: "foo", Namespace: "bazz"},
						Spec:     map[string]interface{}{"match": "foo"},
					},
				},
			},
			true,
		},
		{
			"unknown-kinds",
			nil,
			&fakeValidator{errors.New("dummy")},
			&validateRequest{
				Operation: Update,
				Resources: []*BackEndResource{
					{
						Kind:     "unknown",
						Metadata: ResourceMeta{Name: "foo", Namespace: "bar"},
						Spec:     map[string]interface{}{"match": "foo"},
					},
				},
			},
			true,
		},
		{
			"unknown-kinds",
			nil,
			countValidator(1),
			&validateRequest{
				Operation: Update,
				Resources: []*BackEndResource{
					{
						Kind:     "unknown",
						Metadata: ResourceMeta{Name: "foo", Namespace: "bar"},
						Spec:     map[string]interface{}{"match": "foo"},
					},
					{
						Kind:     ruleKind,
						Metadata: ResourceMeta{Name: "foo", Namespace: "bar"},
						Spec:     map[string]interface{}{"match": "foo"},
					},
				},
			},
			true,
		},
		{
			"deprecated field",
			nil,
			&fakeValidator{},
			&validateRequest{
				Operation: Update,
				Resources: []*BackEndResource{
					{
						Kind:     ruleKind,
						Metadata: ResourceMeta{Name: "foo", Namespace: "bar"},
						Spec:     map[string]interface{}{"selector": "foo"},
					},
				},
			},
			false,
		},
		{
			"deprecated field with new ones",
			nil,
			&fakeValidator{},
			&validateRequest{
				Operation: Update,
				Resources: []*BackEndResource{
					{
						Kind:     ruleKind,
						Metadata: ResourceMeta{Name: "foo", Namespace: "bar"},
						Spec:     map[string]interface{}{"selector": "foo", "match": "foo"},
					},
				},
			},
			true,
		},
	} {
		t.Run(c.title, func(tt *testing.T) {
			v := NewValidatorServer(c.targetNS, map[string]proto.Message{ruleKind: &cpb.Rule{}}, c.validator)
			err := v.validate(c.in)
			gotSuccess := err == nil
			if gotSuccess != c.ok {
				tt.Errorf("got %v, want %v", err, c.ok)
			}
		})
	}
}

func TestServeHTTP(t *testing.T) {
	for _, c := range []struct {
		title   string
		body    string
		allowed bool
	}{
		{
			"base",
			makeRequest("update", sampleResource),
			true,
		},
		{
			"empty",
			"",
			true,
		},
	} {
		t.Run(c.title, func(tt *testing.T) {
			v := NewValidatorServer(nil, map[string]proto.Message{"rule": &cpb.Rule{}}, nil)
			req := httptest.NewRequest("POST", "/", strings.NewReader(c.body))
			req.Header.Add("Content-Type", "application/json")
			fmt.Printf("%+v\n", req)
			recorder := httptest.NewRecorder()
			v.ServeHTTP(recorder, req)
			resp := &validateResponse{}
			if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
				tt.Error(err)
			}
			if resp.Allowed != c.allowed {
				tt.Errorf("Got %+v, want %v", resp, c.allowed)
			}
		})
	}
}
