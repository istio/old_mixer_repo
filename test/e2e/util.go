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
	"testing"
	"istio.io/mixer/pkg/adapter"
	"os"
	"path"
	"istio.io/mixer/pkg/attribute"
	"reflect"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"istio.io/api/mixer/v1"
)

func getCnfgs(srvcCnfg, attrCnfg string) (dir string) {
	tmpDir := path.Join(os.TempDir(), "e2eStoreDir")
	os.MkdirAll(tmpDir, os.ModePerm)

	srvcCnfgFile, _ := os.Create(path.Join(tmpDir, "srvc.yaml"))
	globalCnfgFile, _ := os.Create(path.Join(tmpDir, "global.yaml"))

	_, _ = globalCnfgFile.Write([]byte(attrCnfg))
	_, _ = srvcCnfgFile.Write([]byte(srvcCnfg))

	_ = globalCnfgFile.Close()
	_ = srvcCnfgFile.Close()

	return tmpDir
}

// return adapterInfoFns + corresponding SkyAdapter object.
func cnstrAdapterInfos(adptBehaviors []adptBehavior) ([]adapter.InfoFn, []*spyAdapter) {
	var adapterInfos []adapter.InfoFn = make([]adapter.InfoFn, 0)
	var spyAdapters []*spyAdapter = make([]*spyAdapter, 0)
	for _, b := range adptBehaviors {
		sa := newSpyAdapter(b)
		spyAdapters = append(spyAdapters, sa)
		adapterInfos = append(adapterInfos, sa.getAdptInfoFn())
	}
	return adapterInfos, spyAdapters
}

func getAttrBag(attribs map[string]interface{}) istio_mixer_v1.Attributes {
	requestBag := attribute.GetMutableBag(nil)
	requestBag.Set(configIdentityAttribute, identityDomainAttribute)
	for k, v := range attribs {
		requestBag.Set(k, v)
	}

	var attrs istio_mixer_v1.Attributes
	requestBag.ToProto(&attrs, nil, 0)
	return attrs
}

func cmpSliceAndErr(msg string, t *testing.T, act, exp interface{}) {
	a := InterfaceSlice(exp)
	b := InterfaceSlice(act)
	if len(a) != len(b) {
		t.Errorf(fmt.Sprintf("Not equal -> %s.\nActual :\n%s\n\nExpected :\n%s",msg, spew.Sdump(act), spew.Sdump(exp)))
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
			t.Errorf(fmt.Sprintf("Not equal -> %s.\nActual :\n%s\n\nExpected :\n%s",msg, spew.Sdump(act), spew.Sdump(exp)))
			return
		}
	}
	return
}


func cmpMapAndErr(msg string, t *testing.T, act, exp interface{}) {
	want := InterfaceMap(exp)
	got := InterfaceMap(act)
	if len(want) != len(got) {
		t.Errorf(fmt.Sprintf("Not equal -> %s.\nActual :\n%s\n\nExpected :\n%s",msg, spew.Sdump(act), spew.Sdump(exp)))
		return
	}

	for wk, wv := range want {
		if v, found := got[wk]; !found {
			t.Errorf(fmt.Sprintf("Not equal -> %s.\nActual :\n%s\n\nExpected :\n%s",msg, spew.Sdump(act), spew.Sdump(exp)))
			return
		} else {
			if !reflect.DeepEqual(wv, v) {
				t.Errorf(fmt.Sprintf("Not equal -> %s.\nActual :\n%s\n\nExpected :\n%s",msg, spew.Sdump(act), spew.Sdump(exp)))
				return
			}
		}
	}
	return
}


func InterfaceSlice(slice interface{}) []interface{} {
	s := reflect.ValueOf(slice)

	ret := make([]interface{}, s.Len())
	for i := 0; i < s.Len(); i++ {
		ret[i] = s.Index(i).Interface()
	}

	return ret
}


func InterfaceMap(m interface{}) map[interface{}]interface{} {
	s := reflect.ValueOf(m)

	ret := make(map[interface{}]interface{}, s.Len())
	for i := 0; i < s.Len(); i++ {
		k := s.MapKeys()[i]
		v := s.MapIndex(k)
		ret[k.Interface()] = v.Interface()
	}

	return ret
}