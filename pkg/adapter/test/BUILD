package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    testonly = 1,
    srcs = glob(
        include = ["*.go"],
        exclude = ["*_test.go"],
    ),
    deps = [
        "//pkg/adapter:go_default_library",
    ],
)
