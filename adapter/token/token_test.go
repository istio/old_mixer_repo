package token

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

import (
	"testing"
	tokenConfig "istio.io/mixer/adapter/token/config"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/adapterManager"
	"istio.io/mixer/pkg/config"
	"istio.io/mixer/pkg/adapter/test"

)

func TestRegisteredForAttributes(t *testing.T) {
	builders := adapterManager.BuilderMap([]adapter.RegisterFn{Register})

	k := config.AttributesKind
	found := false
	for _, token := range builders {
		if token.Kinds.IsSet(k) {
			found = true
		}
		if !found {
			t.Errorf("The token adapter is not registered for kind %s", k)
		}
	}
}

func TestBasicLifecycle(t *testing.T) {
	b := newBuilder()
	env := test.NewEnv(t)
	tag, err := b.BuildAttributesGenerator(env, b.DefaultConfig())
	if err != nil {
		t.Errorf("Unable to create aspect: %v", err)
	}
	if err := tag.Close(); err != nil {
		t.Errorf("Unable to close aspect: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Errorf("Unable to close builder: %v", err)
	}
}

func TestDefaultBuilderConf(t *testing.T) {
	//default conf = empty conf
	b := newBuilder()

	if b.Name() == "" {
		t.Error("Name() => all builders need names")
	}

	if b.Description() == "" {
		t.Errorf("Description() => builder '%s' doesn't provide a valid description", b.Name())
	}

	c := b.DefaultConfig()
	if err := b.ValidateConfig(c); err != nil {
		t.Errorf("ValidateConfig() => builder '%s' can't validate its default configuration: %v", b.Name(), err)
	}

	if err := b.Close(); err != nil {
		t.Errorf("Close() => builder '%s' fails to close when used with its default configuration: %v", b.Name(), err)
	}
}

func TestConfig(t *testing.T) {
	testConfigs := []struct {
		name     string
		conf     *tokenConfig.Params
		errCount int
	}{
		{
			"empty config (default)",
			&tokenConfig.Params{},
			0, //default empty config is valid - considered as having no issuers
		},
		{
			"empty issuer array config",
			&tokenConfig.Params{Issuers: make([]*tokenConfig.Issuer, 0)},
			0, //config has an empty issuer array
		},
		{
			"single empty issuer",
			&tokenConfig.Params{Issuers: make([]*tokenConfig.Issuer, 1)},
			1, //nil issuer
		},
		{
			"invalid name",
			&tokenConfig.Params{
				Issuers: []*tokenConfig.Issuer{
					{Name: "invalidchars>", PubKeyUrl: "https://pubkeys.org:5111"},
				},
			},
			1, //invalid name character >
		},
		{
			"invalid url",
			&tokenConfig.Params{
				Issuers: []*tokenConfig.Issuer{
					{Name: "validchars", PubKeyUrl: "https://pubkeys..org::5111"},
				},
			},
			1, //invalid url
		},
		{
			"invalid name,url",
			&tokenConfig.Params{
				Issuers: []*tokenConfig.Issuer{
					{Name: "3badstartingcharacter", PubKeyUrl: "www.pubkeys..org"},
				},
			},
			2, //invalid url + name
		},
		{
			"valid name,url",
			&tokenConfig.Params{
				Issuers: []*tokenConfig.Issuer{
					{Name: "w3", PubKeyUrl: "W3.ibm.com/pubkeys:7670"},
				},
			},
			0, //
		},
		{
			"issuer with multiple mappings to the same claim name",
			&tokenConfig.Params{
				Issuers: []*tokenConfig.Issuer{
					{Name: "w3", PubKeyUrl: "W3.ibm.com/pubkeys:7670", ClaimNames: []string{"dup", "dup", "sub"}},
				},
			},
			1, //duplicate claim name
		},
		{
			"issuer with empty claim names",
			&tokenConfig.Params{
				Issuers: []*tokenConfig.Issuer{
					{Name: "w3", PubKeyUrl: "W3.ibm.com/pubkeys:7670", ClaimNames: []string{}},
				},
			},
			0, //duplicate claim name
		},
	}

	b := newBuilder()
	for _, v := range testConfigs {
		err := b.ValidateConfig(v.conf)
		if err != nil && v.errCount == 0 {
			t.Fatalf("Expected config: %v, to pass validation: %v, but got the following errors: %v", v.name, v.conf, err.Multi.Errors)
		}
		if err == nil && v.errCount != 0 {
			t.Fatalf("Expected config: %v to fail validation, but it didn't: %v", v.name, v.conf)
		}
		if (err != nil && v.errCount != 0) && (len(err.Multi.Errors) != v.errCount) {
			t.Fatalf("Expected config to generate %d errors; got %d ;\n errors: %v", v.errCount, len(err.Multi.Errors), err.Multi.Errors)
		}
	}
}

func TestIssuersFromConfig(t *testing.T) {
	testConfigs := []struct {
		name string
		conf *tokenConfig.Params
	}{
		{
			"empty config (default)",
			&tokenConfig.Params{}, //default empty config is valid - no issuer objects will be created
		},
		{
			"empty issuer array config",
			&tokenConfig.Params{Issuers: make([]*tokenConfig.Issuer, 0)}, //no issuer objects will be created
		},
		{
			"valid single issuer",
			&tokenConfig.Params{
				Issuers: []*tokenConfig.Issuer{
					{Name: "w3", PubKeyUrl: "W3.iss1.com/pubkeys:7670"},
				},
			},
		},
		{
			"valid single issuer with claim names and re-names",
			&tokenConfig.Params{
				Issuers: []*tokenConfig.Issuer{
					{Name: "w3", PubKeyUrl: "W3.iss1.com/pubkeys:7670", ClaimNames: []string{"sub", "admin", "servers.NY.ip"}},
				},
			},
		},
		{
			"valid multiple issuers",
			&tokenConfig.Params{
				Issuers: []*tokenConfig.Issuer{
					{Name: "w3", PubKeyUrl: "W3.iss1.com/pubkeys:7670", ClaimNames: []string{"sub", "admin", "servers.NY.ip"}},
					{Name: "giss", PubKeyUrl: "login.iss2.com/pubkeys:5120", ClaimNames: []string{"sub", "admin", "last-login"}},
				},
			},
		},
	}

	for _, v := range testConfigs {
		cfg, err := NewTokenConfig(v.conf)
		if err != nil {
			t.Fatalf("Expected config: %v, to generate config issuer objects successfuly: %v, but got the following error: %v", v.name, v.conf, err)
		}
		if issNum := len(v.conf.Issuers); len(cfg.Issuers) != issNum {
			t.Fatalf("Expected config: %v, to generate %v issuer objects, but got %v", v.name, issNum, len(cfg.Issuers))
		}
		for _, issuer := range v.conf.Issuers {
			if _, exists := cfg.Issuers[issuer.Name]; !exists {
				t.Fatalf("Expected config: %v, to create an issuer object named %v, but it didn't", v.name, issuer.Name)
			}
		}
	}

}


func TestPubkeyFetch(t *testing.T){
	testConfigs := []struct {
		name string
		conf *tokenConfig.Params
		isValidIssuer []bool
	}{
		{
			"single working issuer",
			&tokenConfig.Params{
				Issuers: []*tokenConfig.Issuer{
					{Name: "Iam", PubKeyUrl: "https://iam.ng.bluemix.net/oidc/jwks"},
				},
			},
			[]bool{true},
		},
		{
			"working issuer and more non-working issuers",
			&tokenConfig.Params{
				Issuers: []*tokenConfig.Issuer{
					{Name: "Iam", PubKeyUrl: "https://iam.ng.bluemix.net/oidc/jwks"},
					{Name: "almostIam", PubKeyUrl: "https://iam.ng.bluemix.net/oidc/jwksy"},
					{Name: "jwks24/7", PubKeyUrl: "https://keys.all.day.org"},
				},
			},
			[]bool{true,false,false},
		},
	}

	for _,v := range testConfigs{

		b := newBuilder()
		env := test.NewEnv(t)
		a, err := b.BuildAttributesGenerator(env, v.conf)
		tag := a.(*tokenAttrGen)
		if err != nil {
			t.Errorf("Unable to create aspect: %v", err)
		}
		for i,issuer := range v.conf.Issuers{
			iss := tag.cfg.Issuers[issuer.Name].(*defaultJWTIssuer)
			iss.RLock()
			if v.isValidIssuer[i] && len(iss.pubKeys) <= 0{
				t.Errorf("Expected keys to be fetched from working issuer: %v, but they werent", iss.name)
			}
			if !v.isValidIssuer[i] && len(iss.pubKeys) > 0{
				t.Errorf("issuer: %v isn't real, didn't expect to find keys in cache.", iss.name)
			}
			iss.RUnlock()
		}
		if err := tag.Close(); err != nil {
			t.Errorf("Unable to close aspect: %v", err)
		}
		if err := b.Close(); err != nil {
			t.Errorf("Unable to close builder: %v", err)
		}
	}

}
