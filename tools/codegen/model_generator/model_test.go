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
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"

	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
)

func TestMissingPackageName(t *testing.T) {
	testError(t,
		"testdata/MissingPackageName.proto",
		"package name missing")
}

func TestMissingTemplateNameExt(t *testing.T) {
	testError(t,
		"testdata/MissingTemplateNameExt.proto",
		"has only one of the following two options")
}

func TestMissingTemplateVarietyExt(t *testing.T) {
	testError(t,
		"testdata/MissingTemplateVarietyExt.proto",
		"has only one of the following two options")
}

func TestMissingBothRequriedExt(t *testing.T) {
	testError(t,
		"testdata/MissingBothRequiredExt.proto",
		"one proto file that has both extensions")
}

func TestTypeMessage(t *testing.T) {
	testError(t,
		"testdata/MissingTypeMessage.proto",
		"should have a message 'Type'")
}

func TestConstructorMessage(t *testing.T) {
	testError(t,
		"testdata/MissingConstructorMessage.proto",
		"should have a message 'Constructor'")
}

func TestBasicTopLevelFields(t *testing.T) {
	testFilename := "testdata/BasicTopLevelFields.proto"
	model, _ := createTestModel(t,
		testFilename)
	if model.PackageName != "foo_bar" {
		t.Fatalf("CreateModel(%s).PackageName = %v, wanted %s", testFilename, model.PackageName, "foo_bar")
	}
	if model.Name != "List" {
		t.Fatalf("CreateModel(%s).Name = %v, wanted %s", testFilename, model.Name, "List")
	}
	if model.VarietyName != "Check" {
		t.Fatalf("CreateModel(%s).VarietyName = %v, wanted %s", testFilename, model.VarietyName, "Check")
	}
	if model.Check != true {
		t.Fatalf("CreateModel(%s).Check = %v, wanted %s", testFilename, model.Check, "true")
	}
}

func TestConstructorDirRefAndImports(t *testing.T) {
	testFilename := "testdata/ConstructorAndImports.proto"
	model, _ := createTestModel(t,
		testFilename)

	expectedTxt := "istio_mixer_v1_config_descriptor \"mixer/v1/config/descriptor\""
	if !contains(model.Imports, expectedTxt) {
		t.Fatalf("CreateModel(%s).Imports = %v, wanted to contain %s", testFilename, model.Imports, expectedTxt)
	}
	expectedTxt = "_ \"mixer/tools/codegen/template_extension\""
	if !contains(model.Imports, expectedTxt) {
		t.Fatalf("CreateModel(%s).Imports = %v, wanted to contain %s", testFilename, model.Imports, expectedTxt)
	}

	if len(model.ConstructorFields) != 5 {
		t.Fatalf("len(CreateModel(%s).ConstructorFields) = %v, wanted %d", testFilename, len(model.ConstructorFields), 5)
	}
	testField(t, testFilename, model, "Blacklist", "bool")
	testField(t, testFilename, model, "CheckExpression", "interface{}")
	testField(t, testFilename, model, "Om", "*OtherMessage")
	testField(t, testFilename, model, "Val", "istio_mixer_v1_config_descriptor.ValueType")
	testField(t, testFilename, model, "Submsgfield", "*ConstructorSubmessage")
}

func testField(t *testing.T, testFilename string, model Model, fldName string,  expectedFldType string){
	found := false
	for _, cf := range model.ConstructorFields {
		if cf.Name == fldName {
			found = true
			if cf.Type.Name != expectedFldType {
				t.Fatalf("CreateModel(%s).ConstructorFields[%s] = %s, wanted %s", testFilename, fldName, cf.Type.Name,expectedFldType)
			}
		}
	}
	if !found {
		t.Fatalf("CreateModel(%s).ConstructorFields = %v, wanted to contain field with name '%s'", testFilename, model.ConstructorFields, fldName)
	}
}
func testError(t *testing.T, inputTemplateProto string, expectedError string) {

	_, err := createTestModel(t, inputTemplateProto)

	if !strings.Contains(err.Error(), expectedError) {
		t.Fatalf("CreateModel(%s) = %v, \n wanted err that contains string `%v`", inputTemplateProto, err, fmt.Errorf(expectedError))
	}
}

func createTestModel(t *testing.T, inputTemplateProto string) (Model, error) {
	outDir := path.Join("testdata", t.Name())
	_, _ = filepath.Abs(outDir)
	err := os.RemoveAll(outDir)
	os.MkdirAll(outDir, os.ModePerm)
	defer os.RemoveAll(outDir)

	outFDS := path.Join(outDir, "outFDS.pb")
	defer os.Remove(outFDS)
	err = generteFDSFileHacky(inputTemplateProto, outFDS)
	if err != nil {
		t.Fatalf("Unable to generate file descriptor set %v", err)
	}

	fds, err := getFileDescSet(outFDS)
	if err != nil {
		t.Fatalf("Unable to parse file descriptor set file %v", err)

	}

	parser, err := CreateFileDescriptorSetParser(fds, map[string]string{})
	return CreateModel(parser)
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

// TODO: This is blocking the test to be enabled from Bazel.
func generteFDSFileHacky(protoFile string, outputFDSFile string) error {

	// HACK HACK. Depending on dir structure is super fragile.
	// Explore how to generate File Descriptor set in a better way.
	protocCmd := []string{
		path.Join("mixer/tools/codegen/model_generator", protoFile),
		"-o",
		fmt.Sprintf("%s", path.Join("mixer/tools/codegen/model_generator", outputFDSFile)),
		"-I=.",
		"-I=api",
		"--include_imports",
	}
	cmd := exec.Command("protoc", protocCmd...)
	dir := path.Join(os.Getenv("GOPATH"), "src/istio.io")
	cmd.Dir = dir
	cmd.Stderr = os.Stderr // For debugging
	err := cmd.Run()
	return err
}
