package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")
load(":hndlrCnfg.bzl", "handler_config_library")

go_library(
    name = "go_default_library",
    srcs = [
        "info.go",
    ],
    library = ":handler_config_lib",  # keep
    visibility = ["//visibility:public"],
    deps = [
        "//pkg/adapter:go_default_library",
        "//pkg/status:go_default_library",
        "//template/checknothing:go_default_library",
        "//template/listentry:go_default_library",
        "//template/logentry:go_default_library",
        "//template/metric:go_default_library",
        "//template/quota:go_default_library",
        "//template/reportnothing:go_default_library",
        "@com_github_gogo_protobuf//proto:go_default_library",
        "@com_github_googleapis_googleapis//:google/rpc",
        "@com_github_hashicorp_go_multierror//:go_default_library",
    ],
)

handler_config_library(
    name = "handler_config_lib",
    packages = {
        "//template/listentry:go_default_library_proto.descriptor_set": "istio.io/mixer/template/listentry",
        "//template/logentry:go_default_library_proto.descriptor_set": "istio.io/mixer/template/logentry",
        "//template/metric:go_default_library_proto.descriptor_set": "istio.io/mixer/template/metric",
        "//template/quota:go_default_library_proto.descriptor_set": "istio.io/mixer/template/quota",
        "//template/reportnothing:go_default_library_proto.descriptor_set": "istio.io/mixer/template/reportnothing",
        "//template/checknothing:go_default_library_proto.descriptor_set": "istio.io/mixer/template/checknothing",
    },
    deps = [
        "//template/checknothing:go_default_library",
        "//template/listentry:go_default_library",
        "//template/logentry:go_default_library",
        "//template/metric:go_default_library",
        "//template/quota:go_default_library",
        "//template/reportnothing:go_default_library",
    ],
)
