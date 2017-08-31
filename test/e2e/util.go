// Copyright 2016 Istio Authors
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

package e2e

import (
	"fmt"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"

	"istio.io/api/mixer/v1"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/attribute"
)

// GetCnfgs takes the operator configuration as strings and creates directory with config files from it.
func GetCnfgs(srvcCnfg, attrCnfg string) (dir string) {
	tmpDir := path.Join(os.TempDir(), "e2eStoreDir")
	_ = os.MkdirAll(tmpDir, os.ModePerm)

	srvcCnfgFile, _ := os.Create(path.Join(tmpDir, "srvc.yaml"))
	globalCnfgFile, _ := os.Create(path.Join(tmpDir, "global.yaml"))

	_, _ = globalCnfgFile.Write([]byte(attrCnfg))
	_, _ = srvcCnfgFile.Write([]byte(srvcCnfg))

	_ = globalCnfgFile.Close()
	_ = srvcCnfgFile.Close()

	return tmpDir
}

// CnstrAdapterInfos constructs spyAdapters for each of the adptBehavior. It returns
// the constructed spyAdapters along with the adapters Info functions.
func CnstrAdapterInfos(adptBehaviors []adptBehavior) ([]adapter.InfoFn, []*SpyAdapter) {
	var adapterInfos []adapter.InfoFn = make([]adapter.InfoFn, 0)
	spyAdapters := make([]*SpyAdapter, 0)
	for _, b := range adptBehaviors {
		sa := newSpyAdapter(b)
		spyAdapters = append(spyAdapters, sa)
		adapterInfos = append(adapterInfos, sa.getAdptInfoFn())
	}
	return adapterInfos, spyAdapters
}

// GetAttributes returns mixer Attributes
func GetAttributes(attribs map[string]interface{}) istio_mixer_v1.Attributes {
	requestBag := attribute.GetMutableBag(nil)
	requestBag.Set(configIdentityAttribute, identityDomainAttribute)
	for k, v := range attribs {
		requestBag.Set(k, v)
	}

	var attrs istio_mixer_v1.Attributes
	requestBag.ToProto(&attrs, nil, 0)
	return attrs
}

// CmpSliceAndErr compares two slices
func CmpSliceAndErr(msg string, t *testing.T, act, exp interface{}) {
	a := interfaceSlice(exp)
	b := interfaceSlice(act)
	if len(a) != len(b) {
		t.Errorf(fmt.Sprintf("Not equal -> %s.\nActual :\n%s\n\nExpected :\n%s", msg, spew.Sdump(act), spew.Sdump(exp)))
		return
	}

	for _, x1 := range a {
		f := false
		for _, x2 := range b {
			if reflect.DeepEqual(x1, x2) {
				f = true
			}
		}
		if !f {
			t.Errorf(fmt.Sprintf("Not equal -> %s.\nActual :\n%s\n\nExpected :\n%s", msg, spew.Sdump(act), spew.Sdump(exp)))
			return
		}
	}
	return
}

// CmpMapAndErr compares two maps
func CmpMapAndErr(msg string, t *testing.T, act, exp interface{}) {
	want := interfaceMap(exp)
	got := interfaceMap(act)
	if len(want) != len(got) {
		t.Errorf(fmt.Sprintf("Not equal -> %s.\nActual :\n%s\n\nExpected :\n%s", msg, spew.Sdump(act), spew.Sdump(exp)))
		return
	}

	for wk, wv := range want {
		if v, found := got[wk]; !found {
			t.Errorf(fmt.Sprintf("Not equal -> %s.\nActual :\n%s\n\nExpected :\n%s", msg, spew.Sdump(act), spew.Sdump(exp)))
		} else {
			if !reflect.DeepEqual(wv, v) {
				t.Errorf(fmt.Sprintf("Not equal -> %s.\nActual :\n%s\n\nExpected :\n%s", msg, spew.Sdump(act), spew.Sdump(exp)))
			}
		}
	}
	return
}

func interfaceSlice(slice interface{}) []interface{} {
	s := reflect.ValueOf(slice)

	ret := make([]interface{}, s.Len())
	for i := 0; i < s.Len(); i++ {
		ret[i] = s.Index(i).Interface()
	}

	return ret
}

func interfaceMap(m interface{}) map[interface{}]interface{} {
	s := reflect.ValueOf(m)

	ret := make(map[interface{}]interface{}, s.Len())
	for i := 0; i < s.Len(); i++ {
		k := s.MapKeys()[i]
		v := s.MapIndex(k)
		ret[k.Interface()] = v.Interface()
	}

	return ret
}
