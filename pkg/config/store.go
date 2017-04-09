package config

import (
	"fmt"
	"strings"
)

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

// ChangeType denotes the type of change
type ChangeType int

const (
	// ChangeTypeUpdate - change was an update or a create to a key.
	ChangeTypeUpdate ChangeType = iota
	// ChangeTypeDelete - key was removed.
	ChangeTypeDelete
)

// Change a change record in the changelog
type Change struct {
	// Key that was affected
	Key string `json:"key"`
	// Type how did the key change
	Type ChangeType `json:"change_type"`
	// change log index number of the change
	Index int `json:"index"`
}

// KVStore defines the back end interface used by mixer
// and the mixer config API server
// It should support back ends like redis, etcd and NFS
// All commands should return an index which can be used
// to Read changes. If a KVStore does not support it,
// it may return -1
type KVStore interface {
	// Get value at a key, false if not found.
	Get(key string) (value string, index int, found bool)

	// Set a value.
	Set(key string, value string) (index int, err error)

	// List keys with the prefix.
	List(key string, recurse bool) (keys []string, index int, err error)

	// Delete a key.
	Delete(key string) error
}

// StoreListener listens for calls from the store that some keys have changed.
type StoreListener interface {
	// StoreChange notify listener that a new change is available
	StoreChange(index int)
}

// ChangeLogReader read change log from the KV Store
type ChangeLogReader interface {
	// ReadChangeLog reads change events >= index
	ReadChangeLog(index int) ([]*Change, error)
}

// ChangeNotifier implements change notification machinery for the KVStore
type ChangeNotifier interface {
	// Register StoreChangeListener
	// KVStore should call this method when there is a change
	// The client should issue ReadChangeLog to see what has changed if the call is available.
	// else it should re-read the store, perform diff and apply changes.
	RegisterStoreChangeListener(s StoreListener)
}

// URL types supported by the config store
const (
	// example fs:///tmp/testdata/configroot
	FSUrl = "fs"
	// example redis://:password@hostname:port/db_number
	RedisURL = "redis"
)

// NewStore create a new store based on the config URL.
func NewStore(configURL string) (KVStore, error) {
	urlParts := strings.Split(configURL, "://")

	if len(urlParts) < 2 {
		return nil, fmt.Errorf("invalid config URL %s %s", configURL, urlParts)
	}

	switch urlParts[0] {
	case FSUrl:
		return newFSStore(urlParts[1]), nil
	case RedisURL:
		return nil, fmt.Errorf("config URL %s not implemented", urlParts[0])
	}

	return nil, fmt.Errorf("unknown config URL %s %s", configURL, urlParts)
}