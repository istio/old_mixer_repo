# Istio Mixer: Template Developer’s Guide

Templates are a foundational building block of the Mixer architecture. This document
describes the template format and how to extend Mixer with custom templates.

- [Background](#background)
- [Template format](#template-format)
- [Supported field names](#supported-field-names)
- [Supported field types](#supported-field-types)
- [Adding a template to Mixer](#adding-a-template-to-mixer)
- [Template evolution compatibility](#template-evolution-compatibility)

## Background

Mixer is an attribute processing machine. It ingests attributes coming from the proxy and responds by
triggering calls into a suite of adapters. These adapters in turn communicate with infrastructure
backends that offer a variety of capabilities. The operator that configures Istio controls what Mixer
does with incoming attributes and which adapters are called as a result.

![Attribute Processing Machine](./attribute_machine.svg) 

Mixer categorizes adapters based on the type of data they consume. For example, there are metric adapters, logging
adapters, quota adapters, access control adapters, and more. The type of data Mixer delivers to individual adapters
depends on each adapter’s category. All adapters of a given category receive the same type of data. For example,
metric adapters all receive metrics at runtime.

Mixer determines the format of data that each adapter receive using *templates*. A template is a simple protobuf message
which lays out the kind of data that a specific category of adapter will be given at runtime. The template determines
the data adapters receive at runtime, as well as the instances operators must create in order to use an adapter.
The [Adapter Developer's Guide](./adapters.md) explains how templates are automatically transformed into Go
artifacts that can be used by adapter developers, and into config definitions that can be used by operators. 

Mixer includes a number of [canonical templates](https://github.com/istio/mixer/tree/master/template) which cover
most of the anticipated workloads that Mixer is expected to be used with. However, the set of supported templates can
readily be extended in order to support emerging usage scenarios. Note that it’s preferable to use existing templates
when possible as it tends to deliver a better end-to-end story for the ecosystem. 

This document describes the simple rules used to create templates for Mixer.
The [Adapter Developer’s Guide](https://github.com/istio/mixer/blob/master/doc/dev/adapters.md) describes how to use templates to
create adapters.

## Template format

Templates are expressed using the protobuf syntax. They are generally fairly simple data structures. An an example, here is the `listentry` template:

<pre>
syntax = "proto3";

package listEntry;

import "mixer/v1/template/extensions.proto";

option (istio.mixer.v1.template.template_variety) = TEMPLATE_VARIETY_CHECK;

// ListEntry is used to verify the presence/absence of a string
// within a list.
//
// When writing the configuration, the value for the fields associated with this template can either be a
// literal or an [expression](https://istio.io/docs/reference/config/mixer/expression-language.html). Please note that if the datatype of a field is not istio.mixer.v1.config.descriptor.ValueType,
// then the expression's [inferred type](https://istio.io/docs/reference/config/mixer/expression-language.html#type-checking) must match the datatype of the field.
//
// Example config:
// 
// apiVersion: "config.istio.io/v1alpha2"
// kind: listentry
// metadata:
//   name: appversion
//   namespace: istio-config-default
// spec:
//   value: source.labels["version"]
message Template {
    // Specifies the entry to verify in the list.
    string value = 1;
}
</pre>

The interesting parts of this definition are:

- **Package Name**. The package name determines the name by which the template will be known, both by
adapter authors and operators using adapters that implement the template. 
The name should be written in camelCase.

- **Variety**. The variety of a template determines at what point in the Mixer's processing pipeline the
adapters that implement the template will be called. This can be CHECK, REPORT, or QUOTA. 

- **Message Name**. The name of the template message should always be `Template`

- **Fields**. The fields represent the general shape of the data that will be delivered to the adapters at
runtime. The operator will need to populate these fields via configuration.

## Supported field names

Template fields can have any valid protobuf field name, except for the reserved name `name`.

## Supported field types

Templates currently only support a subset of the full protobuf type system. Fields in a template can
be one of the following types:

- `string`
- `int64`
- `double`
- `bool`
- `istio.mixer.v1.config.descriptor.ValueType`
- `google.protobuf.Timestamp`
- `google.protobuf.Duration`
- `map<string, string>`
- `map<string, int64>`
- `map<string, double>`
- `map<string, bool>`
- `map<string, google.protobuf.Timestamp>`
- `map<string, google.protobuf.Duration>`
- `map<string, istio.mixer.v1.config.descriptor.ValueType>`

There is currently no support for nested messages, enums, `oneof`, and `repeated`.

The type `istio.mixer.v1.config.descriptor.ValueType` has a special meaning. Use of this type
tells Mixer that the associated value can be any of the supported attribute
types are defined by the [ValueType](https://github.com/istio/api/blob/master/mixer/v1/config/descriptor/value_type.proto)
enum. The specific type that will be used at runtime depends on the configuration the operator writes.
Adapters are told what these types are at configuration time so they can prepare themselves accordingly.

## Adding a template to Mixer

Templates are statically linked into Mixer. To add or modify a template, it is therefore necessary to produce a new
Mixer binary.

** TBD **

## Template evolution compatibility

Here's what can be done to a template and remain backward compatible with existing adapters and operator configurations that use the
template:

- Adding a field

The following changes cannot generally be made while maintaining compatibility:

- Renaming the template
- Changing the template's variety
- Removing a field
- Renaming a field
- Changing the type of a field
