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

/*
Code in this class is mostly a copy from https://github.com/golang/protobuf/blob/master/protoc-gen-go/generator/generator.go
TODO:
-  Handle name collision. When multiple files in the FileDescriptorSet have the same name, the protoc-gen-go do name
   substitution before printing the package name. Our generated code that might reference those packages should also
   use the substituted name that protoc-gen-go would use to name that package.
-  Handle go_package option. Currently the name of the package is computed just based on the file path.
-  Handle Oneofs. The protoc-gen-go have special logic (removed from this file, but present in the protoc-gen-go).
   Our generated interface code might also need that special logic if the Instance object wants to reference a
   oneof from the generated mytemplate.pb.go
-  Handle Extensions. Should we allow Constructor message to have extension fields. Extensions have are a complete
   different handling in protoc-gen-go. So far I am what it means for our generated code.
-  Accept all the other parameters that protoc-gen-go takes in our generator. Our codegen should use those
   paramters for it's codegen and pass to the protoc-gen-go plugin (invoking another protoc process).
*/

package modelgen

import (
	"fmt"
	"strings"
	"unicode"
	"path"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"os"
)

type FileDescriptorSetParser struct {
	typeNameToObject  map[string]Object // Key is a fully-qualified name in input syntax.
	Param             map[string]string // Command-line parameters.
	PackageImportPath string            // Go import path of the package we're generating code for
	ImportMap         map[string]string // Mapping from .proto file name to import path

	Pkg map[string]string // The names under which we import support packages

	packageName    string                     // What we're calling ourselves.
	allFiles       []*FileDescriptor          // All files in the tree
	allFilesByName map[string]*FileDescriptor // All files by filename.
	usedPackages   map[string]bool            // Names of packages used in current file.
}

type common struct {
	file *descriptor.FileDescriptorProto // File this object comes from.
}

type FileDescriptor struct {
	*descriptor.FileDescriptorProto
	desc []*Descriptor     // All the messages defined in this file.
	enum []*EnumDescriptor // All the enums defined in this file.

	proto3 bool // whether to generate proto3 code for this file
}

type Descriptor struct {
	common
	*descriptor.DescriptorProto
	parent      *Descriptor       // The containing message, if any.
	nested      []*Descriptor     // Inner messages, if any.
	enums       []*EnumDescriptor // Inner enums, if any.
	typename    []string          // Cached typename vector.
	group       bool
	refPackages []string
}

type EnumDescriptor struct {
	common
	*descriptor.EnumDescriptorProto
	parent   *Descriptor // The containing message, if any.
	typename []string    // Cached typename vector.
}

type Object interface {
	PackageName() string // The name we use in our output (a_b_c), possibly renamed for uniqueness.
	TypeName() []string
	File() *descriptor.FileDescriptorProto
}

func CreateFileDescriptorSetParser(fds *descriptor.FileDescriptorSet, importMap map[string]string) (*FileDescriptorSetParser, error) {
	parser := &FileDescriptorSetParser{ImportMap: importMap}
	parser.WrapTypes(fds)
	parser.BuildTypeNameMap()
	return parser, nil
}

func (g *FileDescriptorSetParser) WrapTypes(fds *descriptor.FileDescriptorSet) {
	g.allFiles = make([]*FileDescriptor, 0, len(fds.File))
	g.allFilesByName = make(map[string]*FileDescriptor, len(g.allFiles))
	for _, f := range fds.File {
		g.WrapFileDescriptor(f)

	}
}

func (g *FileDescriptorSetParser) WrapFileDescriptor(f *descriptor.FileDescriptorProto) {
	if _, ok := g.allFilesByName[f.GetName()]; !ok {
		// We must wrap the descriptors before we wrap the enums
		descs := wrapDescriptors(f)
		g.buildNestedDescriptors(descs)
		enums := wrapEnumDescriptors(f, descs)
		g.buildNestedEnums(descs, enums)
		fd := &FileDescriptor{
			FileDescriptorProto: f,
			desc:                descs,
			enum:                enums,
			proto3:              fileIsProto3(f),
		}

		g.allFiles = append(g.allFiles, fd)
		g.allFilesByName[f.GetName()] = fd
	}
}

func wrapDescriptors(file *descriptor.FileDescriptorProto) []*Descriptor {
	sl := make([]*Descriptor, 0, len(file.MessageType)+10)
	for i, desc := range file.MessageType {
		sl = wrapThisDescriptor(sl, desc, nil, file, i)
	}
	return sl
}

func wrapThisDescriptor(sl []*Descriptor, desc *descriptor.DescriptorProto, parent *Descriptor, file *descriptor.FileDescriptorProto, index int) []*Descriptor {
	sl = append(sl, newDescriptor(desc, parent, file, index))
	me := sl[len(sl)-1]
	for i, nested := range desc.NestedType {
		sl = wrapThisDescriptor(sl, nested, me, file, i)
	}
	return sl
}

func wrapEnumDescriptors(file *descriptor.FileDescriptorProto, descs []*Descriptor) []*EnumDescriptor {
	sl := make([]*EnumDescriptor, 0, len(file.EnumType)+10)
	// Top-level enums.
	for i, enum := range file.EnumType {
		sl = append(sl, newEnumDescriptor(enum, nil, file, i))
	}
	// Enums within messages. Enums within embedded messages appear in the outer-most message.
	for _, nested := range descs {
		for i, enum := range nested.EnumType {
			sl = append(sl, newEnumDescriptor(enum, nested, file, i))
		}
	}
	return sl
}

// Descriptor methods ..

func newDescriptor(desc *descriptor.DescriptorProto, parent *Descriptor, file *descriptor.FileDescriptorProto, index int) *Descriptor {
	d := &Descriptor{
		common:          common{file},
		DescriptorProto: desc,
		parent:          parent,
	}

	// The only way to distinguish a group from a message is whether
	// the containing message has a TYPE_GROUP field that matches.
	if parent != nil {
		parts := d.TypeName()
		if file.Package != nil {
			parts = append([]string{*file.Package}, parts...)
		}
		exp := "." + strings.Join(parts, ".")
		for _, field := range parent.Field {
			if field.GetType() == descriptor.FieldDescriptorProto_TYPE_GROUP && field.GetTypeName() == exp {
				d.group = true
				break
			}
		}
	}

	return d
}

func (g *FileDescriptorSetParser) buildNestedDescriptors(descs []*Descriptor) {
	for _, desc := range descs {
		if len(desc.NestedType) != 0 {
			for _, nest := range descs {
				if nest.parent == desc {
					desc.nested = append(desc.nested, nest)
				}
			}
			if len(desc.nested) != len(desc.NestedType) {
				g.Fail("internal error: nesting failure for", desc.GetName())
			}
		}
	}
}

func (d *Descriptor) TypeName() []string {
	if d.typename != nil {
		return d.typename
	}
	n := 0
	for parent := d; parent != nil; parent = parent.parent {
		n++
	}
	s := make([]string, n, n)
	for parent := d; parent != nil; parent = parent.parent {
		n--
		s[n] = parent.GetName()
	}
	d.typename = s
	return s
}

// Enum methods ..

func newEnumDescriptor(desc *descriptor.EnumDescriptorProto, parent *Descriptor, file *descriptor.FileDescriptorProto, index int) *EnumDescriptor {
	ed := &EnumDescriptor{
		common:          common{file},
		EnumDescriptorProto: desc,
		parent:              parent,
	}
	return ed
}

func (g *FileDescriptorSetParser) buildNestedEnums(descs []*Descriptor, enums []*EnumDescriptor) {
	for _, desc := range descs {
		if len(desc.EnumType) != 0 {
			for _, enum := range enums {
				if enum.parent == desc {
					desc.enums = append(desc.enums, enum)
				}
			}
			if len(desc.enums) != len(desc.EnumType) {
				g.Fail("internal error: enum nesting failure for", desc.GetName())
			}
		}
	}
}

func (e *EnumDescriptor) TypeName() (s []string) {
	if e.typename != nil {
		return e.typename
	}
	name := e.GetName()
	if e.parent == nil {
		s = make([]string, 1)
	} else {
		pname := e.parent.TypeName()
		s = make([]string, len(pname)+1)
		copy(s, pname)
	}
	s[len(s)-1] = name
	e.typename = s
	return s
}

// File level methods

func (d *FileDescriptor) goFileName() string {
	name := *d.Name
	if ext := path.Ext(name); ext == ".proto" || ext == ".protodevel" {
		name = name[:len(name)-len(ext)]
	}
	name += ".pb.go"

	// Does the file have a "go_package" option?
	// If it does, it may override the filename.
	if impPath, _, ok := d.goPackageOption(); ok && impPath != "" {
		// Replace the existing dirname with the declared import path.
		_, name = path.Split(name)
		name = path.Join(impPath, name)
		return name
	}

	return name
}

// goPackageOption interprets the file's go_package option.
// If there is no go_package, it returns ("", "", false).
// If there's a simple name, it returns ("", pkg, true).
// If the option implies an import path, it returns (impPath, pkg, true).
func (d *FileDescriptor) goPackageOption() (impPath, pkg string, ok bool) {
	pkg = d.GetOptions().GetGoPackage()
	if pkg == "" {
		return
	}
	ok = true
	// The presence of a slash implies there's an import path.
	slash := strings.LastIndex(pkg, "/")
	if slash < 0 {
		return
	}
	impPath, pkg = pkg, pkg[slash+1:]
	// A semicolon-delimited suffix overrides the package name.
	sc := strings.IndexByte(impPath, ';')
	if sc < 0 {
		return
	}
	impPath, pkg = impPath[:sc], impPath[sc+1:]
	return
}

func (d *FileDescriptor) PackageName() string { return goPackageName(*d.FileDescriptorProto.Name) }

// FileDescriptorSetParser methods

func (g *FileDescriptorSetParser) Fail(msgs ...string) {
	s := strings.Join(msgs, " ")
	fmt.Fprintln(os.Stderr,"model_generator: error:", s)
	os.Exit(1)
}

func (g *FileDescriptorSetParser) FileOf(fd *descriptor.FileDescriptorProto) *FileDescriptor {
	for _, file := range g.allFiles {
		if file.FileDescriptorProto == fd {
			return file
		}
	}
	g.Fail("could not find file in table:", fd.GetName())
	return nil
}

func (g *FileDescriptorSetParser) fileByName(filename string) *FileDescriptor {
	return g.allFilesByName[filename]
}

func (g *FileDescriptorSetParser) GoType(message *descriptor.DescriptorProto, field *descriptor.FieldDescriptorProto) (typ string) {
	switch *field.Type {
	case descriptor.FieldDescriptorProto_TYPE_DOUBLE:
		typ = "float64"
	case descriptor.FieldDescriptorProto_TYPE_FLOAT:
		typ = "float32"
	case descriptor.FieldDescriptorProto_TYPE_INT64:
		typ = "int64"
	case descriptor.FieldDescriptorProto_TYPE_UINT64:
		typ = "uint64"
	case descriptor.FieldDescriptorProto_TYPE_INT32:
		typ = "int32"
	case descriptor.FieldDescriptorProto_TYPE_UINT32:
		typ = "uint32"
	case descriptor.FieldDescriptorProto_TYPE_FIXED64:
		typ = "uint64"
	case descriptor.FieldDescriptorProto_TYPE_FIXED32:
		typ = "uint32"
	case descriptor.FieldDescriptorProto_TYPE_BOOL:
		typ = "bool"
	case descriptor.FieldDescriptorProto_TYPE_STRING:
		typ = "string"
	case descriptor.FieldDescriptorProto_TYPE_GROUP:
		// TODO : What needs to be done in this case?
		g.Fail(fmt.Sprintf("unsupported field type %s for field %s", descriptor.FieldDescriptorProto_TYPE_GROUP.String(), field.GetName()))
	case descriptor.FieldDescriptorProto_TYPE_MESSAGE:
		desc := g.ObjectNamed(field.GetTypeName())
		typ = "*" + g.TypeName(desc)

		if d, ok := desc.(*Descriptor); ok && d.GetOptions().GetMapEntry() {
			keyField, valField := d.Field[0], d.Field[1]
			keyType := g.GoType(d.DescriptorProto, keyField)
			valType := g.GoType(d.DescriptorProto, valField)

			keyType = strings.TrimPrefix(keyType, "*")
			switch *valField.Type {
			case descriptor.FieldDescriptorProto_TYPE_ENUM:
				valType = strings.TrimPrefix(valType, "*")

			case descriptor.FieldDescriptorProto_TYPE_MESSAGE:

			default:
				valType = strings.TrimPrefix(valType, "*")
			}

			typ = fmt.Sprintf("map[%s]%s", keyType, valType)
			return
		}
	case descriptor.FieldDescriptorProto_TYPE_BYTES:
		typ = "[]byte"
	case descriptor.FieldDescriptorProto_TYPE_ENUM:
		desc := g.ObjectNamed(field.GetTypeName())
		typ = g.TypeName(desc)
	case descriptor.FieldDescriptorProto_TYPE_SFIXED32:
		typ = "int32"
	case descriptor.FieldDescriptorProto_TYPE_SFIXED64:
		typ = "int64"
	case descriptor.FieldDescriptorProto_TYPE_SINT32:
		typ = "int32"
	case descriptor.FieldDescriptorProto_TYPE_SINT64:
		typ = "int64"
	default:
		g.Fail("unknown type for", field.GetName())

	}
	if isRepeated(field) {
		typ = "[]" + typ
	} else if message != nil {
			return
	} else if field.OneofIndex != nil && message != nil {
		g.Fail("oneof not supported ", field.GetName())
		return
	} else if needsStar(*field.Type) {
		typ = "*" + typ
	}
	return
}

func (g *FileDescriptorSetParser) TypeName(obj Object) string {
	return g.DefaultPackageName(obj) + CamelCaseSlice(obj.TypeName())
}

func (g *FileDescriptorSetParser) DefaultPackageName(obj Object) string {
	// TODO if the protoc is not executed with --include_imports, this
	// is guaranteed to throw NPE.
	// Handle it.
	if obj == nil {
		return ""
	}
	pkg := obj.PackageName()
	if pkg == g.packageName {
		return ""
	}
	return pkg + "."
}

// ObjectNamed, given a fully-qualified input type name as it appears in the input data,
// returns the descriptor for the message or enum with that name.
func (g *FileDescriptorSetParser) ObjectNamed(typeName string) Object {
	o, ok := g.typeNameToObject[typeName]
	if !ok {
		g.Fail("can't find object with type", typeName)
	}

	return o
}

func (g *FileDescriptorSetParser) BuildTypeNameMap() {
	g.typeNameToObject = make(map[string]Object)
	for _, f := range g.allFiles {
		// The names in this loop are defined by the proto world, not us, so the
		// package name may be empty.  If so, the dotted package name of X will
		// be ".X"; otherwise it will be ".pkg.X".
		dottedPkg := "." + f.GetPackage()
		if dottedPkg != "." {
			dottedPkg += "."
		}
		for _, enum := range f.enum {
			name := dottedPkg + dottedSlice(enum.TypeName())
			g.typeNameToObject[name] = enum
		}
		for _, desc := range f.desc {
			name := dottedPkg + dottedSlice(desc.TypeName())
			g.typeNameToObject[name] = desc
		}
	}
}

// common struct methods

func (c *common) PackageName() string {
	f := c.file
	return goPackageName(f.GetPackage())
}

func (c *common) File() *descriptor.FileDescriptorProto { return c.file }

// helper methods

func dottedSlice(elem []string) string { return strings.Join(elem, ".") }

func isRepeated(field *descriptor.FieldDescriptorProto) bool {
	return field.Label != nil && *field.Label == descriptor.FieldDescriptorProto_LABEL_REPEATED
}

func needsStar(typ descriptor.FieldDescriptorProto_Type) bool {
	switch typ {
	case descriptor.FieldDescriptorProto_TYPE_GROUP:
		return false
	case descriptor.FieldDescriptorProto_TYPE_MESSAGE:
		return false
	case descriptor.FieldDescriptorProto_TYPE_BYTES:
		return false
	}
	return true
}

// Is c an ASCII lower-case letter?
func isASCIILower(c byte) bool {
	return 'a' <= c && c <= 'z'
}

// Is c an ASCII digit?
func isASCIIDigit(c byte) bool {
	return '0' <= c && c <= '9'
}

func CamelCase(s string) string {
	if s == "" {
		return ""
	}
	t := make([]byte, 0, 32)
	i := 0
	if s[0] == '_' {
		// Need a capital letter; drop the '_'.
		t = append(t, 'X')
		i++
	}
	// Invariant: if the next letter is lower case, it must be converted
	// to upper case.
	// That is, we process a word at a time, where words are marked by _ or
	// upper case letter. Digits are treated as words.
	for ; i < len(s); i++ {
		c := s[i]
		if c == '_' && i+1 < len(s) && isASCIILower(s[i+1]) {
			continue // Skip the underscore in s.
		}
		if isASCIIDigit(c) {
			t = append(t, c)
			continue
		}
		// Assume we have a letter now - if not, it's a bogus identifier.
		// The next word is a sequence of characters that must start upper case.
		if isASCIILower(c) {
			c ^= ' ' // Make it a capital letter.
		}
		t = append(t, c) // Guaranteed not lower case.
		// Accept lower case sequence that follows.
		for i+1 < len(s) && isASCIILower(s[i+1]) {
			i++
			t = append(t, s[i])
		}
	}
	return string(t)
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func CamelCaseSlice(elem []string) string { return CamelCase(strings.Join(elem, "_")) }

func fileIsProto3(file *descriptor.FileDescriptorProto) bool {
	return file.GetSyntax() == "proto3"
}

// badToUnderscore is the mapping function used to generate Go names from package names,
// which can be dotted in the input .proto file.  It replaces non-identifier characters such as
// dot or dash with underscore.
func badToUnderscore(r rune) rune {
	if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
		return r
	}
	return '_'
}

func goPackageName(pkg string) string {
	return strings.Map(badToUnderscore, pkg)
}


