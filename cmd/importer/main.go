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

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/golang/glog"

	"istio.io/mixer/pkg/config"
	"istio.io/mixer/pkg/version"
)

var (
	source      = flag.String("source", "", "the location URL of the config source")
	destination = flag.String("destination", "", "the location URL of the destination source")
	vflag       = flag.Bool("version", false, "show the version")
)

func importerUsage() {
	fmt.Fprintf(os.Stderr, "importer is a utility to import config data into a backend from another one.\n"+
		"This is used for setting up a new backend database with default data.\n"+
		"\n\nUsage:\n\timporter --source=<source> --destination=<destination>\n\nFlags:\n")
	flag.PrintDefaults()
}

func main() {
	flag.Usage = importerUsage
	flag.Parse()

	if *vflag {
		fmt.Printf("%s", version.Info)
		return
	}

	if *source == "" || *destination == "" {
		importerUsage()
		return
	}

	ss, err := config.NewStore(*source)
	if err != nil {
		glog.Errorf("can't access to source %s: %v", *source, err)
		return
	}
	defer ss.Close()

	ds, err := config.NewStore(*destination)
	if err != nil {
		glog.Errorf("can't access to destination %s: %v", *destination, err)
		return
	}
	defer ds.Close()

	skeys, _, err := ss.List("/", true)
	if err != nil {
		glog.Errorf("can't take the list of keys from %s: %v", *source, err)
		return
	}
	for _, key := range skeys {
		if glog.V(4) {
			glog.Info("copying", key)
		}
		val, _, found := ss.Get(key)
		if !found {
			glog.Error("failed to get", key)
			continue
		}
		_, err = ds.Set(key, val)
		if err != nil {
			glog.Error("failed to set", key)
		}
	}
}
