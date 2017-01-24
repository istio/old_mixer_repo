// Copyright 2017 Google Inc.
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

package config

import (
	"crypto/sha1"
	"io/ioutil"
	"time"

	"github.com/golang/glog"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/expr"
)

// ChangeListener listens for config change notifications.
type ChangeListener interface {
	ConfigChange(cfg *Runtime)
}

// Manager represents the config Manager.
// It is responsible for fetching and receiving configuration changes.
// It applied validated changes to the registered config change listeners.
// api.Handler listens for config changes.
type Manager struct {
	ManagerArgs

	cl      []ChangeListener
	closing chan bool
	scSha   [sha1.Size]byte
	gcSha   [sha1.Size]byte
}

// ManagerArgs are constructor args for a manager
type ManagerArgs struct {
	Eval          expr.Evaluator
	AspectF       ValidatorFinder
	BuilderF      ValidatorFinder
	GlobalConfig  string
	ServiceConfig string
}

// NewManager returns a config.Manager given ManagerArgs.
func NewManager(args *ManagerArgs) *Manager {
	return &Manager{ManagerArgs: *args}
}

// Register makes the ConfigManager aware of a ConfigChangeListener.
func (c *Manager) Register(cc ChangeListener) {
	c.cl = append(c.cl, cc)
}

func read(fname string) ([sha1.Size]byte, string, error) {
	var data []byte
	var err error
	if data, err = ioutil.ReadFile(fname); err != nil {
		return [sha1.Size]byte{}, "", err
	}
	return sha1.Sum(data), string(data[:]), nil
}

// fetch config and return runtime if a new one is available.
func (c *Manager) fetch() (*Runtime, error) {
	var vd *Validated
	var cerr *adapter.ConfigErrors

	scSha, sc, err1 := read(c.ServiceConfig)
	if err1 != nil {
		return nil, err1
	}

	gcSha, gc, err2 := read(c.GlobalConfig)
	if err2 != nil {
		return nil, err2
	}

	if gcSha == c.gcSha && scSha == c.scSha {
		glog.V(2).Info("No Change")
		return nil, nil
	}

	v := NewValidator(c.AspectF, c.BuilderF, true, c.Eval)
	if vd, cerr = v.Validate(sc, gc); cerr != nil {
		return nil, cerr
	}

	c.gcSha = gcSha
	c.scSha = scSha
	return NewRuntime(vd, c.Eval), nil
}

// fetchAndNotify fetches a new config and notifies listeners if something has changed
func (c *Manager) fetchAndNotify() error {
	rt, err := c.fetch()
	if err != nil {
		return err
	}
	if rt == nil {
		return nil
	}

	glog.Infof("Installing new config from %s sha=%x ", c.ServiceConfig, c.scSha)
	for _, cl := range c.cl {
		cl.ConfigChange(rt)
	}
	return nil
}

// Close stops the config manager go routine.
func (c *Manager) Close() { close(c.closing) }

func (c *Manager) loop() {
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()
	done := false
	for !done {
		select {
		case <-ticker.C:
			err := c.fetchAndNotify()
			if err != nil {
				glog.Warning(err)
			}
		case <-c.closing:
			done = true
		}
	}
}

// Start watching for configuration changes and handle updates.
func (c *Manager) Start() {
	err := c.fetchAndNotify()
	// We make an attempt to synchronously fetch and notify configuration
	// If it is not successful, we will continue to watch for changes.
	go c.loop()
	if err != nil {
		glog.Warning("Unable to process config", err)
	}
}
