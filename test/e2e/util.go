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
	"os"
	"path"
	"reflect"
)

// GetCfgs takes the operator configuration as strings and creates directory with config files from it.
func GetCfgs(srvcCnfg, attrCnfg string) (dir string) {
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

// CmpSliceAndErr compares two slices
func CmpSliceAndErr(act, exp interface{}) bool {
	a := interfaceSlice(exp)
	b := interfaceSlice(act)
	if len(a) != len(b) {
		return false
	}

	for _, x1 := range a {
		f := false
		for _, x2 := range b {
			if reflect.DeepEqual(x1, x2) {
				f = true
			}
		}
		if !f {
			return false
		}
	}
	return true
}

// CmpMapAndErr compares two maps
func CmpMapAndErr(act, exp interface{}) bool {
	want := interfaceMap(exp)
	got := interfaceMap(act)
	if len(want) != len(got) {
		return false
	}

	for wk, wv := range want {
		if v, found := got[wk]; !found {
			return false
		} else {
			if !reflect.DeepEqual(wv, v) {
				return false
			}
		}
	}
	return true
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
