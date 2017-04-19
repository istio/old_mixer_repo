package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "noop.go",
    ],
    deps = [
        "//adapter/noop/config:go_default_library",
        "//pkg/adapter:go_default_library",
        "@com_github_googleapis_googleapis//:google/rpc",
    ],
)
