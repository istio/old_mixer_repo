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

package builtin

import (
	"io"
	"log"
	"os"
	"path"

	"github.com/pborman/uuid"

	"istio.io/mixer/pkg/adapter"
)

func constructAdapterInfos(adptBehaviors []AdapterBehavior) ([]adapter.InfoFn, []*Adapter) {
	var adapterInfos []adapter.InfoFn = make([]adapter.InfoFn, 0)
	spyAdapters := make([]*Adapter, 0)
	for _, b := range adptBehaviors {
		sa := NewSpyAdapter(b)
		spyAdapters = append(spyAdapters, sa)
		adapterInfos = append(adapterInfos, sa.GetAdptInfoFn())
	}
	return adapterInfos, spyAdapters
}

func closeHelper(c io.Closer) {
	err := c.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func buildConfigStore(srcDir string) (string, error) {
	currentPath, err := os.Getwd()
	if err != nil {
		return "", err
	}

	destDir := path.Join(currentPath, uuid.New())
	err = copyFolder(srcDir, destDir)
	if err != nil {
		return "", err
	}
	err = copyFile("rules.yaml", path.Join(destDir, "rules.yaml"))
	if err != nil {
		return "", err
	}
	return destDir, err
}

func copyFolder(source string, dest string) (err error) {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}

	err = os.MkdirAll(dest, info.Mode())
	if err != nil {
		return err
	}

	directory, _ := os.Open(source)
	objects, err := directory.Readdir(-1)

	for _, obj := range objects {
		src := source + "/" + obj.Name()
		destin := dest + "/" + obj.Name()

		if obj.IsDir() {
			err = copyFolder(src, destin)
			if err != nil {
				return err
			}
		} else {
			err = copyFile(src, destin)
			if err != nil {
				return err
			}
		}

	}
	return
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}

	defer closeHelper(in)
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer closeHelper(out)
	_, err = io.Copy(out, in)
	return err
}
