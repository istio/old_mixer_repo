package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "template.gen.go",
    ],
    visibility = ["//visibility:public"],
    deps = [
        "//pkg/adapter:go_default_library",
        "//pkg/adapter/template:go_default_library",
        "//pkg/attribute:go_default_library",
        "//pkg/expr:go_default_library",
        "//pkg/template:go_default_library",
        "//test/e2e/template/report:go_default_library",
        "@com_github_gogo_protobuf//proto:go_default_library",
        "@com_github_golang_glog//:go_default_library",
        "@com_github_istio_api//:mixer/v1/config/descriptor",  # keep
    ],
)
