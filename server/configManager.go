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

package main

import "istio.io/mixer/config"

// ConfigManager keeps track of the configuration, and manages configured adapter instances.
type ConfigManager struct {
}

// NewConfigManager returns a new ConfigManager instance
func NewConfigManager() (*ConfigManager, error) {
	var mgr = ConfigManager{}
	return &mgr, nil
}

// GetListCheckerConfig returns a list of ListCheckerConfigs for the given serverID/peerID values.
func (manager *ConfigManager) GetListCheckerConfig(serverID string, peerID string) ([]*config.ListCheckerConfig, error) {
	// TODO: return an empty list for now, until we have support for configuration reading/evaluating.
	return make([]*config.ListCheckerConfig, 0), nil
}
