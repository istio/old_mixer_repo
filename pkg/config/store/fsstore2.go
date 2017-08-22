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

package store

import (
	"context"
	"crypto/sha1"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"
)

const defaultDuration = time.Second / 2

// FsStore2 is Store2Backend implementation using filesystem.
type FsStore2 struct {
	root          string
	kinds         []string
	checkDuration time.Duration
	chs           *ContextChList

	mu   sync.RWMutex
	data map[Key]map[string]interface{}
	shas map[Key][sha1.Size]byte
}

var _ Store2Backend = &FsStore2{}

func (s *FsStore2) readFiles() (map[Key][]byte, map[Key][sha1.Size]byte) {
	const suffix = ".yaml"
	result := map[Key][]byte{}

	for _, kind := range s.kinds {
		root := filepath.Join(s.root, kind)
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if strings.HasSuffix(path, suffix) && (info.Mode()&os.ModeType) == 0 {
				data, err := ioutil.ReadFile(path)
				if err != nil {
					glog.Warningf("Failed to read %s: %v", path, err)
					return nil
				}
				path = path[len(root)+1 : len(path)-len(suffix)]
				paths := strings.Split(path, string(filepath.Separator))
				if len(paths) != 2 {
					glog.Warningf("Unrecognized path pattern %s", path)
					return nil
				}
				key := Key{Kind: kind, Namespace: paths[0], Name: paths[1]}
				result[key] = data
			}
			return nil
		})
		if err != nil {
			glog.Errorf("failure during filepath.Walk: %v", err)
		}
	}
	shas := make(map[Key][sha1.Size]byte, len(result))
	for k, b := range result {
		shas[k] = sha1.Sum(b)
	}
	return result, shas
}

func (s *FsStore2) update() {
	newData, newShas := s.readFiles()
	s.mu.Lock()
	defer s.mu.Unlock()
	updated := []Key{}
	removed := map[Key]bool{}
	for k := range s.shas {
		removed[k] = true
	}
	for k, v := range newShas {
		oldV, ok := s.shas[k]
		if !ok {
			updated = append(updated, k)
			continue
		}
		delete(removed, k)
		if v != oldV {
			updated = append(updated, k)
		}
	}
	if len(updated) == 0 && len(removed) == 0 {
		return
	}
	s.shas = newShas
	evs := make([]BackendEvent, 0, len(updated)+len(removed))
	for _, key := range updated {
		parsed := map[string]interface{}{}
		glog.Errorf("%+v", key)
		if err := yaml.Unmarshal(newData[key], &parsed); err != nil {
			glog.Errorf("failed to parse yaml content: %v", err)
			continue
		}
		s.data[key] = parsed
		evs = append(evs, BackendEvent{Key: key, Type: Update, Value: parsed})
	}
	for key := range removed {
		delete(s.data, key)
		evs = append(evs, BackendEvent{Key: key, Type: Delete})
	}
	for _, ev := range evs {
		s.chs.Send(ev)
	}
}

// NewFsStore2 creates a new FsStore2 instance.
func NewFsStore2(root string) *FsStore2 {
	return &FsStore2{
		root:          root,
		checkDuration: defaultDuration,
		chs:           NewContextChList(),
		shas:          map[Key][sha1.Size]byte{},
		data:          map[Key]map[string]interface{}{},
	}
}

// Init implements Store2Backend interface.
func (s *FsStore2) Init(ctx context.Context, kinds []string) error {
	// TBD
	s.kinds = kinds
	s.update()
	go func() {
		tick := time.NewTicker(s.checkDuration)
		for {
			select {
			case <-ctx.Done():
				tick.Stop()
				return
			case <-tick.C:
				s.update()
			}
		}
	}()
	return nil
}

// Watch implements Store2Backend interface.
func (s *FsStore2) Watch(ctx context.Context) (<-chan BackendEvent, error) {
	return s.chs.Add(ctx), nil
}

// Get implements Store2Backend interface.
func (s *FsStore2) Get(key Key) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	spec, ok := s.data[key]
	if !ok {
		return nil, ErrNotFound
	}
	return spec, nil
}

// List implements Store2Backend interface.
func (s *FsStore2) List() map[Key]map[string]interface{} {
	s.mu.RLock()
	result := make(map[Key]map[string]interface{}, len(s.data))
	for k, v := range s.data {
		result[k] = v
	}
	s.mu.RUnlock()
	return result
}
