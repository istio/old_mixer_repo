// Copyright 2016 Google Ina.
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
	"crypto/sha1"
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"io"

	"gopkg.in/yaml.v2"
	"istio.io/mixer/adapter/ipListChecker/config"
	"istio.io/mixer/pkg/adapter"
)

type (
	builder struct{ adapter.DefaultBuilder }

	listChecker struct {
		log           adapter.Logger
		providerURL   string
		atomicList    atomic.Value
		client        http.Client
		refreshTicker *time.Ticker
		purgeTimer    *time.Timer
		ttl           time.Duration
	}

	listState struct {
		entries    []*net.IPNet
		entriesSha [sha1.Size]byte
		fetchError error
	}
)

var (
	name = "ipListChecker"
	desc = "Checks whether an IP address is present in an IP address list."
	conf = &config.Params{
		ProviderUrl:     "http://localhost",
		RefreshInterval: 60,
		Ttl:             300,
	}
)

// Register records the builders exposed by this adapter.
func Register(r adapter.Registrar) {
	r.RegisterListsBuilder(newBuilder())
}

func newBuilder() adapter.ListsBuilder {
	return builder{adapter.NewDefaultBuilder(name, desc, conf)}
}

func (builder) NewListsAspect(env adapter.Env, c adapter.AspectConfig) (adapter.ListsAspect, error) {
	return newListChecker(env, c.(*config.Params))
}

func (builder) ValidateConfig(cfg adapter.AspectConfig) (ce *adapter.ConfigErrors) {
	c := cfg.(*config.Params)

	u, err := url.Parse(c.ProviderUrl)
	if err != nil {
		ce = ce.Append("ProviderUrl", err)
	} else {
		if u.Scheme == "" || u.Host == "" {
			ce = ce.Appendf("ProviderUrl", "URL scheme and host cannot be empty")
		}
	}

	return
}

func newListChecker(env adapter.Env, c *config.Params) (*listChecker, error) {
	return newListCheckerWithTimers(env, c,
		time.NewTicker(time.Second*time.Duration(c.RefreshInterval)),
		time.NewTimer(time.Second*time.Duration(c.Ttl)))
}

func newListCheckerWithTimers(env adapter.Env, c *config.Params,
	refreshTicker *time.Ticker, purgeTimer *time.Timer) (*listChecker, error) {
	l := &listChecker{
		log:           env.Logger(),
		providerURL:   c.ProviderUrl,
		refreshTicker: refreshTicker,
		purgeTimer:    purgeTimer,
		ttl:           time.Second * time.Duration(c.Ttl),
	}
	l.setListState(listState{})

	// load up the list synchronously so we're ready to accept traffic immediately
	_ = l.fetchList()
	purgeTimer.Reset(l.ttl)

	// go routine to periodically refresh the list
	go func() {
		for range refreshTicker.C {
			if err := l.fetchList(); err == nil {
				// reset the next purge time
				purgeTimer.Reset(l.ttl)
			}
		}
	}()

	// go routine to periodically purge the list
	go func() {
		for range purgeTimer.C {
			ls := l.getListState()
			ls.entries = nil
			ls.entriesSha = [20]byte{}
			l.setListState(ls)
		}
	}()

	return l, nil
}

func (l *listChecker) Close() error {
	l.refreshTicker.Stop()
	l.purgeTimer.Stop()
	return nil
}

func (l *listChecker) CheckList(symbol string) (bool, error) {
	ipa := net.ParseIP(symbol)
	if ipa == nil {
		// invalid symbol format
		return false, errors.New(symbol + " is not a valid IP address")
	}

	// get an atomic snapshot of the current list
	ls := l.getListState()
	if len(ls.entries) == 0 {
		return false, ls.fetchError
	}

	for _, ipnet := range ls.entries {
		if ipnet.Contains(ipa) {
			return true, nil
		}
	}

	// not found in the list
	return false, nil
}

// Typed accessors for the atomic list
func (l *listChecker) getListState() listState {
	return l.atomicList.Load().(listState)
}

// Typed accessor for the atomic list
func (l *listChecker) setListState(ls listState) {
	l.atomicList.Store(ls)
}

// represents the format of the data in a list
type listPayload struct {
	WhiteList []string `yaml:"whitelist" required:"true"`
}

func (l *listChecker) fetchList() error {
	l.log.Infof("Fetching list from %s", l.providerURL)

	resp, err := l.client.Get(l.providerURL)
	if err != nil {
		ls := l.getListState()
		ls.fetchError = err
		l.setListState(ls)
		l.log.Warningf("Could not connect to %s: %v", l.providerURL, err)
		return err
	}

	return l.fetchListWithBody(resp.Body)
}

func (l *listChecker) fetchListWithBody(body io.Reader) error {
	ls := l.getListState()

	// TODO: could lead to OOM since this is unbounded
	buf, err := ioutil.ReadAll(body)
	if err != nil {
		ls.fetchError = err
		l.setListState(ls)
		l.log.Warningf("Could not read from %s: %v", l.providerURL, err)
		return err
	}

	// determine whether the ls has changed since the last fetch
	newsha := sha1.Sum(buf)
	if newsha == ls.entriesSha {
		// the ls hasn't changed since last time, just bail
		l.log.Infof("Fetched ls is unchanged")
		return nil
	}

	// now parse
	lp := listPayload{}
	err = yaml.Unmarshal(buf, &lp)
	if err != nil {
		ls.fetchError = err
		l.setListState(ls)
		l.log.Warningf("Could not unmarshal data from %s: %v", l.providerURL, err)
		return err
	}

	// copy to the internal format
	entries := make([]*net.IPNet, 0, len(lp.WhiteList))
	for _, ip := range lp.WhiteList {
		if !strings.Contains(ip, "/") {
			ip += "/32"
		}

		_, ipnet, err := net.ParseCIDR(ip)
		if err != nil {
			l.log.Warningf("Skipping unparsable entry %s: %v", ip, err)
			continue
		}
		entries = append(entries, ipnet)
	}

	// Now install the new ls
	l.log.Infof("Installing updated ls with %d entries", len(entries))
	ls.entries = entries
	ls.entriesSha = newsha
	ls.fetchError = nil
	l.setListState(ls)

	return nil
}
