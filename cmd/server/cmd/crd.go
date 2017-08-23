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
	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"istio.io/mixer/cmd/shared"
	pkgAdapter "istio.io/mixer/pkg/adapter"
	pkgadapter "istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/template"
)

func crdCmd(tmplInfos map[string]template.Info, adapters []pkgAdapter.InfoFn, printf shared.FormatFn) *cobra.Command {
	adapterCmd := cobra.Command{
		Use:   "crd",
		Short: "CRDs (CustomResourceDefinition) available in Mixer",
	}

	adapterCmd.AddCommand(&cobra.Command{
		Use:   "adapter",
		Short: "List CRDs for available adapters",
		Run: func(cmd *cobra.Command, args []string) {
			listCrdsAdapters(printf, adapters)
		},
	})

	adapterCmd.AddCommand(&cobra.Command{
		Use:   "instance",
		Short: "List CRDs for available instance kinds (mesh functions)",
		Run: func(cmd *cobra.Command, args []string) {
			listCrdsInstances(printf, tmplInfos)
		},
	})

	return &adapterCmd
}

func listCrdsAdapters(printf shared.FormatFn, infoFns []pkgadapter.InfoFn) {
	for _, infoFn := range infoFns {
		info := infoFn()
		printCrd(printf, info.Name /* TODO make this info.shortName when related PR is in. */, info.Name, "mixer-adapter")
	}
}

func listCrdsInstances(printf shared.FormatFn, tmplInfos map[string]template.Info) {
	for _, info := range tmplInfos {
		printCrd(printf, info.Name, info.HndlrInterfaceName, "mixer-instance")
	}
}

func printCrd(printf shared.FormatFn, shrtName string, implName string, istioLabel string) {
	crd := apiextensionsv1beta1.CustomResourceDefinition{
		TypeMeta: meta_v1.TypeMeta{
			Kind: "CustomResourceDefinition",
		},
		ObjectMeta: meta_v1.ObjectMeta{
			Name: shrtName + ".config.istio.io",
			Labels: map[string]string{
				"impl":  implName,
				"istio": istioLabel,
			},
		},
		Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
			Group:   "config.istio.io",
			Version: "v1alpha2",
			Scope:   apiextensionsv1beta1.NamespaceScoped,
			Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
				Plural:   shrtName + "s",
				Singular: shrtName,
				Kind:     shrtName,
			},
		},
	}
	out, err := yaml.Marshal(crd)
	if err != nil {
		printf("%s", err)
		return
	}
	printf(string(out))
	printf("---\n")
}
