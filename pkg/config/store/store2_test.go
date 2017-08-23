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
	"net/url"
	"reflect"
	"testing"

	"github.com/gogo/protobuf/proto"

	cfg "istio.io/mixer/pkg/config/proto"
)

type memstore struct {
	data map[Key]map[string]interface{}
	ch   chan BackendEvent
}

func (m *memstore) Init(ctx context.Context, kinds []string) error {
	return nil
}

func (m *memstore) Watch(ctx context.Context) (<-chan BackendEvent, error) {
	m.ch = make(chan BackendEvent)
	return m.ch, nil
}

func (m *memstore) Get(key Key) (map[string]interface{}, error) {
	v, ok := m.data[key]
	if !ok {
		return nil, ErrNotFound
	}
	return v, nil
}

func (m *memstore) List() map[Key]map[string]interface{} {
	return m.data
}

func registerMemstore(builders map[string]Store2Builder) {
	builders["memstore"] = func(*url.URL) (Store2Backend, error) {
		return &memstore{data: map[Key]map[string]interface{}{}}, nil
	}
}

func TestStore2(t *testing.T) {
	r := NewRegistry2(registerMemstore)
	s, err := r.NewStore2("memstore://")
	if err != nil {
		t.Fatal(err)
	}
	m := s.(*store2).backend.(*memstore)
	kinds := map[string]proto.Message{"Handler": &cfg.Handler{}}
	if err = s.Init(context.Background(), kinds); err != nil {
		t.Fatal(err)
	}
	k := Key{Kind: "Handler", Name: "name", Namespace: "ns"}
	h1 := &cfg.Handler{}
	if err = s.Get(k, h1); err != ErrNotFound {
		t.Errorf("Got %v, Want ErrNotFound", err)
	}
	m.data[k] = map[string]interface{}{"name": "default", "adapter": "noop"}
	if err = s.Get(k, h1); err != nil {
		t.Errorf("Got %v, Want nil", err)
	}
	want := &cfg.Handler{Name: "default", Adapter: "noop"}
	if !reflect.DeepEqual(h1, want) {
		t.Errorf("Got %v, Want %v", h1, want)
	}
	wantList := map[Key]proto.Message{k: want}
	if lst := s.List(); !reflect.DeepEqual(lst, wantList) {
		t.Errorf("Got %+v, Want %+v", lst, wantList)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := s.Watch(ctx)
	if err != nil {
		t.Error(err)
	}
	m.ch <- BackendEvent{
		Key:   k,
		Type:  Update,
		Value: map[string]interface{}{"name": "default", "adapter": "noop"},
	}
	wantEv := Event{Key: k, Type: Update, Value: want}
	if ev := <-ch; !reflect.DeepEqual(ev, wantEv) {
		t.Errorf("Got %+v, Want %+v", ev, wantEv)
	}
}

func TestRegistry2(t *testing.T) {
	r := NewRegistry2(registerMemstore)
	for _, c := range []struct {
		u  string
		ok bool
	}{
		{"memstore://", true},
		{"mem://", false},
	} {
		_, err := r.NewStore2(c.u)
		ok := err == nil
		if ok != c.ok {
			t.Errorf("Want %v, Got %v, Err %v", c.ok, ok, err)
		}
	}
}
