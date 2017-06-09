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

package model_generator

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	tmplExtns "istio.io/mixer/tools/codegen/template_extension"
	"github.com/golang/protobuf/proto"
)

/*
Data model to be used to generate code for istio/mixer
*/
type Model struct {
	// top level fields
	Name        string
	Check       bool
	PackageName string
	VarietyName string

	// imports
	Imports []string

	// Constructor fields
	ConstructorFields []FieldInfo
}

type FieldInfo struct {
	Name string
	Type TypeInfo
}

type TypeInfo struct {
	Name string
}

const FullNameOfExprMessage = "istio_mixer_v1_config_template.Expr"
const FullNameOfExprMessageWithPtr = "*" + FullNameOfExprMessage

// Creates a Model object.
func CreateModel(parser *FileDescriptorSetParser) (Model, error) {
	model := &Model{}

	templateProto, err := getTmplFileDesc(parser.allFiles)
	if err != nil {
		return *model, err
	}
	if !templateProto.proto3 {
		return *model, fmt.Errorf("Only proto3 template files are allowed")
	}
	// set the current generated code package to the package of the
	// templateProto. This will make sure references within the
	// generated file into the template's pb.go file are fully qualified.
	parser.packageName = goPackageName(templateProto.GetPackage())

	err = model.addTopLevelFields(templateProto)
	if err != nil {
		return *model, err
	}

	// ensure Constructor is present
	cnstrDesc, err := getRequiredMsg(templateProto, "Constructor")
	if err != nil {
		return *model, err
	}

	// ensure Constructor is present
	_, err = getRequiredMsg(templateProto, "Type")
	if err != nil {
		return *model, err
	}

	err = model.addInstanceFieldFromConstructor(parser, templateProto)
	if err != nil {
		return *model, err
	}

	// imports only referenced by the Constructor message.
	model.addImports(parser, templateProto, cnstrDesc)
	if err != nil {
		return *model, err
	}

	return *model, err
}

// Find the file that has the options TemplateVariety and TemplateName. There should only be one such file.
func getTmplFileDesc(fds []*FileDescriptor) (*FileDescriptor, error) {
	var templateDescriptorProto *FileDescriptor = nil

	for _, fd := range fds {
		if fd.GetOptions() == nil {
			continue
		}
		if !proto.HasExtension(fd.GetOptions(), tmplExtns.E_TemplateName) && !proto.HasExtension(fd.GetOptions(), tmplExtns.E_TemplateVariety) {
			continue
		} else if proto.HasExtension(fd.GetOptions(), tmplExtns.E_TemplateName) && proto.HasExtension(fd.GetOptions(), tmplExtns.E_TemplateVariety) {
			if templateDescriptorProto == nil {
				templateDescriptorProto = fd
			} else {
				return nil, fmt.Errorf("Proto files %s and %s, both have"+
					" the options %s and %s. Only one proto file is allowed with those options",
					fd.GetName(), templateDescriptorProto.Name,
					tmplExtns.E_TemplateVariety.Name, tmplExtns.E_TemplateName.Name)

			}
		} else {
			return nil, fmt.Errorf("Proto files %s has only one of the "+
				"following two options %s and %s. Both options are required.",
				fd.GetName(), tmplExtns.E_TemplateVariety.Name, tmplExtns.E_TemplateName.Name)
		}
	}
	if templateDescriptorProto == nil {
		return nil, fmt.Errorf("There has to be one proto file that has both extensions %s and %s",
			tmplExtns.E_TemplateVariety.Name, tmplExtns.E_TemplateVariety.Name)
	}
	return templateDescriptorProto, nil
}

// Add all the file level fields to the model.
func (m *Model) addTopLevelFields(fd *FileDescriptor) error {
	if fd.Package == nil {
		return fmt.Errorf("package name missing on file %s", fd.GetName())
	}
	m.PackageName = goPackageName(strings.TrimSpace(*fd.Package))

	tmplName, err := proto.GetExtension(fd.GetOptions(), tmplExtns.E_TemplateName)
	if err != nil {
		// This func should only get called for FileDescriptor that has this attribute,
		// therefore it is impossible to get to this state.
		return fmt.Errorf("file option %s is required", tmplExtns.E_TemplateName.Name)
	}
	if name, ok := tmplName.(*string); !ok {
		// protoc should mandate the type. It is impossible to get to this state.
		return fmt.Errorf("%s should be of type string", tmplExtns.E_TemplateName.Name)
	} else {
		m.Name = *name
	}

	tmplVariety, err := proto.GetExtension(fd.GetOptions(), tmplExtns.E_TemplateVariety)
	if err != nil {
		// This func should only get called for FileDescriptor that has this attribute,
		// therefore it is impossible to get to this state.
		return fmt.Errorf("file option %s is required", tmplExtns.E_TemplateVariety.Name)
	}
	if *(tmplVariety.(*tmplExtns.TemplateVariety)) == tmplExtns.TemplateVariety_TEMPLATE_VARIETY_CHECK {
		m.Check = true
		m.VarietyName = "Check"
	} else {
		m.Check = false
		m.VarietyName = "Report"
	}
	return nil
}

// Build field information about the Constructor message.
func (m *Model) addInstanceFieldFromConstructor(parser *FileDescriptorSetParser, fdp *FileDescriptor) error {
	m.ConstructorFields = make([]FieldInfo, 0)
	cstrDesc, err := getRequiredMsg(fdp, "Constructor")
	if err != nil {
		return err
	}
	for _, fieldDesc := range cstrDesc.Field {
		fieldName := CamelCase(fieldDesc.GetName())
		typename := parser.GoType(cstrDesc.DescriptorProto, fieldDesc)
		// TODO : Can there be more than one expressions in a type for a field in Constructor ?
		typename = strings.Replace(typename, FullNameOfExprMessageWithPtr, "interface{}", 1)

		m.ConstructorFields = append(m.ConstructorFields, FieldInfo{Name: fieldName, Type: TypeInfo{Name: typename}})
	}
	return nil
}

func (m *Model) addImports(parser *FileDescriptorSetParser, fileDescriptor *FileDescriptor, typDesc *Descriptor) {
	usedPackages := getReferencedPackagesWithinType(parser, typDesc)
	m.Imports = make([]string, 0)
	for _, s := range fileDescriptor.Dependency {
		fd := parser.fileByName(s)
		// Do not import our own package.
		if fd.PackageName() == parser.packageName {
			continue
		}
		filename := fd.goFileName()
		// By default, import path is the dirname of the Go filename.
		importPath := path.Dir(filename)
		if substitution, ok := parser.ImportMap[s]; ok {
			importPath = substitution
		}

		pname := goPackageName(fd.GetPackage())
		if !contains(usedPackages, pname) {
			pname = "_"
		}
		m.Imports = append(m.Imports, pname+" "+strconv.Quote(importPath))
	}
}

func getRequiredMsg(fdp *FileDescriptor, msgName string) (*Descriptor, error) {
	var cstrDesc *Descriptor = nil
	for _, desc := range fdp.desc {
		if desc.GetName() == msgName {
			cstrDesc = desc
			break
		}
	}
	if cstrDesc == nil {
		return nil, fmt.Errorf("%s should have a message '%s'", fdp.GetName(), msgName)
	}
	return cstrDesc, nil
}

func getReferencedPackagesWithinType(parser *FileDescriptorSetParser, typDesc *Descriptor) []string {
	result := make([]string, 0)
	for _, fieldDesc := range typDesc.GetField() {
		getReferencedPackagesByField(parser, fieldDesc, typDesc.PackageName(), &result)
	}

	return result
}

func getReferencedPackagesByField(parser *FileDescriptorSetParser, fieldDesc *descriptor.FieldDescriptorProto, parentPackage string, referencedPkgs *[]string) {
	if *fieldDesc.Type == descriptor.FieldDescriptorProto_TYPE_MESSAGE || *fieldDesc.Type == descriptor.FieldDescriptorProto_TYPE_ENUM {
		refDesc := parser.ObjectNamed(fieldDesc.GetTypeName())
		if d, ok := refDesc.(*Descriptor); ok {
			if fmt.Sprintf("%s.%s", d.PackageName(), d.GetName()) == FullNameOfExprMessage {
				return
			}
		}
		if d, ok := refDesc.(*Descriptor); ok && d.GetOptions().GetMapEntry() {
			keyField, valField := d.Field[0], d.Field[1]
			getReferencedPackagesByField(parser, keyField, parentPackage, referencedPkgs)
			getReferencedPackagesByField(parser, valField, parentPackage, referencedPkgs)
		} else {
			pname := goPackageName(parser.FileOf(refDesc.File()).GetPackage())
			if parentPackage != pname && !contains(*referencedPkgs, pname) {
				*referencedPkgs = append(*referencedPkgs, pname)
			}
		}
	}
}
