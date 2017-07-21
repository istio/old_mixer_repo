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
	"bytes"
	"fmt"
	"go/format"
	"io/ioutil"
	"os"
	"text/template"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
	"golang.org/x/tools/imports"

	tmpl "istio.io/mixer/tools/codegen/pkg/interfacegen/template"
	"istio.io/mixer/tools/codegen/pkg/modelgen"
)

// Generator generates Go interfaces for adapters to implement for a given Template.
type Generator struct {
	OIntface string
	OTmpl    string
	ImptMap  map[string]string
}

// Generate creates a Go interfaces for adapters to implement for a given Template.
func (g *Generator) Generate(fdsFile string) error {

	intfaceTmpl, err := template.New("ProcInterface").Parse(tmpl.InterfaceTemplate)
	if err != nil {
		return fmt.Errorf("cannot load template: %v", err)
	}

	fds, err := getFileDescSet(fdsFile)
	if err != nil {
		return fmt.Errorf("cannot parse file '%s' as a FileDescriptorSetProto. %v", fdsFile, err)
	}

	parser, err := modelgen.CreateFileDescriptorSetParser(fds, g.ImptMap)
	if err != nil {
		return fmt.Errorf("cannot parse file '%s' as a FileDescriptorSetProto. %v", fdsFile, err)
	}

	model, err := modelgen.Create(parser)
	if err != nil {
		return err
	}

	intfaceBuf := new(bytes.Buffer)
	err = intfaceTmpl.Execute(intfaceBuf, model)
	if err != nil {
		return fmt.Errorf("cannot execute the template with the given data: %v", err)
	}

	fmtd, err := format.Source(intfaceBuf.Bytes())
	if err != nil {
		return fmt.Errorf("could not format generated code: %v", err)
	}

	imports.LocalPrefix = "istio.io"
	// OutFilePath provides context for import path. We rely on the supplied bytes for content.
	imptd, err := imports.Process(g.OIntface, fmtd, nil)
	if err != nil {
		return fmt.Errorf("could not fix imports for generated code: %v", err)
	}

	revisedTemplateTmpl, err := template.New("RevisedTemplateTmpl").Parse(tmpl.RevisedTemplateTmpl)
	if err != nil {
		return fmt.Errorf("cannot load template: %v", err)
	}

	tmplBuf := new(bytes.Buffer)
	err = revisedTemplateTmpl.Execute(tmplBuf, model)
	if err != nil {
		return fmt.Errorf("cannot execute the template with the given data: %v", err)
	}

	// Everything succeeded, now write to the file.
	if f, err := os.Create(g.OIntface); err != nil {
		return err
	} else if _, err = f.Write(imptd); err != nil {
		return err
	} else {
		// file successfully written, close it.
		f.Close()
	}

	if f, err := os.Create(g.OTmpl); err != nil {
		return err
	} else if _, err = f.Write(tmplBuf.Bytes()); err != nil {
		return err
	} else {
		// file successfully written, close it.
		return f.Close()
	}
}

func getFileDescSet(path string) (*descriptor.FileDescriptorSet, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fds := &descriptor.FileDescriptorSet{}
	err = proto.Unmarshal(bytes, fds)
	if err != nil {
		return nil, err
	}

	return fds, nil
}
