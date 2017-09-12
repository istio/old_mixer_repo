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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	admissionV1alpha1 "k8s.io/api/admission/v1alpha1"
	registration "k8s.io/api/admissionregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/typed/core/v1/fake"
	k8stesting "k8s.io/client-go/testing"

	"istio.io/mixer/pkg/config/store"
)

type fakeValidator struct {
	err error
}

func (v *fakeValidator) Validate(t store.ChangeType, key store.Key, br *store.BackEndResource) error {
	return v.err
}

func sendRecv(v *ValidatorServer, in []byte, ctype string) []byte {
	req := httptest.NewRequest("", "/", bytes.NewBuffer(in))
	if ctype == "" {
		ctype = "application/json"
	}
	req.Header.Add("Content-Type", ctype)
	resp := httptest.NewRecorder()
	v.ServeHTTP(resp, req)
	return resp.Body.Bytes()
}

const spec = `{"spec":{
	"kind":{"group":"config.istio.io", "version":"v1alpha2", "kind":"Handler"},
	"name":"foo",
	"namespace":"bar",
	"operation":"%s",
	"object":%s
}}
`

const objectSpec = `{
	"kind": "Handler",
	"apiVersion": "config.istio.io/v1alpha2",
	"metadata": {
		"name": "foo",
		"namespace": "bar"
	},
	"spec":{"name":"foo", "adapter":"foo"}
}`

func TestValidation(t *testing.T) {
	for _, c := range []struct {
		title     string
		validator store.BackendValidator
		in        string
		targetNS  []string
		ctype     string
		ok        bool
	}{
		{
			"validator-success",
			&fakeValidator{},
			fmt.Sprintf(spec, "CREATE", objectSpec),
			nil,
			"",
			true,
		},
		{
			"validator-fail",
			&fakeValidator{errors.New("dummy")},
			fmt.Sprintf(spec, "DELETE", objectSpec),
			nil,
			"",
			false,
		},
		{
			"pass the resources which are outside of target namespaces",
			&fakeValidator{errors.New("dummy")},
			fmt.Sprintf(spec, "CREATE", objectSpec),
			[]string{"not-bar"},
			"",
			true,
		},
		{
			"illformed json",
			&fakeValidator{},
			`}`,
			nil,
			"",
			false,
		},
		{
			"unknown event type",
			&fakeValidator{},
			fmt.Sprintf(spec, "UNKNOWN", objectSpec),
			nil,
			"",
			false,
		},
		{
			"invalid content",
			&fakeValidator{},
			fmt.Sprintf(spec, "CREATE", `{"spec":{"name":"foo","adapter":"foo"}}`),
			nil,
			"",
			false,
		},
		{
			"unexpected content type",
			&fakeValidator{},
			fmt.Sprintf(spec, "CREATE", objectSpec),
			nil,
			"text/plain",
			false,
		},
	} {
		t.Run(c.title, func(tt *testing.T) {
			v := NewValidatorServer("", "ns", []string{"Handler"}, c.targetNS, nil, c.validator)
			result := sendRecv(v, []byte(c.in), c.ctype)
			review := &admissionV1alpha1.AdmissionReview{}
			if err := json.Unmarshal(result, review); err != nil {
				tt.Fatalf("Failed to unmarshal response: %v (got %s)", err, result)
			}
			if review.Status.Allowed != c.ok {
				tt.Errorf("Want %v, Got %+v", c.ok, review.Status)
			}
		})
	}
}

func testCertProvider(t *testing.T, cp CertProvider, add func(name string, data []byte)) {
	_, _, _, err := cp.Get()
	if err == nil {
		t.Errorf("Got nil, Want error")
	}
	add(keyFilename, []byte("key-data"))
	_, _, _, err = cp.Get()
	if err == nil {
		t.Errorf("Got nil, Want error")
	}
	add(certFilename, []byte("cert-data"))
	_, _, _, err = cp.Get()
	if err == nil {
		t.Errorf("Got nil, Want error")
	}
	add("unrelated.pem", []byte("unrelated-data"))
	_, _, _, err = cp.Get()
	if err == nil {
		t.Errorf("Got nil, Want error")
	}
	add(caFilename, []byte("ca-data"))
	key, cert, ca, err := cp.Get()
	if err != nil {
		t.Errorf("Got %v, Want nil", err)
	}
	if !bytes.Equal(key, []byte("key-data")) {
		t.Errorf("Got %s, Want key-data", key)
	}
	if !bytes.Equal(cert, []byte("cert-data")) {
		t.Errorf("Got %s, Want cert-data", key)
	}
	if !bytes.Equal(ca, []byte("ca-data")) {
		t.Errorf("Got %s, Want ca-data", key)
	}
}

func TestFileCertProvider(t *testing.T) {
	fsroot, err := ioutil.TempDir("/tmp/", "filecertprovider")
	if err != nil {
		t.Fatal(err)
	}
	fp := NewFileCertProvider(fsroot)
	testCertProvider(t, fp, func(name string, data []byte) {
		if err = ioutil.WriteFile(filepath.Join(fsroot, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	})
	if err = os.RemoveAll(fsroot); err != nil {
		t.Error(err)
	}
}

func TestSecretCertProvider(t *testing.T) {
	fakeObj := &k8stesting.Fake{}
	name := "test-name"
	secret := &corev1.Secret{Data: map[string][]byte{}}
	secret.Name = name
	w := watch.NewRaceFreeFake()
	fakeObj.AddWatchReactor("secrets", func(action k8stesting.Action) (bool, watch.Interface, error) {
		return true, w, nil
	})
	w.Add(secret)
	cp := NewSecretCertProvider((&fake.FakeCoreV1{Fake: fakeObj}).Secrets("testing"), name)
	testCertProvider(t, cp, func(name string, data []byte) {
		w.Reset()
		secret.Data[name] = data
		w.Modify(secret)
	})
}

func TestEnsureRegistration(t *testing.T) {
	client := &k8sfake.Clientset{Fake: k8stesting.Fake{}}
	fakeObj := &client.Fake
	vs := &ValidatorServer{
		ns:          "testing-ns",
		webhookName: "testing-name",
		names:       []string{"foos", "bars"},
		client:      client,
	}
	var created *registration.ExternalAdmissionHookConfiguration
	fakeObj.AddReactor("create", "externaladmissionhookconfigurations", func(action k8stesting.Action) (bool, runtime.Object, error) {
		var ok bool
		created, ok = action.(k8stesting.CreateAction).GetObject().(*registration.ExternalAdmissionHookConfiguration)
		if !ok {
			return false, nil, nil
		}
		return true, created, nil
	})
	if err := vs.ensureRegistration([]byte("cacert-data")); err != nil {
		t.Errorf("Got %v, Want nil", err)
	}
	want := &registration.ExternalAdmissionHookConfiguration{
		ExternalAdmissionHooks: []registration.ExternalAdmissionHook{
			{
				Name: "testing-name.testing-ns.mixer.istio.io",
				Rules: []registration.RuleWithOperations{{
					Operations: []registration.OperationType{
						registration.Create,
						registration.Update,
						registration.Delete},
					Rule: registration.Rule{
						APIGroups:   []string{apiGroup},
						APIVersions: []string{apiVersion},
						Resources:   []string{"foos", "bars"},
					},
				}},
				ClientConfig: registration.AdmissionHookClientConfig{
					Service: registration.ServiceReference{
						Namespace: "testing-ns",
						Name:      "testing-name",
					},
					CABundle: []byte("cacert-data"),
				},
			},
		},
	}
	want.Name = "testing-name.testing-ns.mixer.istio.io"

	if !reflect.DeepEqual(created, want) {
		t.Errorf("Got %+v\nWant %+v", created, want)
	}
}
