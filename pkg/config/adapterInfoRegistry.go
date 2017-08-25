// Copyright 2017 Istio Authors.
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

package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/golang/glog"

	"istio.io/mixer/pkg/adapter"
)

type adapterInfoRegistry struct {
	adapterInfosByName map[string]*adapter.BuilderInfo
}

// DNS1035Str defines regex to test for DNS-1035 compatible names.
const DNS1035Str = "^[a-z]([-a-z0-9]*[a-z0-9])?$"

// DNS1035 is the prebuilt matcher for DNS-1035 compatible names.
var DNS1035 = regexp.MustCompile(DNS1035Str)

func dns1035Err(name string) error {
	return fmt.Errorf("%s cannot be converted to a DNS-1035 compatible name %s", name, DNS1035Str)
}

// NameToCfgKind returns Kind name given an adapter name.
// istio.io/mixer/adapter/denier --> denier-mixer-istio-io
// github.com/user1/mixerAdapters/denier --> denier-mixeradapters-user1
// metricsco.io/mixerAdapters/metrics1 --> metrics1-mixeradapters-metricso-io
// metricsco.io/mixerAdapters/adapters/metrics2 --> metrics2-mixeradapters-metricsco-io
// kind = $pkg-$reponame-$org
// Resulting name is DNS-1035 compatible. [a-z]([-a-z0-9]*[a-z0-9])?
func NameToCfgKind(adapterName string) (string, error) {
	adapterName = strings.ToLower(adapterName)
	adapterName = strings.Replace(adapterName, ".", "-", -1)
	const github = "github-com" // since . is replaced by - above.

	// if name is already compatible just use it.
	if DNS1035.MatchString(adapterName) {
		return adapterName, nil
	}

	comps := strings.Split(adapterName, "/")
	if len(comps) < 3 || (comps[0] == github && len(comps) < 4) {
		return "", dns1035Err(adapterName)
	}

	pkg := comps[len(comps)-1]

	// username is the org if repo starts with github
	cidx := 0
	if comps[0] == github {
		cidx++
	}
	org := comps[cidx]
	reponame := comps[cidx+1]

	name := pkg + "-" + reponame + "-" + org
	if !DNS1035.MatchString(name) {
		return "", dns1035Err(adapterName)
	}

	return name, nil
}

type handlerBuilderValidator func(hndlrBuilder adapter.HandlerBuilder, t string) (bool, string)

// newRegistry2 returns a new adapterInfoRegistry or error.
func newRegistry2Internal(infos []adapter.InfoFn, hndlrBldrValidator handlerBuilderValidator) (*adapterInfoRegistry, error) {
	r := &adapterInfoRegistry{make(map[string]*adapter.BuilderInfo)}
	for idx, info := range infos {
		glog.V(3).Infof("Registering [%d] %#v", idx, info)
		adptInfo := info()

		if len(adptInfo.ShortName) == 0 {
			var err error
			if adptInfo.ShortName, err = NameToCfgKind(adptInfo.Name); err != nil {
				return nil, err
			}
		}

		if a, ok := r.adapterInfosByName[adptInfo.ShortName]; ok {
			// error only if 2 different adapter.Info objects are trying to identify by the
			// same Name.
			return nil, fmt.Errorf("duplicate registration for '%s' : old = %v new = %v", a.Name, adptInfo, a)
		}
		if adptInfo.ValidateConfig == nil {
			// error if adapter has not provided the ValidateConfig func.
			return nil, fmt.Errorf("Adapter info %v from adapter %s does not contain value for ValidateConfig"+
				" function field.", adptInfo, adptInfo.Name)
		}
		if adptInfo.DefaultConfig == nil {
			// error if adapter has not provided the DefaultConfig func.
			return nil, fmt.Errorf("Adapter info %v from adapter %s does not contain value for DefaultConfig "+
				"field.", adptInfo, adptInfo.Name)
		}
		if ok, errMsg := doesBuilderSupportsTemplates(adptInfo, hndlrBldrValidator); !ok {
			// error if an Adapter's HandlerBuilder does not implement interfaces that it says it wants to support.
			return nil, fmt.Errorf("HandlerBuilder from adapter %s does not implement the required interfaces"+
				" for the templates it supports: %s", adptInfo.Name, errMsg)
		}
		r.adapterInfosByName[adptInfo.ShortName] = &adptInfo
	}
	return r, nil
}

// newRegistry2 returns a new adapterInfoRegistry.
func newRegistry2(infos []adapter.InfoFn, hndlrBldrValidator handlerBuilderValidator) (r *adapterInfoRegistry) {
	var err error
	if r, err = newRegistry2Internal(infos, hndlrBldrValidator); err != nil {
		panic(err)
	}
	return
}

// AdapterInfoMap returns the known adapter.Infos, indexed by their names.
func AdapterInfoMap(handlerRegFns []adapter.InfoFn,
	hndlrBldrValidator handlerBuilderValidator) map[string]*adapter.BuilderInfo {
	return newRegistry2(handlerRegFns, hndlrBldrValidator).adapterInfosByName
}

// FindAdapterInfo returns the adapter.Info object with the given name.
func (r *adapterInfoRegistry) FindAdapterInfo(name string) (b *adapter.BuilderInfo, found bool) {
	bi, found := r.adapterInfosByName[name]
	if !found {
		return nil, false
	}
	return bi, true
}

func doesBuilderSupportsTemplates(info adapter.BuilderInfo, hndlrBldrValidator handlerBuilderValidator) (bool, string) {
	handlerBuilder := info.CreateHandlerBuilder()
	resultMsgs := make([]string, 0)
	for _, t := range info.SupportedTemplates {
		if ok, errMsg := hndlrBldrValidator(handlerBuilder, t); !ok {
			resultMsgs = append(resultMsgs, errMsg)
		}
	}
	if len(resultMsgs) != 0 {
		return false, strings.Join(resultMsgs, "\n")
	}
	return true, ""
}
