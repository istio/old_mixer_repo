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

package interfacegen

import (
	"io/ioutil"
	"os"
	"text/template"

	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"istio.io/mixer/tools/codegen/pkg/modelgen"
)

type Generator struct {
	OutFilePath   string
	ImportMapping map[string]string
}

func (g *Generator) Generate(fdsFile string) error {
	// This path works for bazel. TODO  Help pls !!

	tmplPath := "template/ProcInterface.tmpl"
	t, err := ioutil.ReadFile(tmplPath)
	if err != nil {
		panic(fmt.Errorf("cannot load template from path %s", tmplPath))
	}

	tmpl, err := template.New("ProcInterface").Parse(string(t))
	if err != nil {
		panic(err)
	}

	f, err := os.Create(g.OutFilePath)
	if err != nil {
		return err
	}

	fds, err := getFileDescSet(fdsFile)
	if err != nil {
		return err
	}
	parser, err := modelgen.CreateFileDescriptorSetParser(fds, g.ImportMapping)
	if err != nil {
		return err
	}

	model, err := modelgen.Create(parser)
	if err != nil {
		f.WriteString(err.Error())
		return err
	}

	return tmpl.Execute(f, model)
}

func getFileDescSet(path string) (*descriptor.FileDescriptorSet, error) {
	byts, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fds := &descriptor.FileDescriptorSet{}
	err = proto.Unmarshal(byts, fds)

	return fds, err
}
