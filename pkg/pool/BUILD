package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "buffer.go",
        "goroutine.go",
        "intern.go",
    ],
    importpath = "istio.io/mixer/pkg/pool",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    size = "small",
    srcs = [
        "buffer_test.go",
        "goroutine_test.go",
        "intern_test.go",
    ],
    importpath = "istio.io/mixer/pkg/pool",
    library = ":go_default_library",
)
