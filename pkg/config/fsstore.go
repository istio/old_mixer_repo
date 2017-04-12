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

// Package config FSStore
// implements file system KeyValueStore interface
package config

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
)

// FSStore implements file system KeyValueStore and change Store
type FSStore struct {
	// root directory
	root string
	// file suffix
	suffix string

	// testing and fault injection
	writeCloserGetter writeCloserFunc
	readfile          readFileFunc
	mkdirAll          mkdirAllFunc
	remove            removeFunc
}

// for testing
type writeCloser interface {
	io.WriteCloser
	Name() string
}
type writeCloserFunc func(dir string, prefix string) (f writeCloser, err error)
type readFileFunc func(filename string) ([]byte, error)
type mkdirAllFunc func(path string, perm os.FileMode) error
type removeFunc func(name string) error

func newFSStore(root string) *FSStore {
	return &FSStore{
		root:   root,
		suffix: ".yml",
		writeCloserGetter: func(a, b string) (f writeCloser, err error) {
			f, err = ioutil.TempFile(a, b)
			return
		},
		readfile: ioutil.ReadFile,
		mkdirAll: os.MkdirAll,
		remove:   os.Remove,
	}
}

func (f *FSStore) String() string {
	return fmt.Sprintf("FSStore: %s", f.root)
}

// NewCompatFSStore creates and returns an FSStore using old style
// globalConfig and serviceConfig
func NewCompatFSStore(globalConfig string, serviceConfig string) (fs *FSStore, err error) {
	// no configURL, but serviceConfig and globalConfig are specified.
	// compatibility
	var data []byte
	if data, err = ioutil.ReadFile(globalConfig); err != nil {
		return nil, err
	}
	gc := string(data)

	if data, err = ioutil.ReadFile(serviceConfig); err != nil {
		return nil, err
	}
	sc := string(data)

	var dir string
	if dir, err = ioutil.TempDir(os.TempDir(), "FSStore"); err != nil {
		return nil, err
	}

	fs = newFSStore(dir)

	// intialize FSstore

	if _, err = fs.Set(keyGlobalServiceConfig, sc); err != nil {
		return nil, err
	}

	if _, err = fs.Set(keyDescriptors, gc); err != nil {
		return nil, err
	}

	if _, err = fs.Set(keyAdapters, gc); err != nil {
		return nil, err
	}
	return fs, nil
}

// force compile time check.
var _ KeyValueStore = &FSStore{}

func (f *FSStore) getPath(key string) string {
	return path.Join(f.root, key)
}

const indexNotSupported = -1

// Get value at a key, false if not found.
func (f *FSStore) Get(key string) (value string, index int, found bool) {
	p := f.getPath(key) + f.suffix
	var b []byte
	var err error

	if b, err = f.readfile(p); err != nil {
		if !os.IsNotExist(err) {
			glog.Warningf("Could not access %s: %s", p, err)
		}
		return "", indexNotSupported, false
	}
	return string(b), indexNotSupported, true
}

// Set a value
func (f *FSStore) Set(key string, value string) (index int, err error) {
	p := f.getPath(key) + f.suffix
	if err = f.mkdirAll(filepath.Dir(p), os.ModeDir|os.ModePerm); err != nil {
		return indexNotSupported, err
	}

	var tf writeCloser
	if tf, err = f.writeCloserGetter(f.root, "FSStore_Set"); err != nil {
		return indexNotSupported, err
	}
	if _, err = tf.Write([]byte(value)); err != nil {
		return indexNotSupported, err
	}
	if err = tf.Close(); err != nil {
		return indexNotSupported, err
	}
	// atomically rename
	return indexNotSupported, os.Rename(tf.Name(), p)
}

// List keys with the prefix
func (f *FSStore) List(key string, recurse bool) (keys []string, index int, err error) {
	keys = make([]string, 0, 10)
	cc := &keys

	err = filepath.Walk(f.getPath(key), func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(path, f.suffix) {
			*cc = append(*cc, path[len(f.root):len(path)-len(f.suffix)])
		}
		return nil
	})
	return keys, indexNotSupported, err
}

// Delete removes a key from the fs store.
func (f *FSStore) Delete(key string) (err error) {
	p := f.getPath(key) + f.suffix
	if err = f.remove(p); err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}
