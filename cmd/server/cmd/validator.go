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
	"fmt"
	"net/http"

	"github.com/gogo/protobuf/proto"
	"github.com/spf13/cobra"

	"istio.io/mixer/cmd/shared"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/config"
	"istio.io/mixer/pkg/config/store"
	"istio.io/mixer/pkg/runtime"
	"istio.io/mixer/pkg/template"
)

type validatorConfig struct {
	targetNamespaces []string
	resources        map[string]proto.Message
	port             uint16
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
	validatorCmd.PersistentFlags().StringArrayVar(&vc.targetNamespaces, "target-namespaces", []string{},
		"the list of namespaces where changes should be validated. Empty means to validate everything.")
	validatorCmd.PersistentFlags().Uint16VarP(&vc.port, "port", "p", 9099, "the port number of the server")
	return validatorCmd
}

func createValidatorServer(vc *validatorConfig) (*store.ValidatorServer, error) {
	// TODO: bind this with pkg/runtime for referential integrity.
	return store.NewValidatorServer(vc.targetNamespaces, vc.resources, nil), nil
}

func runValidator(vc *validatorConfig, printf, fatalf shared.FormatFn) {
	vs, err := createValidatorServer(vc)
	if err != nil {
		fatalf("Failed to create validator server: %v", err)
	}
	printf("Starting the validator server on port %d", vc.port)
	if err = http.ListenAndServe(fmt.Sprintf(":%d", vc.port), vs); err != nil {
		fatalf("Failed to start the validator server: %v", err)
	}
}
