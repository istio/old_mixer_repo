// Copyright 2017 Google Inc.
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
	"net/url"
	"os"
	"strings"

	"github.com/golang/glog"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	meta_v1 "k8s.io/client-go/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ConfigMapScheme defines url scheme for a config map.
const ConfigMapScheme = "configmap"

type (
	readMap              func(ns string, mapName string) (*v1.ConfigMap, error)
	buildConfigFromFlags func(masterUrl, kubeconfigPath string) (*rest.Config, error)
	newForConfig         func(config *rest.Config) (*kubernetes.Clientset, error)

	// K8sResourceFetcher provides ability to read from configMapUrl.
	K8sResourceFetcher struct {
		readMap              readMap
		buildConfigFromFlags buildConfigFromFlags
		newForConfig         newForConfig
	}
)

// NewK8sResourceFetcher constructs a kubernetes resource fetcher.
// if kubecofig is omitted, in-cluster or local config is used appropriately.
func NewK8sResourceFetcher(kubeconfig string) (*K8sResourceFetcher, error) {
	f := &K8sResourceFetcher{
		buildConfigFromFlags: clientcmd.BuildConfigFromFlags,
		newForConfig:         kubernetes.NewForConfig,
	}
	return f.newK8sResourceFetcher(kubeconfig)
}

func (f *K8sResourceFetcher) newK8sResourceFetcher(kubeconfig string) (*K8sResourceFetcher, error) {
	// if not running in k8s, use standard local config.
	if os.Getenv("KUBERNETES_SERVICE_HOST") == "" && kubeconfig == "" {
		kubeconfig = os.Getenv("HOME") + "/.kube/config"
	}
	config, err := f.buildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	k8sClient, err := f.newForConfig(config)
	if err != nil {
		return nil, err
	}

	f.readMap = func(ns string, mapName string) (*v1.ConfigMap, error) {
		return k8sClient.ConfigMaps(ns).Get(mapName, meta_v1.GetOptions{})
	}

	return f, nil
}

// ReadFile reads the specified configmap resource
func (f *K8sResourceFetcher) ReadFile(configMapURL string) ([]byte, error) {
	u, err := url.Parse(configMapURL)
	if err != nil {
		return nil, err
	}

	if u.Scheme != ConfigMapScheme {
		return nil, fmt.Errorf("unknown url scheme %s", u.Scheme)
	}

	mapname := strings.TrimLeft(u.Path, "/")

	cfg, err := f.readMap(u.Host, mapname)
	if err != nil {
		return nil, err
	}
	if len(cfg.Data) > 1 {
		glog.Warningf("map has multiple configs")
	}
	var data []byte
	for _, v := range cfg.Data {
		data = []byte(v)
		break
	}
	return data, nil
}
