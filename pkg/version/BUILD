load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = glob(
        include = ["*.go"],
        exclude = ["*_test.go"],
    ),
    visibility = ["//visibility:public"],
)

go_test(
    name = "small_tests",
    size = "small",
    srcs = glob(["*_test.go"]),
    library = ":go_default_library",
)
