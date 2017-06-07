package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_binary(
    name = "importer",
    srcs = [
        "main.go",
    ],
    linkstamp = "istio.io/mixer/pkg/version",
    visibility = ["//visibility:public"],
    deps = [
        "//cmd/shared:go_default_library",
        "//pkg/config:go_default_library",
        "//pkg/version:go_default_library",
        "@com_github_golang_glog//:go_default_library",
    ],
)
