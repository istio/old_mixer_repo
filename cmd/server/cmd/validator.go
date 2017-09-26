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

package cmd

import (
	"github.com/gogo/protobuf/proto"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"istio.io/mixer/cmd/shared"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/config"
	"istio.io/mixer/pkg/config/crd"
	"istio.io/mixer/pkg/config/store"
	"istio.io/mixer/pkg/runtime"
	"istio.io/mixer/pkg/template"
)

type validatorConfig struct {
	name             string
	namespace        string
	targetNamespaces []string
	resources        map[string]proto.Message
	port             uint16
	httpPort uint16
	secretName       string
	certsDir         string
}

func validatorCmd(info map[string]template.Info, adapters []adapter.InfoFn, printf, fatalf shared.FormatFn) *cobra.Command {
	vc := &validatorConfig{
		resources: runtime.KindMap(config.InventoryMap(adapters), info),
	}
	validatorCmd := &cobra.Command{
		Use:   "validator",
		Short: "Runs an https server for validations. Works as an external admission webhook for k8s",
		Run: func(cmd *cobra.Command, args []string) {
			runValidator(vc, printf, fatalf)
		},
	}
	validatorCmd.PersistentFlags().StringVar(&vc.namespace, "namespace", "default", "the namespace where this webhook is deployed")
	validatorCmd.PersistentFlags().StringVar(&vc.name, "webhook-name", "istio-mixer-webhook", "the name of the webhook")
	validatorCmd.PersistentFlags().StringArrayVar(&vc.targetNamespaces, "target-namespaces", []string{},
		"the list of namespaces where changes should be validated. Empty means to validate everything.")
	validatorCmd.PersistentFlags().Uint16VarP(&vc.port, "port", "p", 9099, "the port number of the server")
	validatorCmd.PersistentFlags().Uint16Var(&vc.httpPort, "http-port", 9199, "the port number to access to the server through plain http")
	validatorCmd.PersistentFlags().StringVar(&vc.certsDir, "certs", "/etc/certs", "the directory name where cert files are stored")
	validatorCmd.PersistentFlags().StringVar(&vc.secretName, "secret-name", "", "The name of k8s secret where the certificates are stored")
	return validatorCmd
}

func createK8sClient() (*kubernetes.Clientset, error) {
	// Validator needs to run within a cluster. It should work as long as InClusterConfig is valid.
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func createValidatorServer(vc *validatorConfig, client kubernetes.Interface) (*crd.ValidatorServer, error) {
	// TODO: bind this with pkg/runtime for referential integrity.
	v := store.NewValidator(nil, vc.resources)
	resourceNames := make([]string, 0, len(vc.resources))
	for name := range vc.resources {
		resourceNames = append(resourceNames, pluralize(name))
	}
	return crd.NewValidatorServer(vc.name, vc.namespace, resourceNames, vc.targetNamespaces, client, v), nil
}

func createCertProvider(vc *validatorConfig, client *kubernetes.Clientset) crd.CertProvider {
	if vc.secretName != "" {
		return crd.NewSecretCertProvider(client.CoreV1().Secrets(vc.namespace), vc.secretName)
	}
	return crd.NewFileCertProvider(vc.certsDir)
}

func runValidator(vc *validatorConfig, printf, fatalf shared.FormatFn) {
	client, err := createK8sClient()
	if err != nil {
		printf("Failed to create kubernetes client: %v", err)
		printf("Starting plain http server, but external admission hook is not enabled")
		client = nil
	}
	vs, err := createValidatorServer(vc, client)
	if err != nil {
		fatalf("Failed to create validator server: %v", err)
	}
	if client != nil {
		go func() {
			if err = vs.StartWebhook(vc.port, createCertProvider(vc, client)); err != nil {
				fatalf("Failed to start validator server: %v", err)
			}
		}()
	}
	if err = vs.StartHTTP(vc.httpPort); err != nil {
		fatalf("Failed to start plain http server: %v", err)
	}
}
