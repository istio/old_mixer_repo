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

package crd

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	// import GKE cluster authentication plugin
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// import OIDC cluster authentication plugin, e.g. for Tectonic
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"istio.io/mixer/pkg/config/store"
)

// defaultDiscoveryBuilder builds the actual discovery client using the kubernetes config.
func defaultDiscoveryBuilder(conf *rest.Config) (discovery.DiscoveryInterface, error) {
	client, err := discovery.NewDiscoveryClientForConfig(conf)
	return client, err
}

// dynamicListenerWatcherBuilder is the builder of cache.ListerWatcher by using actual
// k8s.io/client-go/dynamic.Client.
type dynamicListerWatcherBuilder struct {
	client *dynamic.Client
}

func newDynamicListenerWatcherBuilder(conf *rest.Config) (listerWatcherBuilderInterface, error) {
	client, err := dynamic.NewClient(conf)
	if err != nil {
		return nil, err
	}
	return &dynamicListerWatcherBuilder{client}, nil
}

func (b *dynamicListerWatcherBuilder) build(res metav1.APIResource) cache.ListerWatcher {
	return b.client.Resource(&res, "")
}

// NewStore creates a new Store instance.
func NewStore(u *url.URL) (store.Store2Backend, error) {
	kubeconfig := u.Path
	namespaces := u.Query().Get("ns")
	retryTimeout := crdRetryTimeout
	retryTimeoutParam := u.Query().Get("retry-timeout")
	if retryTimeoutParam != "" {
		if timeout, err := time.ParseDuration(retryTimeoutParam); err == nil {
			retryTimeout = timeout
		} else {
			glog.Errorf("Failed to parse retry-timeout flag, using the default timeout %v: %v", crdRetryTimeout, err)
		}
	}
	conf, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	conf.APIPath = "/apis"
	conf.GroupVersion = &schema.GroupVersion{Group: apiGroup, Version: apiVersion}
	s := &Store{
		conf:                 conf,
		retryTimeout:         retryTimeout,
		discoveryBuilder:     defaultDiscoveryBuilder,
		listerWatcherBuilder: newDynamicListenerWatcherBuilder,
	}
	if len(namespaces) > 0 {
		s.ns = map[string]bool{}
		for _, n := range strings.Split(namespaces, ",") {
			s.ns[n] = true
		}
	}
	return s, nil
}

// Register registers this module as a Store2Backend.
// Do not use 'init()' for automatic registration; linker will drop
// the whole module because it looks unused.
func Register(builders map[string]store.Store2Builder) {
	builders["k8s"] = NewStore
	builders["kube"] = NewStore
	builders["kubernetes"] = NewStore
}

// Start starts the validation server.
func (v *ValidatorServer) Start(port uint16, certProvider CertProvider) error {
	key, cert, caCert, err := certProvider.Get()
	if err != nil {
		return err
	}
	sCert, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return err
	}
	// Unlike the webhook example code, this validation server does not take the kube-system's certificate and does not
	// require client cert, since istioctl (which is not a kube-system client) will take the role of the validation
	// if the external admission webhook isn't ready.
	cfg := &tls.Config{Certificates: []tls.Certificate{sCert}}
	server := &http.Server{
		Addr:      fmt.Sprintf(":%d", port),
		Handler:   v,
		TLSConfig: cfg,
	}
	// ensureRegistration can fail if external admission webhook is not yet ready.
	if err = v.ensureRegistration(caCert); err != nil {
		glog.V(3).Infof("Failed to register the validation server as the webhook: %v", err)
	}
	glog.Infof("server starting with port %d", port)
	return server.ListenAndServeTLS("", "")
}
