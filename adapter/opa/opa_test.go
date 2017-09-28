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

func TestSinglePolicy(t *testing.T) {
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
			Action: map[string]descriptor.ValueType{
				"namespace": descriptor.STRING,
				"service":   descriptor.STRING,
				"path":      descriptor.STRING,
				"verb":      descriptor.STRING,
			},
		},
	})

	b.SetAdapterConfig(&config.Params{
		Policy: []string{`package mixerauthz
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
	      input.subject.user = rule.users[_]
	      input.action.verb = rule.verbs[_]
	    }`},
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
			Action: make(map[string]interface {
			}),
		}
		instance.Subject["user"] = c.user
		instance.Action["verb"] = c.verb

		result, err := authzHandler.HandleAuthz(context.Background(), &instance)
		if err != nil {
			t.Errorf("Got error %v, expecting success", err)
		}

		if result.Status.Code != int32(c.exptected) {
			t.Errorf("Got error %v, expecting success", err)
		}
	}
}

func TestMultiplePolicy(t *testing.T) {
	info := GetInfo()

	if !contains(info.SupportedTemplates, authz.TemplateName) {
		t.Error("Didn't find all expected supported templates")
	}

	b := info.NewBuilder().(*builder)

	b.SetAuthzTypes(map[string]*authz.Type{
		"authzInstance.authz.istio-config-default": {
			Subject: map[string]descriptor.ValueType{
				"user": descriptor.STRING,
			},
			Action: map[string]descriptor.ValueType{
				"source": descriptor.STRING,
				"target": descriptor.STRING,
				"path":   descriptor.STRING,
				"method": descriptor.STRING,
			},
		},
	})

	b.SetAdapterConfig(&config.Params{
		Policy: []string{
			`
			package example
			import data.service_graph
			import data.org_chart

			# Deny request by default.
			default allow = false

			# Allow request if...
			allow {
			    service_graph.allow  # service graph policy allows, and...
			    org_chart.allow      # org chart policy allows.
			}
		`, `
			package org_chart

			parsed_path = p {
			    trim(input.action.path, "/", trimmed)
			    split(trimmed, "/", p)
			}

			employees = {
			    "bob": {"manager": "janet", "roles": ["engineering"]},
			    "alice": {"manager": "janet", "roles": ["engineering"]},
			    "janet": {"roles": ["engineering"]},
			    "ken": {"roles": ["hr"]},
			}

			# Allow access to non-sensitive APIs.
			allow { not is_sensitive_api }

			is_sensitive_api {
			    parsed_path[0] = "reviews"
			}

			# Allow users access to sensitive APIs serving their own data.
			allow {
			    parsed_path = ["reviews", user]
			    input.subject.user = user
			}

			# Allow managers access to sensitive APIs serving their reports' data.
			allow {
			    parsed_path = ["reviews", user]
			    input.subject.user = employees[user].manager
			}

			# Allow HR to access all APIs.
			allow {
			    is_hr
			}

			is_hr {
			    input.subject.user = user
			    employees[user].roles[_] = "hr"
			}
		`, `
			package service_graph

			service_graph = {
			    "landing_page": ["details", "reviews"],
			    "reviews": ["ratings"],
			}

			default allow = false

			allow {
			    input.action.external = true
			    input.action.target = "landing_page"
			}

			allow {
			    allowed_targets = service_graph[input.action.source]
			    input.action.target = allowed_targets[_]
			}
		`},
		CheckMethod: "data.example.allow",
	})

	if err := b.Validate(); err != nil {
		t.Fatalf("Got error %v, expecting success", err)
	}

	handler, err := b.Build(context.Background(), test.NewEnv(t))
	if err != nil {
		t.Fatalf("Got error %v, expecting success", err)
	}

	cases := []struct {
		method    string
		path      string
		source    string
		target    string
		user      string
		exptected rpc.Code
	}{
		// manager
		{"GET", "/reviews/janet", "landing_page", "reviews", "janet", rpc.OK},
		{"GET", "/reviews/alice", "landing_page", "reviews", "janet", rpc.OK},
		{"GET", "/reviews/bob", "landing_page", "reviews", "janet", rpc.OK},
		{"GET", "/reviews/ken", "landing_page", "reviews", "janet", rpc.PERMISSION_DENIED},
		// self
		{"GET", "/reviews/alice", "landing_page", "reviews", "alice", rpc.OK},
		{"GET", "/reviews/janet", "landing_page", "reviews", "alice", rpc.PERMISSION_DENIED},
		{"GET", "/reviews/bob", "landing_page", "reviews", "alice", rpc.PERMISSION_DENIED},
		{"GET", "/reviews/ken", "landing_page", "reviews", "alice", rpc.PERMISSION_DENIED},
		// hr
		{"GET", "/reviews/janet", "landing_page", "reviews", "ken", rpc.OK},
		{"GET", "/reviews/alice", "landing_page", "reviews", "ken", rpc.OK},
		{"GET", "/reviews/bob", "landing_page", "reviews", "ken", rpc.OK},
		{"GET", "/reviews/ken", "landing_page", "reviews", "ken", rpc.OK},
		// service
		{"GET", "/reviews/janet", "landing_page", "review", "janet", rpc.PERMISSION_DENIED},
		{"GET", "/reviews/janet", "source_page", "reviews", "janet", rpc.PERMISSION_DENIED},
	}

	authzHandler := handler.(authz.Handler)

	for _, c := range cases {
		instance := authz.Instance{
			Subject: make(map[string]interface {
			}),
			Action: make(map[string]interface {
			}),
		}
		instance.Action["method"] = c.method
		instance.Action["path"] = c.path
		instance.Action["source"] = c.source
		instance.Action["target"] = c.target
		instance.Subject["user"] = c.user

		result, err := authzHandler.HandleAuthz(context.Background(), &instance)
		if err != nil {
			t.Errorf("Got error %v, expecting success", err)
		}

		if result.Status.Code != int32(c.exptected) {
			t.Errorf("Got status %v, expecting %v", result.Status.Code, c.exptected)
		}
	}
}

func TestFailOpenClose(t *testing.T) {
	cases := []struct {
		policy    string
		failClose bool
		exptected rpc.Code
	}{
		{"bad policy", false, rpc.OK},
		{"bad policy", true, rpc.PERMISSION_DENIED},
	}

	info := GetInfo()

	if !contains(info.SupportedTemplates, authz.TemplateName) {
		t.Error("Didn't find all expected supported templates")
	}

	for _, c := range cases {
		b := info.NewBuilder().(*builder)

		b.SetAuthzTypes(map[string]*authz.Type{
			"authzInstance.authz.istio-config-default": {
				Subject: map[string]descriptor.ValueType{
					"user":            descriptor.STRING,
					"serviceAccount":  descriptor.STRING,
					"sourceNamespace": descriptor.STRING,
				},
				Action: map[string]descriptor.ValueType{
					"namespace": descriptor.STRING,
					"service":   descriptor.STRING,
					"path":      descriptor.STRING,
				},
			},
		})

		// OPA policy with invalid syntax
		b.SetAdapterConfig(&config.Params{
			Policy:      []string{c.policy},
			CheckMethod: "data.mixerauthz.allow",
			FailClose:   c.failClose,
		})

		if err := b.Validate(); err != nil {
			t.Errorf("Got error %v, expecting success", err)
		}

		handler, err := b.Build(context.Background(), test.NewEnv(t))
		if err != nil {
			t.Fatalf("Got error %v, expecting success", err)
		}

		instance := authz.Instance{
			Subject: make(map[string]interface{}),
			Action:  make(map[string]interface{}),
		}
		authzHandler := handler.(authz.Handler)

		result, err := authzHandler.HandleAuthz(context.Background(), &instance)
		if err != nil {
			t.Errorf("Got error %v, expecting success", err)
		}

		if result.Status.Code != int32(c.exptected) {
			t.Errorf("Got error %v, expecting success", err)
		}
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
