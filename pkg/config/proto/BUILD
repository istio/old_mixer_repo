load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["combined.go"],
    visibility = ["//visibility:public"],
    deps = ["@io_istio_api//mixer/v1/config:go_default_library"],
)
