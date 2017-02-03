// Copyright 2016 Google Inc.
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

package ipListChecker

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"gopkg.in/yaml.v2"

	"time"

	"istio.io/mixer/adapter/ipListChecker/config"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/adapter/test"
)

type junk struct{}

func (junk) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("nothing good ever happens to me")
}

func TestBasic(t *testing.T) {
	list0 := listPayload{
		WhiteList: []string{"10.10.11.2", "10.10.11.3", "9.9.9.9/28"},
	}

	list1 := listPayload{
		WhiteList: []string{"10.10.11.2", "X", "10.10.11.3"},
	}

	list2 := []string{"JUNK"}

	listToUse := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var out []byte
		var err error

		if listToUse == 0 {
			out, err = yaml.Marshal(list0)
		} else if listToUse == 1 {
			out, err = yaml.Marshal(list1)
		} else {
			out, err = yaml.Marshal(list2)
		}

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if _, err := w.Write(out); err != nil {
			t.Errorf("w.Write failed: %v", err)
		}
	}))
	defer ts.Close()
	b := newBuilder()

	config := config.Params{
		ProviderUrl:     ts.URL,
		RefreshInterval: 1,
		Ttl:             10,
	}

	a, err := b.NewListsAspect(test.NewEnv(t), &config)
	if err != nil {
		t.Errorf("Unable to create aspect: %v", err)
	}

	cases := []struct {
		addr   string
		result bool
		fail   bool
	}{
		{"10.10.11.2", true, false},
		{"9.9.9.1", true, false},
		{"120.10.11.2", false, false},
		{"XYZ", false, true},
	}

	for _, c := range cases {
		ok, err := a.CheckList(c.addr)
		if (err != nil) != c.fail {
			t.Errorf("CheckList(%s): did not expect err '%v'", c.addr, err)
		}

		if ok != c.result {
			t.Errorf("CheckList(%s): expecting '%v', got '%v'", c.addr, c.result, ok)
		}
	}

	lc := a.(*listChecker)

	// do a NOP refresh of the same data
	if err := lc.fetchList(); err != nil {
		t.Errorf("Unable to refresh list: %v", err)
	}

	// now try to parse a list with errors
	listToUse = 1
	if err := lc.fetchList(); err != nil {
		t.Errorf("Unable to refresh list: %v", err)
	}
	list := lc.getListState()
	if len(list.entries) != 2 {
		t.Errorf("Expecting %d, got %d entries", len(list1.WhiteList)-1, len(list.entries))
	}

	// now try to parse a list in the wrong format
	listToUse = 2
	if err := lc.fetchList(); err == nil {
		t.Errorf("Expecting error, got success")
	}
	list = lc.getListState()
	if len(list.entries) != 2 {
		t.Errorf("Expecting %d, got %d entries", len(list1.WhiteList)-1, len(list.entries))
	}

	// now try to process an incorrect body
	if err := lc.fetchListWithBody(junk{}); err == nil {
		t.Errorf("Expecting failure, got success")
	}

	if err := a.Close(); err != nil {
		t.Errorf("Unable to close aspect: %v", err)
	}

	if err := b.Close(); err != nil {
		t.Errorf("Unable to close builder: %v", err)
	}
}

func TestBadUrl(t *testing.T) {
	b := newBuilder()

	config := config.Params{
		ProviderUrl:     "http://abadurl.com",
		RefreshInterval: 1,
		Ttl:             10,
	}

	a, err := b.NewListsAspect(test.NewEnv(t), &config)
	if err != nil {
		t.Errorf("Unable to create aspect: %v", err)
	}

	if _, err = a.CheckList("1.2.3.4"); err == nil {
		t.Errorf("Expecting failure, got success")
	}

	if err := a.Close(); err != nil {
		t.Errorf("Unable to close aspect: %v", err)
	}

	if err := b.Close(); err != nil {
		t.Errorf("Unable to close builder: %v", err)
	}
}

func TestValidateConfig(t *testing.T) {
	type testCase struct {
		cfg   config.Params
		field string
	}

	cases := []testCase{
		{
			cfg:   config.Params{ProviderUrl: "Foo", RefreshInterval: 1, Ttl: 2},
			field: "ProviderUrl",
		},

		{
			cfg:   config.Params{ProviderUrl: ":", RefreshInterval: 1, Ttl: 2},
			field: "ProviderUrl",
		},

		{
			cfg:   config.Params{ProviderUrl: "http:", RefreshInterval: 1, Ttl: 2},
			field: "ProviderUrl",
		},

		{
			cfg:   config.Params{ProviderUrl: "http://", RefreshInterval: 1, Ttl: 2},
			field: "ProviderUrl",
		},

		{
			cfg:   config.Params{ProviderUrl: "http:///FOO", RefreshInterval: 1, Ttl: 2},
			field: "ProviderUrl",
		},
	}

	b := newBuilder()
	for i, c := range cases {
		err := b.ValidateConfig(&c.cfg).Multi.Errors[0].(adapter.ConfigError)
		if err.Field != c.field {
			t.Errorf("Case %d: expecting error for field %s, got %s", i, c.field, err.Field)
		}
	}
}

func TestRefreshAndPurge(t *testing.T) {
	list := listPayload{
		WhiteList: []string{"10.10.11.2", "10.10.11.3", "9.9.9.9/28"},
	}

	fetched := make(chan bool, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var out []byte
		var err error

		out, err = yaml.Marshal(list)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if _, err := w.Write(out); err != nil {
			t.Errorf("w.Write failed: %v", err)
		}
		fetched <- true
	}))
	defer ts.Close()

	config := config.Params{
		ProviderUrl:     ts.URL,
		RefreshInterval: 1,
		Ttl:             10,
	}

	refreshChan := make(chan time.Time)
	purgeChan := make(chan time.Time)

	refreshTicker := time.NewTicker(time.Second * 3600)
	purgeTimer := time.NewTimer(time.Second * 3600)

	refreshTicker.C = refreshChan
	purgeTimer.C = purgeChan

	a, err := newListCheckerWithTimers(test.NewEnv(t), &config, refreshTicker, purgeTimer)
	if err != nil {
		t.Errorf("Unable to create aspect: %v", err)
	}

	<-fetched

	// cause a refetch
	refreshChan <- time.Now()
	<-fetched

	// cause a purge
	purgeChan <- time.Now()

	// wait for the purge
	for len(a.getListState().entries) == 0 {
		time.Sleep(time.Millisecond)
	}

	// cause a refetch
	refreshChan <- time.Now()
	<-fetched

	if len(a.getListState().entries) == 0 {
		t.Errorf("Expecting the list to contain some entries, but it was empty")
	}

	if err := a.Close(); err != nil {
		t.Errorf("Unable to close aspect: %v", err)
	}
}

func TestInvariants(t *testing.T) {
	test.AdapterInvariants(Register, t)
}
