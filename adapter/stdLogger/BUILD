package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "stdLogger.go",
    ],
    deps = [
        "//adapter/stdLogger/config:go_default_library",
        "//pkg/adapter:go_default_library",
        "//pkg/aspect:go_default_library",
        "//pkg/aspect/logger:go_default_library",
        "//pkg/registry:go_default_library",
        "@com_github_golang_protobuf//proto:go_default_library",
        "@com_github_hashicorp_go_multierror//:go_default_library",
    ],
)

go_test(
    name = "small_tests",
    size = "small",
    srcs = ["stdLogger_test.go"],
    library = ":go_default_library",
    deps = [
        "@com_github_google_go_genproto//googleapis/rpc/status:go_default_library",
    ],
)
