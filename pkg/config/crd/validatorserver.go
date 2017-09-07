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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"github.com/golang/glog"
	admissionV1alpha1 "k8s.io/api/admission/v1alpha1"
	registration "k8s.io/api/admissionregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	admission "k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	"istio.io/mixer/pkg/config/store"
)

const (
	keyFilename  = "server-key.pem"
	certFilename = "server-cert.pem"
	caFilename   = "ca-cert.pem"
)

const (
	webhookServiceName = "istio-mixer-webhook"
)

// CertProvider is the source of certificate data for running validator server.
type CertProvider interface {
	Get() (serverKey, serverCert, caCert []byte, err error)
}

type fileCertProvider struct {
	root string
}

func (fp *fileCertProvider) Get() ([]byte, []byte, []byte, error) {
	key, err := ioutil.ReadFile(filepath.Join(fp.root, keyFilename))
	if err != nil {
		return nil, nil, nil, err
	}
	cert, err := ioutil.ReadFile(filepath.Join(fp.root, certFilename))
	if err != nil {
		return nil, nil, nil, err
	}
	caCert, err := ioutil.ReadFile(filepath.Join(fp.root, caFilename))
	return key, cert, caCert, err
}

type secretCertProvider struct {
	client     corev1client.SecretInterface
	secretName string
}

func (sp *secretCertProvider) Get() ([]byte, []byte, []byte, error) {
	wch, err := sp.client.Watch(metav1.ListOptions{})
	if err != nil {
		return nil, nil, nil, err
	}
	defer wch.Stop()
	for ev := range wch.ResultChan() {
		obj, ok := ev.Object.(*corev1.Secret)
		if !ok {
			continue
		}
		if obj.Name == sp.secretName {
			key, ok := obj.Data[keyFilename]
			if !ok {
				return nil, nil, nil, fmt.Errorf("%s is missing in the secret %+v", keyFilename, obj)
			}
			cert, ok := obj.Data[certFilename]
			if !ok {
				return nil, nil, nil, fmt.Errorf("%s is missing in the secret %+v", certFilename, obj)
			}
			caCert, ok := obj.Data[caFilename]
			if !ok {
				return nil, nil, nil, fmt.Errorf("%s is missing in the secret %v", caFilename, obj)
			}
			return key, cert, caCert, nil
		}
	}
	return nil, nil, nil, errors.New("should not reach")
}

// NewFileCertProvider looks the certificate in the local filesystem.
func NewFileCertProvider(root string) CertProvider {
	return &fileCertProvider{root}
}

// NewSecretCertProvider looks the certificate through k8s secret, with the specified name.
func NewSecretCertProvider(client corev1client.SecretInterface, secretName string) CertProvider {
	return &secretCertProvider{client, secretName}
}

// ValidatorServer is an https server which triggers the validation of configs
// through external admission webhook.
type ValidatorServer struct {
	webhookName string
	ns          string
	names       []string
	targetNS    map[string]bool
	client      kubernetes.Interface
	validator   store.BackendValidator
}

// NewValidatorServer creates a new ValidatorServer.
func NewValidatorServer(
	name, namespace string,
	resources []string,
	targetNamespaces []string,
	client kubernetes.Interface,
	validator store.BackendValidator) *ValidatorServer {
	if name == "" {
		name = webhookServiceName
	}
	var targetNS map[string]bool
	if len(targetNamespaces) > 0 {
		targetNS = map[string]bool{}
		for _, ns := range targetNamespaces {
			targetNS[ns] = true
		}
	}
	return &ValidatorServer{
		webhookName: name,
		ns:          namespace,
		names:       resources,
		targetNS:    targetNS,
		client:      client,
		validator:   validator,
	}
}

func errorToAdmissionReviewStatus(err error) admissionV1alpha1.AdmissionReviewStatus {
	if err == nil {
		return admissionV1alpha1.AdmissionReviewStatus{
			Allowed: true,
			Result:  &metav1.Status{Status: "Success"},
		}
	}
	return admissionV1alpha1.AdmissionReviewStatus{
		Allowed: false,
		Result: &metav1.Status{
			Status:  "Failure",
			Message: err.Error(),
		},
	}
}

func (v *ValidatorServer) validate(data []byte) error {
	ar := &admissionV1alpha1.AdmissionReview{}
	if err := json.Unmarshal(data, ar); err != nil {
		return err
	}
	if v.targetNS != nil && !v.targetNS[ar.Spec.Namespace] {
		// not in the target. Ignoring.
		return nil
	}
	uns := &unstructured.Unstructured{}
	if err := json.Unmarshal(ar.Spec.Object.Raw, &uns); err != nil {
		return err
	}

	var ct store.ChangeType
	switch ar.Spec.Operation {
	case admission.Create, admission.Update:
		ct = store.Update
	case admission.Delete:
		ct = store.Delete
	default:
		return fmt.Errorf("operation %s is not supported", ar.Spec.Operation)
	}
	key := store.Key{
		Kind:      ar.Spec.Kind.Kind,
		Name:      ar.Spec.Name,
		Namespace: ar.Spec.Namespace,
	}
	spec, _ := uns.Object["spec"].(map[string]interface{})
	return v.validator.Validate(ct, key, spec)
}

func (v *ValidatorServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	glog.V(7).Infof("received: %s", body)

	var err error
	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType == "application/json" {
		err = v.validate(body)
	} else {
		err = fmt.Errorf("contentType=%s, expect application/json", contentType)
	}

	glog.V(7).Infof("response: %v", err)
	ar := admissionV1alpha1.AdmissionReview{
		Status: errorToAdmissionReviewStatus(err),
	}

	resp, err := json.Marshal(ar)
	if err != nil {
		glog.Error(err)
	}
	if _, err := w.Write(resp); err != nil {
		glog.Error(err)
	}
}

func (v *ValidatorServer) ensureRegistration(caCert []byte) error {
	webhookName := fmt.Sprintf("%s.%s.mixer.istio.io", v.webhookName, v.ns)
	client := v.client.AdmissionregistrationV1alpha1().ExternalAdmissionHookConfigurations()
	if _, err := client.Get(webhookName, metav1.GetOptions{}); err == nil {
		if err = client.Delete(webhookName, nil); err != nil {
			return err
		}
	}
	webhookConfig := &registration.ExternalAdmissionHookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: webhookName,
		},
		ExternalAdmissionHooks: []registration.ExternalAdmissionHook{
			{
				Name: webhookName,
				Rules: []registration.RuleWithOperations{{
					Operations: []registration.OperationType{
						registration.Create,
						registration.Update,
						registration.Delete},
					Rule: registration.Rule{
						APIGroups:   []string{apiGroup},
						APIVersions: []string{apiVersion},
						Resources:   v.names,
					},
				}},
				ClientConfig: registration.AdmissionHookClientConfig{
					Service: registration.ServiceReference{
						Namespace: v.ns,
						Name:      v.webhookName,
					},
					CABundle: caCert,
				},
			},
		},
	}
	_, err := client.Create(webhookConfig)
	return err
}
