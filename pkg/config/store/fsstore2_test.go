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
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/ghodss/yaml"
)

const testingCheckDuration = time.Millisecond * 5

func getTempFSStore2() (*fsStore2, string) {
	fsroot, _ := ioutil.TempDir("/tmp/", "fsStore2-")
	s := NewFsStore2(fsroot).(*fsStore2)
	s.checkDuration = testingCheckDuration
	return s, fsroot
}

func cleanupRootIfOK(t *testing.T, fsroot string) {
	if t.Failed() {
		t.Errorf("Test failed. The data remains at %s", fsroot)
		return
	}
	if err := os.RemoveAll(fsroot); err != nil {
		t.Errorf("Failed on cleanup %s: %v", fsroot, err)
	}
}

func waitFor(wch <-chan BackendEvent, ct ChangeType, key Key) {
	for ev := range wch {
		if ev.Key == key && ev.Type == ct {
			return
		}
	}
}

func write(fsroot string, k Key, data map[string]interface{}) error {
	path := filepath.Join(fsroot, k.Kind, k.Namespace, k.Name+".yaml")
	bytes, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return ioutil.WriteFile(path, bytes, 0644)
}

func TestFSStore2(t *testing.T) {
	s, fsroot := getTempFSStore2()
	defer cleanupRootIfOK(t, fsroot)
	const ns = "istio-mixer-testing"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Init(ctx, []string{"Handler", "Action"}); err != nil {
		t.Fatal(err.Error())
	}

	wch, err := s.Watch(ctx)
	if err != nil {
		t.Fatal(err.Error())
	}
	k := Key{Kind: "Handler", Namespace: ns, Name: "default"}
	if _, err = s.Get(k); err != ErrNotFound {
		t.Errorf("Got %v, Want ErrNotFound", err)
	}
	h := map[string]interface{}{"name": "default", "adapter": "noop"}
	if err = write(fsroot, k, h); err != nil {
		t.Fatalf("Got %v, Want nil", err)
	}
	waitFor(wch, Update, k)
	h2, err := s.Get(k)
	if err != nil {
		t.Errorf("Got %v, Want nil", err)
	}
	if !reflect.DeepEqual(h, h2) {
		t.Errorf("Got %+v, Want %+v", h2, h)
	}
	want := map[Key]map[string]interface{}{k: h2}
	if lst := s.List(); !reflect.DeepEqual(lst, want) {
		t.Errorf("Got %+v, Want %+v", lst, want)
	}
	h["adapter"] = "noop2"
	if err = write(fsroot, k, h); err != nil {
		t.Fatalf("Got %v, Want nil", err)
	}
	waitFor(wch, Update, k)
	if h2, err = s.Get(k); err != nil {
		t.Errorf("Got %v, Want nil", err)
	}
	if !reflect.DeepEqual(h, h2) {
		t.Errorf("Got %+v, Want %+v", h2, h)
	}
	if err = os.Remove(filepath.Join(fsroot, k.Kind, k.Namespace, k.Name+".yaml")); err != nil {
		t.Errorf("Got %v, Want nil", err)
	}
	waitFor(wch, Delete, k)
	if _, err := s.Get(k); err != ErrNotFound {
		t.Errorf("Got %v, Want ErrNotFound", err)
	}
}

func TestFSStore2WrongKind(t *testing.T) {
	s, fsroot := getTempFSStore2()
	defer cleanupRootIfOK(t, fsroot)
	const ns = "istio-mixer-testing"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Init(ctx, []string{"Action"}); err != nil {
		t.Fatal(err.Error())
	}

	k := Key{Kind: "Handler", Namespace: ns, Name: "default"}
	h := map[string]interface{}{"name": "default", "adapter": "noop"}
	if err := write(fsroot, k, h); err != nil {
		t.Error("Got nil, Want error")
	}
	time.Sleep(testingCheckDuration)

	if _, err := s.Get(k); err == nil {
		t.Errorf("Got nil, Want error")
	}
}

func TestFSStore2MissingRoot(t *testing.T) {
	s, fsroot := getTempFSStore2()
	if err := os.RemoveAll(fsroot); err != nil {
		t.Fatal(err)
	}
	if err := s.Init(context.Background(), []string{"Kind"}); err != nil {
		t.Errorf("Got %v, Want nil", err)
	}
	if lst := s.List(); len(lst) != 0 {
		t.Errorf("Got %+v, Want empty", lst)
	}
}

func TestFSStore2Robust(t *testing.T) {
	const ns = "testing"
	for _, c := range []struct {
		title   string
		prepare func(fsroot string) error
	}{
		{
			"wrong permission",
			func(fsroot string) error {
				path := filepath.Join(fsroot, "Handler", ns, "aa.yaml")
				return ioutil.WriteFile(path, []byte("foo: bar\n"), 0300)
			},
		},
		{
			"illformed file path",
			func(fsroot string) error {
				path := filepath.Join(fsroot, "Handler", "foo.yaml")
				return ioutil.WriteFile(path, []byte("foo: bar\n"), 0644)
			},
		},
		{
			"illformed yaml",
			func(fsroot string) error {
				path := filepath.Join(fsroot, "Handler", ns, "bb.yaml")
				return ioutil.WriteFile(path, []byte("abc"), 0644)
			},
		},
		{
			"directory",
			func(fsroot string) error {
				return os.MkdirAll(filepath.Join(fsroot, "Handler", ns, "cc.yaml"), 0755)
			},
		},
		{
			"unknown kind",
			func(fsroot string) error {
				k := Key{Kind: "Unknown", Namespace: ns, Name: "default"}
				return write(fsroot, k, map[string]interface{}{"foo": "bar"})
			},
		},
	} {
		t.Run(c.title, func(tt *testing.T) {
			s, fsroot := getTempFSStore2()
			defer cleanupRootIfOK(tt, fsroot)

			// Normal data
			k := Key{Kind: "Handler", Namespace: ns, Name: "default"}
			data := map[string]interface{}{"foo": "bar"}
			if err := write(fsroot, k, data); err != nil {
				tt.Fatalf("Failed to write: %v", err)
			}
			if err := c.prepare(fsroot); err != nil {
				tt.Fatalf("Failed to prepare precondition: %v", err)
			}
			if err := s.Init(context.Background(), []string{"Handler"}); err != nil {
				tt.Fatalf("Init failed: %v", err)
			}
			want := map[Key]map[string]interface{}{k: data}
			if lst := s.List(); len(lst) != 1 || !reflect.DeepEqual(lst, want) {
				tt.Errorf("Got %+v, Want %+v", lst, want)
			}
		})
	}
}
