load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["version.go"],
    importpath = "istio.io/mixer/pkg/version",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    size = "small",
    srcs = ["version_test.go"],
    importpath = "istio.io/mixer/pkg/version",
    library = ":go_default_library",
)
