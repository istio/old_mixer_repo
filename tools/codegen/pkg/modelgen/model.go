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

package modelgen

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"

	tmpl "istio.io/mixer/tools/codegen/pkg/template_extension"
)

const FullNameOfExprMessage = "istio_mixer_v1_config_template.Expr"
const FullNameOfExprMessageWithPtr = "*" + FullNameOfExprMessage

type (
	// Data model to be used to generate code for istio/mixer
	Model struct {
		// top level fields
		Name        string
		PackageName string
		VarietyName string

		// imports
		Imports []string

		// Constructor fields
		ConstructorFields []fieldInfo

		// Warnings/Errors in the Template proto file.
		Diags []Diag
	}

	fieldInfo struct {
		Name string
		Type typeInfo
	}

	typeInfo struct {
		Name string
	}
)

// Create creates a Model object.
func Create(parser *FileDescriptorSetParser) (*Model, error) {
	var templateProto *FileDescriptor

	if template, diags := getTmplFileDesc(parser.allFiles); len(diags) != 0 {
		return nil, createError(diags)
	} else {
		templateProto = template
	}

	// set the current generated code package to the package of the
	// templateProto. This will make sure references within the
	// generated file into the template's pb.go file are fully qualified.
	parser.packageName = goPackageName(templateProto.GetPackage())

	model := &Model{Diags: make([]Diag, 0)}
	model.fillModel(templateProto, parser)
	if len(model.Diags) > 0 {
		return nil, createError(model.Diags)
	}

	return model, nil
}

func createError(diags []Diag) error {
	return fmt.Errorf("Errors during parsing:\n%s", stringifyDiags(diags))
}

func (model *Model) fillModel(templateProto *FileDescriptor, parser *FileDescriptorSetParser) {

	model.addTopLevelFields(templateProto)

	// ensure Type is present
	if _, ok := getRequiredMsg(templateProto, "Type"); !ok {
		model.addError(templateProto.GetName(), UNKNOWN_LINE, "message 'Type' not defined")
	}

	// ensure Constructor is present
	if cnstrDesc, ok := getRequiredMsg(templateProto, "Constructor"); !ok {
		model.addError(templateProto.GetName(), UNKNOWN_LINE, "message 'Constructor' not defined")
	} else {
		model.addInstanceFieldFromConstructor(parser, cnstrDesc)
		model.addImports(parser, templateProto, cnstrDesc)
	}
}

// Find the file that has the options TemplateVariety and TemplateName. There should only be one such file.
func getTmplFileDesc(fds []*FileDescriptor) (*FileDescriptor, []Diag) {
	var templateDescriptorProto *FileDescriptor = nil
	diags := make([]Diag, 0)
	for _, fd := range fds {
		if fd.GetOptions() == nil {
			continue
		}
		if !proto.HasExtension(fd.GetOptions(), tmpl.E_TemplateName) && !proto.HasExtension(fd.GetOptions(), tmpl.E_TemplateVariety) {
			continue
		}

		if !proto.HasExtension(fd.GetOptions(), tmpl.E_TemplateName) || !proto.HasExtension(fd.GetOptions(), tmpl.E_TemplateVariety) {
			diags = append(diags, createDiag(ERROR_DIAG, fd.GetName(), UNKNOWN_LINE, "Contains only one of the following two options %s and %s. Both options are required.",
				[]interface{}{tmpl.E_TemplateVariety.Name, tmpl.E_TemplateName.Name}))
			continue
		}

		if templateDescriptorProto != nil {
			diags = append(diags, createDiag(ERROR_DIAG, fd.GetName(), UNKNOWN_LINE, "Proto files %s and %s, both have the options %s and %s. Only one proto file is allowed with those options",
				[]interface{}{fd.GetName(), templateDescriptorProto.Name, tmpl.E_TemplateVariety.Name, tmpl.E_TemplateName.Name}))
			continue
		}

		templateDescriptorProto = fd
	}

	if templateDescriptorProto == nil {
		diags = append(diags, createDiag(ERROR_DIAG, UNKNOWN_FILE, UNKNOWN_LINE, "There has to be one proto file that has both extensions %s and %s",
			[]interface{}{tmpl.E_TemplateVariety.Name, tmpl.E_TemplateVariety.Name}))
	}

	if len(diags) != 0 {
		return nil, diags
	} else {
		return templateDescriptorProto, nil
	}
}

// Add all the file level fields to the model.
func (m *Model) addTopLevelFields(fd *FileDescriptor) {
	if !fd.proto3 {
		m.addError(fd.GetName(), UNKNOWN_LINE, "Only proto3 template files are allowed")
	}

	if fd.Package != nil {
		m.PackageName = goPackageName(strings.TrimSpace(*fd.Package))
	} else {
		m.addError(fd.GetName(), UNKNOWN_LINE, "package name missing")
	}

	if tmplName, err := proto.GetExtension(fd.GetOptions(), tmpl.E_TemplateName); err == nil {
		if _, ok := tmplName.(*string); !ok {
			// protoc should mandate the type. It is impossible to get to this state.
			m.addError(fd.GetName(), UNKNOWN_LINE, "%s should be of type string", tmpl.E_TemplateName.Name)
		} else {
			m.Name = *tmplName.(*string)
		}
	} else {
		// This func should only get called for FileDescriptor that has this attribute,
		// therefore it is impossible to get to this state.
		m.addError(fd.GetName(), UNKNOWN_LINE, "file option %s is required", tmpl.E_TemplateName.Name)
	}

	if tmplVariety, err := proto.GetExtension(fd.GetOptions(), tmpl.E_TemplateVariety); err == nil {
		m.VarietyName = (*(tmplVariety.(*tmpl.TemplateVariety))).String()
	} else {
		// This func should only get called for FileDescriptor that has this attribute,
		// therefore it is impossible to get to this state.
		m.addError(fd.GetName(), UNKNOWN_LINE, "file option %s is required", tmpl.E_TemplateVariety.Name)
	}
}

// Build field information about the Constructor message.
func (m *Model) addInstanceFieldFromConstructor(parser *FileDescriptorSetParser, cnstrDesc *Descriptor) {
	m.ConstructorFields = make([]fieldInfo, 0)
	for _, fieldDesc := range cnstrDesc.Field {
		fieldName := CamelCase(fieldDesc.GetName())
		// Name field is a reserved field that will be inject in the Instance object. The user defined
		// Constructor should not have a Name field, else there will be a name clash.
		// 'Name' within the Instance object would represent the name of the Constructor:instance_name
		// specified in the operator Yaml file.
		if fieldName == "Name" {
			// TODO add line number for the field.
			m.addError(cnstrDesc.file.GetName(), UNKNOWN_LINE, "Constructor message must not contain the reserved filed name '%s'", fieldDesc.GetName())
			continue
		}
		typename := parser.GoType(cnstrDesc.DescriptorProto, fieldDesc)
		// TODO : Can there be more than one expressions in a type for a field in Constructor ?
		typename = strings.Replace(typename, FullNameOfExprMessageWithPtr, "interface{}", 1)
		m.ConstructorFields = append(m.ConstructorFields, fieldInfo{Name: fieldName, Type: typeInfo{Name: typename}})
	}
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

func getRequiredMsg(fdp *FileDescriptor, msgName string) (*Descriptor, bool) {
	var cstrDesc *Descriptor = nil
	for _, desc := range fdp.desc {
		if desc.GetName() == msgName {
			cstrDesc = desc
			break
		}
	}
	if cstrDesc == nil {
		return nil, false
	}
	return cstrDesc, true
}

func getReferencedPackagesWithinType(parser *FileDescriptorSetParser, typDesc *Descriptor) []string {
	result := make([]string, 0)
	for _, fieldDesc := range typDesc.GetField() {
		getReferencedPackagesByField(parser, fieldDesc, typDesc.PackageName(), &result)
	}

	return result
}

func getReferencedPackagesByField(parser *FileDescriptorSetParser, fieldDesc *descriptor.FieldDescriptorProto, parentPackage string, referencedPkgs *[]string) {
	if *fieldDesc.Type != descriptor.FieldDescriptorProto_TYPE_MESSAGE && *fieldDesc.Type != descriptor.FieldDescriptorProto_TYPE_ENUM {
		return
	}
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
