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
// implements file system KVStore interface
package config

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
)

// FSStore Implements file system KVStore and change Store
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

// force compile time check.
var _ KVStore = &FSStore{}

func (f *FSStore) getPath(key string) string {
	return f.root + string(os.PathSeparator) + key
}

// Get value at a key, false if not found.
func (f *FSStore) Get(key string) (value string, index int, found bool) {
	p := f.getPath(key) + f.suffix
	b, err := f.readfile(p)
	if err != nil {
		if !os.IsNotExist(err) {
			glog.Warningf("Could not access %s: %s", p, err)
		}
		return "", -1, false
	}
	return string(b), -1, true
}

// Set a value
func (f *FSStore) Set(key string, value string) (index int, err error) {
	p := f.getPath(key) + f.suffix
	if err = f.mkdirAll(filepath.Dir(p), os.ModeDir|os.ModePerm); err != nil {
		return -1, err
	}

	var tf writeCloser
	tf, err = f.writeCloserGetter(os.TempDir(), "FSStore_Set")
	if err != nil {
		return -1, err
	}
	_, err = tf.Write([]byte(value))
	if err != nil {
		return -1, err
	}
	err = tf.Close()
	if err != nil {
		return -1, err
	}
	// atomically rename
	return -1, os.Rename(tf.Name(), p)
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
	return keys, -1, err
}

// Delete removed a key from the fs store.
func (f *FSStore) Delete(key string) (err error) {
	p := f.getPath(key) + f.suffix
	err = f.remove(p)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}
