load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["generator.go"],
    importpath = "istio.io/mixer/tools/codegen/pkg/inventory",
    visibility = ["//tools:__subpackages__"],
    deps = ["@org_golang_x_tools//imports:go_default_library"],
)

go_test(
    name = "go_default_test",
    size = "small",
    srcs = ["generator_test.go"],
    importpath = "istio.io/mixer/tools/codegen/pkg/inventory",
    library = ":go_default_library",
)
