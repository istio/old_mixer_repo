package(default_visibility = ["//tools/codegen/pkg/adapter:__pkg__"])

load("@org_pubref_rules_protobuf//protobuf:rules.bzl", "proto_compile")
load("//tools/codegen:generate.bzl", "mixer_proto_library")

mixer_proto_library(
    name = "check",
    testonly = True,
    protos = ["CheckTmpl.proto"],
)

mixer_proto_library(
    name = "report",
    testonly = True,
    protos = ["ReportTmpl.proto"],
)

mixer_proto_library(
    name = "quota",
    testonly = True,
    protos = ["QuotaTmpl.proto"],
)

filegroup(
    name = "descriptors",
    testonly = True,
    srcs = [
        "check_proto.descriptor_set",
        "quota_proto.descriptor_set",
        "report_proto.descriptor_set",
    ],
)

filegroup(
    name = "golden_files",
    testonly = True,
    srcs = [
        "AllTemplates.go.golden",
    ],
)
