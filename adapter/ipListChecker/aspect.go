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

	"github.com/golang/glog"
	"gopkg.in/yaml.v2"
)

type aspectState struct {
	backend         *url.URL
	atomicList      atomic.Value
	fetchedSha      [sha1.Size]byte
	refreshInterval time.Duration
	ttl             time.Duration
	closing         chan bool
	fetchError      error
	client          http.Client
}

func (a *aspectState) Name() string {
	return ImplName
}

func (a *aspectState) Close() error {
	close(a.closing)
	return nil
}

func (a *aspectState) CheckList(symbol string) (bool, error) {
	ipa := net.ParseIP(symbol)
	if ipa == nil {
		// invalid symbol format
		return false, errors.New(symbol + " is not a valid IP address")
	}

	// get an atomic snapshot of the current list
	l := a.getList()
	if l == nil || len(l) == 0 {
		// TODO: would be nice to return the last I/O error received from the provider...
		return false, a.fetchError
	}

	for _, ipnet := range l {
		if ipnet.Contains(ipa) {
			return true, nil
		}
	}

	// not found in the list
	return false, nil
}

// Typed accessors for the atomic list
func (a *aspectState) getList() []*net.IPNet {
	return a.atomicList.Load().([]*net.IPNet)
}

// Typed accessor for the atomic list
func (a *aspectState) setList(l []*net.IPNet) {
	a.atomicList.Store(l)
}

// Updates the list by polling from the provider on a fixed interval
func (a *aspectState) listRefresher() {
	refreshTicker := time.NewTicker(a.refreshInterval)
	purgeTimer := time.NewTimer(a.ttl)

	defer refreshTicker.Stop()
	defer purgeTimer.Stop()

	for {
		select {
		case <-refreshTicker.C:
			// fetch a new list and reset the TTL timer
			a.refreshList()
			purgeTimer.Reset(a.ttl)

		case <-purgeTimer.C:
			// times up, nuke the list and start returning errors
			a.setList(nil)

		case <-a.closing:
			return
		}
	}
}

// represents the format of the data in a list
type listPayload struct {
	WhiteList []string `yaml:"whitelist" required:"true"`
}

func (a *aspectState) refreshList() {
	glog.Infoln("Fetching list from ", a.backend.String())

	resp, err := a.client.Get(a.backend.String())
	if err != nil {
		a.fetchError = err
		glog.Warning("Could not connect to ", a.backend.String(), " ", err)
		return
	}

	// TODO: could lead to OOM since this is unbounded
	var buf []byte
	buf, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		a.fetchError = err
		glog.Warning("Could not read from ", a.backend.String(), " ", err)
		return
	}

	// determine whether the list has changed since the last fetch
	// Note that a.fetchedSha is only read and written by this function
	// in a single thread
	newsha := sha1.Sum(buf)
	if newsha == a.fetchedSha {
		// the list hasn't changed since last time, just bail
		glog.Infoln("Fetched list is unchanged")
		return
	}

	// now parse
	lp := listPayload{}
	err = yaml.Unmarshal(buf, &lp)
	if err != nil {
		a.fetchError = err
		glog.Warning("Could not unmarshal ", a.backend.String(), " ", err)
		return
	}

	// copy to the internal format
	l := make([]*net.IPNet, 0, len(lp.WhiteList))
	for _, ip := range lp.WhiteList {
		if !strings.Contains(ip, "/") {
			ip += "/32"
		}

		_, ipnet, err := net.ParseCIDR(ip)
		if err != nil {
			glog.Warningf("Unable to parse %s -- %v", ip, err)
			continue
		}
		l = append(l, ipnet)
	}

	// Now create a new map and install it
	glog.Infoln("Installing updated list")
	a.setList(l)
	a.fetchedSha = newsha
	a.fetchError = nil
}
