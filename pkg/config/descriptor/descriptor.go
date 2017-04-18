// Copyright 2017 the Istio Authors.
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

package descriptor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/ghodss/yaml"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/glog"

	dpb "istio.io/api/mixer/v1/config/descriptor"
	"istio.io/mixer/pkg/adapter"
	pb "istio.io/mixer/pkg/config/proto"
	"istio.io/mixer/pkg/expr"
)

// Finder describes anything that can provide a view into the config's descriptors by name and type.
type Finder interface {
	expr.AttributeDescriptorFinder

	// GetLog retrieves the log descriptor named `name`
	GetLog(name string) *dpb.LogEntryDescriptor

	// GetMetric retrieves the metric descriptor named `name`
	GetMetric(name string) *dpb.MetricDescriptor

	// GetMonitoredResource retrieves the monitored resource descriptor named `name`
	GetMonitoredResource(name string) *dpb.MonitoredResourceDescriptor

	// GetPrincipal retrieves the security principal descriptor named `name`
	GetPrincipal(name string) *dpb.PrincipalDescriptor

	// GetQuota retrieves the quota descriptor named `name`
	GetQuota(name string) *dpb.QuotaDescriptor
}

type finder struct {
	logs               map[string]*dpb.LogEntryDescriptor
	metrics            map[string]*dpb.MetricDescriptor
	monitoredResources map[string]*dpb.MonitoredResourceDescriptor
	principals         map[string]*dpb.PrincipalDescriptor
	quotas             map[string]*dpb.QuotaDescriptor
	attributes         map[string]*dpb.AttributeDescriptor
}

type newMsg func() (message proto.Message)

var typeMap = map[string]newMsg{
	"logs":               func() proto.Message { return &dpb.LogEntryDescriptor{} },
	"metrics":            func() proto.Message { return &dpb.MetricDescriptor{} },
	"monitoredResources": func() proto.Message { return &dpb.MonitoredResourceDescriptor{} },
	"principals":         func() proto.Message { return &dpb.PrincipalDescriptor{} },
	"quotas":             func() proto.Message { return &dpb.QuotaDescriptor{} },
	"manifests":          func() proto.Message { return &dpb.AttributeDescriptor{} },
}

// Parse parses a descriptor config in its parts.
func Parse(cfg string) (dcfg *pb.GlobalConfig, ce *adapter.ConfigErrors) {
	m := map[string]interface{}{}
	var err error

	if err = yaml.Unmarshal([]byte(cfg), &m); err != nil {
		return nil, ce.Append("DescriptorConfig", err)
	}
	dcfg = &pb.GlobalConfig{}
	var cerr *adapter.ConfigErrors
	var oarr interface{}

	//flatten manifest
	k := "manifests"
	val := m[k]
	if val != nil {
		mani := []*pb.AttributeManifest{}
		for _, msft := range val.([]interface{}) {
			manifest := msft.(map[string]interface{})
			attr := manifest["attributes"].([]interface{})
			if oarr, cerr = processArray(fmt.Sprintf("%s/%v", k, manifest["name"]), attr, typeMap[k]); cerr != nil {
				ce = ce.Extend(cerr)
				continue
			}

			mani = append(mani, &pb.AttributeManifest{
				Attributes: oarr.([]*dpb.AttributeDescriptor),
			})
		}
		dcfg.Manifests = mani
	}

	for k, kfn := range typeMap {
		if k == "manifests" {
			continue
		}
		val = m[k]
		if val == nil {
			if glog.V(2) {
				glog.Warningf("%s missing", k)
			}
			continue
		}

		if oarr, cerr = processArray(k, val.([]interface{}), kfn); cerr != nil {
			ce = ce.Extend(cerr)
			continue
		}
		switch k {
		case "logs":
			dcfg.Logs = oarr.([]*dpb.LogEntryDescriptor)
		case "metrics":
			dcfg.Metrics = oarr.([]*dpb.MetricDescriptor)
		case "quotas":
			dcfg.Quotas = oarr.([]*dpb.QuotaDescriptor)
		}
	}

	return
}

// processArray and return typed array
func processArray(name string, arr []interface{}, newMessage newMsg) (interface{}, *adapter.ConfigErrors) {
	var enc []byte
	var err error
	var ce *adapter.ConfigErrors
	nm := newMessage()
	ptrType := reflect.TypeOf(nm)
	valType := (reflect.Indirect(reflect.ValueOf(nm))).Type()
	outarr := reflect.MakeSlice(reflect.SliceOf(ptrType), 0, len(arr))
	for idx, attr := range arr {
		if enc, err = json.Marshal(attr); err != nil {
			ce = ce.Append(fmt.Sprintf("%s[%d]", name, idx), err)
			continue
		}
		dm := reflect.New(valType).Elem().Addr().Interface().(proto.Message)

		if err = jsonpb.Unmarshal(bytes.NewReader(enc), dm); err != nil {
			ce = ce.Append(fmt.Sprintf("%s[%d]", name, idx), fmt.
				Errorf("%s: %s", err.Error(), fmt.Sprintf(string(enc))))
			continue

		}
		outarr = reflect.Append(outarr, reflect.ValueOf(dm))
	}
	return outarr.Interface(), ce
}

// NewFinder constructs a new Finder for the provided global config.
func NewFinder(cfg *pb.GlobalConfig) Finder {
	f := &finder{
		logs:               make(map[string]*dpb.LogEntryDescriptor),
		metrics:            make(map[string]*dpb.MetricDescriptor),
		monitoredResources: make(map[string]*dpb.MonitoredResourceDescriptor),
		principals:         make(map[string]*dpb.PrincipalDescriptor),
		quotas:             make(map[string]*dpb.QuotaDescriptor),
		attributes:         make(map[string]*dpb.AttributeDescriptor),
	}

	if cfg == nil {
		return f
	}

	for _, desc := range cfg.Logs {
		f.logs[desc.Name] = desc
	}

	for _, desc := range cfg.Metrics {
		f.metrics[desc.Name] = desc
	}

	for _, desc := range cfg.MonitoredResources {
		f.monitoredResources[desc.Name] = desc
	}

	for _, desc := range cfg.Principals {
		f.principals[desc.Name] = desc
	}

	for _, desc := range cfg.Quotas {
		f.quotas[desc.Name] = desc
	}

	for _, manifest := range cfg.Manifests {
		for _, desc := range manifest.Attributes {
			f.attributes[desc.Name] = desc
		}
	}

	return f
}

func (d *finder) GetLog(name string) *dpb.LogEntryDescriptor {
	return d.logs[name]
}

func (d *finder) GetMetric(name string) *dpb.MetricDescriptor {
	return d.metrics[name]
}

func (d *finder) GetMonitoredResource(name string) *dpb.MonitoredResourceDescriptor {
	return d.monitoredResources[name]
}

func (d *finder) GetPrincipal(name string) *dpb.PrincipalDescriptor {
	return d.principals[name]
}

func (d *finder) GetQuota(name string) *dpb.QuotaDescriptor {
	return d.quotas[name]
}

func (d *finder) GetAttribute(name string) *dpb.AttributeDescriptor {
	return d.attributes[name]
}
