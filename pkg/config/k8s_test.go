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
	"errors"
	"strings"
	"testing"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
)

func k8sRF(bErr error, nErr error) *K8sResourceFetcher {
	return &K8sResourceFetcher{
		buildConfigFromFlags: func(masterUrl, kubeconfigPath string) (*rest.Config, error) {
			return nil, bErr
		},
		newForConfig: func(config *rest.Config) (*kubernetes.Clientset, error) {
			return nil, nErr
		},
	}
}

func TestNewK8sResourceFetcherErrors(t *testing.T) {
	_, err := NewK8sResourceFetcher("_DOES_NOT_EXIST_")
	if err == nil {
		t.Error("Expected failure")
	}

	ccErr := errors.New("client Creation Error")
	f := k8sRF(nil, ccErr)
	_, err = f.newK8sResourceFetcher("")
	if err != ccErr {
		t.Errorf("Got :%#v\nWant: %#v", err, ccErr)
	}

	f = k8sRF(nil, nil)
	_, err = f.newK8sResourceFetcher("")
	if err != nil {
		t.Errorf("Unexpected error %#v", err)
	}

}

func TestReadFile(t *testing.T) {
	f := &K8sResourceFetcher{}

	_, err := f.ReadFile("abc.txt")
	substr := "unknown url scheme"
	if err == nil || !strings.Contains(err.Error(), substr) {
		t.Errorf("Got: %s, Want: %#v", err.Error(), substr)
	}

	_, err = f.ReadFile(":abc.txt")
	substr = "missing protocol scheme"
	if err == nil || !strings.Contains(err.Error(), substr) {
		t.Errorf("Got: %s, Want: %#v", err.Error(), substr)
	}

	ccErr := errors.New("unable to read map")
	f = &K8sResourceFetcher{
		readMap: func(ns string, mapName string) (*v1.ConfigMap, error) {
			return nil, ccErr
		},
	}
	_, err = f.ReadFile(ConfigMapScheme + "://abc.txt")
	if err != ccErr {
		t.Errorf("Got: %#v\nWant: %#v", err, ccErr)
	}

	contents := "Got This File"
	f = &K8sResourceFetcher{
		readMap: func(ns string, mapName string) (*v1.ConfigMap, error) {
			return &v1.ConfigMap{
				Data: map[string]string{
					"c0": contents,
					"c1": contents,
				},
			}, nil
		},
	}

	b, err := f.ReadFile(ConfigMapScheme + "://abc.txt")
	if err != nil {
		t.Errorf("Unexpected error %#v", err)
	}

	if string(b[:]) != contents {
		t.Errorf("Got: %s\nWant: %s", string(b[:]), contents)
	}
}
