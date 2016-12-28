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
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/golang/protobuf/proto"

	"istio.io/mixer/pkg/aspect/listChecker"
	"istio.io/mixer/pkg/aspectsupport"
)

const (
	// ImplName is the canonical name of this implementation
	ImplName = "istio/IPListChecker"
)

// Register registration entry point
func Register(r aspectsupport.Registry) {
	r.RegisterCheckList(&adapterState{})
}

type adapterState struct{}

func (a *adapterState) Name() string {
	return ImplName
}

func (a *adapterState) Description() string {
	return "Checks whether an IP address is present in an IP address list."
}

func (a *adapterState) DefaultConfig() proto.Message {
	return &Config{
		ProviderUrl:     "http://localhost",
		RefreshInterval: 60,
		Ttl:             120,
	}
}

func (a *adapterState) ValidateConfig(cfg proto.Message) (err error) {
	c, ok := cfg.(*Config)
	if !ok {
		return fmt.Errorf("Invalid message type %#v", cfg)
	}
	var u *url.URL

	if u, err = url.Parse(c.ProviderUrl); err == nil {
		if u.Scheme == "" || u.Host == "" {
			err = errors.New("Scheme and Host cannot be nil")
		}
	}
	return err
}

func (a *adapterState) Close() error {
	return nil
}

func (a *adapterState) NewAspect(c *listChecker.AdapterConfig) (listChecker.Aspect, error) {
	if err := a.ValidateConfig(c.Message); err != nil {
		return nil, err
	}
	config, found := c.Message.(*Config)
	if !found {
		return nil, fmt.Errorf("Config not found %#v", c.Message)
	}
	var u *url.URL
	var err error
	if u, err = url.Parse(config.ProviderUrl); err != nil {
		// bogus URL format
		return nil, err
	}

	aa := aspectState{
		backend:         u,
		closing:         make(chan bool),
		refreshInterval: time.Second * time.Duration(config.RefreshInterval),
		ttl:             time.Second * time.Duration(config.Ttl),
	}
	aa.client = http.Client{Timeout: aa.ttl}

	// install an empty list
	aa.setList([]*net.IPNet{})

	// load up the list synchronously so we're ready to accept traffic immediately
	aa.refreshList()

	// crank up the async list refresher
	go aa.listRefresher()

	return &aa, nil
}
