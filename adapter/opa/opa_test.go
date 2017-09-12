// Copyright 2017 Istio Authors.
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

package opa

// NOTE: This test will eventually be auto-generated so that it automatically supports all CHECK and QUOTA
//       templates known to Mixer. For now, it's manually curated.

import (
	"context"
	"testing"

	rpc "github.com/googleapis/googleapis/google/rpc"

	descriptor "istio.io/api/mixer/v1/config/descriptor"
	"istio.io/mixer/adapter/opa/config"
	"istio.io/mixer/pkg/adapter/test"
	"istio.io/mixer/template/authz"
)

func TestAuthz(t *testing.T) {
	info := GetInfo()

	if !contains(info.SupportedTemplates, authz.TemplateName) {
		t.Error("Didn't find all expected supported templates")
	}

	b := info.NewBuilder().(*builder)

	b.SetAuthzTypes(map[string]*authz.Type{
		"authzInstance.authz.istio-config-default": {
			Subject: map[string]descriptor.ValueType{
				"user":            descriptor.STRING,
				"serviceAccount":  descriptor.STRING,
				"sourceNamespace": descriptor.STRING,
			},
			Resource: map[string]descriptor.ValueType{
				"namespace": descriptor.STRING,
				"service":   descriptor.STRING,
				"path":      descriptor.STRING,
			},
			Verb: map[string]descriptor.ValueType{
				"verb": descriptor.STRING,
			},
		},
	})

	b.SetAdapterConfig(&config.Params{
		Policy: `package mixerauthz
	    policy = [
	      {
	        "rule": {
	          "verbs": [
	            "storage.buckets.get"
	          ],
	          "users": [
	            "bucket-admins"
	          ]
	        }
	      }
	    ]
	
	    default allow = false
	
	    allow = true {
	      rule = policy[_].rule
	      input.user = rule.users[_]
	      input.verb = rule.verbs[_]
	    }`,
		CheckMethod: "data.mixerauthz.allow",
	})

	if err := b.Validate(); err != nil {
		t.Fatalf("Got error %v, expecting success", err)
	}

	handler, err := b.Build(context.Background(), test.NewEnv(t))
	if err != nil {
		t.Fatalf("Got error %v, expecting success", err)
	}

	cases := []struct {
		user      string
		verb      string
		exptected rpc.Code
	}{
		{"bucket-admins", "storage.buckets.get", rpc.OK},
		{"bucket-admins", "storage.buckets.put", rpc.PERMISSION_DENIED},
		{"bucket-users", "storage.buckets.get", rpc.PERMISSION_DENIED},
	}

	authzHandler := handler.(authz.Handler)

	for _, c := range cases {
		instance := authz.Instance{
			Subject: make(map[string]interface {
			}),
			Resource: make(map[string]interface {
			}),
			Verb: make(map[string]interface {
			}),
		}
		instance.Subject["user"] = c.user
		instance.Verb["verb"] = c.verb

		result, err := authzHandler.HandleAuthz(context.Background(), &instance)
		if err != nil {
			t.Errorf("Got error %v, expecting success", err)
		}

		if result.Status.Code != int32(c.exptected) {
			t.Errorf("Got error %v, expecting success", err)
		}
	}
}

func TestInvalidAuthzTypes(t *testing.T) {
	info := GetInfo()

	if !contains(info.SupportedTemplates, authz.TemplateName) {
		t.Error("Didn't find all expected supported templates")
	}

	b := info.NewBuilder().(*builder)

	b.SetAuthzTypes(map[string]*authz.Type{
		"authzInstance.authz.istio-config-default": {
			Subject: map[string]descriptor.ValueType{
				"user":            descriptor.STRING,
				"serviceAccount":  descriptor.STRING,
				"sourceNamespace": descriptor.STRING,
			},
			Resource: map[string]descriptor.ValueType{
				"user":      descriptor.STRING,
				"namespace": descriptor.STRING,
				"service":   descriptor.STRING,
				"path":      descriptor.STRING,
			},
			Verb: map[string]descriptor.ValueType{
				"verb": descriptor.STRING,
			},
		},
	})

	b.SetAdapterConfig(&config.Params{
		Policy: `package mixerauthz
	    policy = [
	      {
	        "rule": {
	          "verbs": [
	            "storage.buckets.get"
	          ],
	          "users": [
	            "bucket-admins"
	          ]
	        }
	      }
	    ]

	    default allow = false

	    allow = true {
	      rule = policy[_].rule
	      input.user = rule.users[_]
	      input.verb = rule.verbs[_]
	    }`,
		CheckMethod: "data.mixerauthz.allow",
	})

	if err := b.Validate(); err == nil {
		t.Fatalf("Expected error, opa: user in resource is duplicated")
	}
}

func TestInvalidOPAPolicy(t *testing.T) {
	info := GetInfo()

	if !contains(info.SupportedTemplates, authz.TemplateName) {
		t.Error("Didn't find all expected supported templates")
	}

	b := info.NewBuilder().(*builder)

	b.SetAuthzTypes(map[string]*authz.Type{
		"authzInstance.authz.istio-config-default": {
			Subject: map[string]descriptor.ValueType{
				"user":            descriptor.STRING,
				"serviceAccount":  descriptor.STRING,
				"sourceNamespace": descriptor.STRING,
			},
			Resource: map[string]descriptor.ValueType{
				"namespace": descriptor.STRING,
				"service":   descriptor.STRING,
				"path":      descriptor.STRING,
			},
			Verb: map[string]descriptor.ValueType{
				"verb": descriptor.STRING,
			},
		},
	})

	b.SetAdapterConfig(&config.Params{
		Policy: `package mixerauthz
	    policy = [
	      }
	    ]

	    default allow = false

	    allow = true {
	    }`,
		CheckMethod: "data.mixerauthz.allow",
	})

	if err := b.Validate(); err == nil {
		t.Fatalf("Expected error, opa: Failed to parse the OPA policy: 1 error occurred: 2:14: rego_parse_error: no match found, unexpected '='")
	}
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
