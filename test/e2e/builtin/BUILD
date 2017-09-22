# gazelle:ignore
package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")
load("//tools/codegen:adapter.bzl", "mixer_adapter_library")

go_test(
    name = "e2e_builtin_tmpls_testdata",
    size = "small",
    srcs = [
        "common.go",
        "report_test.go",
    ],
    data = [
        "attrs.yaml",
        "handlers.yaml",
        "rules.yaml",
        "//testdata",
    ],
    library = ":go_default_library",  # keep
    deps = [
        "//pkg/adapter:go_default_library",
        "//pkg/attribute:go_default_library",
        "//pkg/template:go_default_library",
        "//template:go_default_library",
        "//template/checknothing:go_default_library",
        "//template/listentry:go_default_library",
        "//template/logentry:go_default_library",
        "//template/metric:go_default_library",
        "//template/quota:go_default_library",
        "//template/reportnothing:go_default_library",
        "//test/e2e:go_default_library",
        "//test/testenv:go_default_library",
        "@com_github_davecgh_go_spew//spew:go_default_library",
        "@com_github_gogo_protobuf//types:go_default_library",
        "@com_github_googleapis_googleapis//:google/rpc",
        "@com_github_istio_api//:mixer/v1",  # keep
        "@com_github_istio_api//:mixer/v1/config/descriptor",  # keep
        "@com_github_pborman_uuid//:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
    ],
)

mixer_adapter_library(
    name = "go_default_library",
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
