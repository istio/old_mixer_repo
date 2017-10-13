load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["fakebag.go"],
    importpath = "istio.io/mixer/pkg/il/testing",
    visibility = ["//visibility:public"],
    deps = ["//pkg/attribute:go_default_library"],
)
