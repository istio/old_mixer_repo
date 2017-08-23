package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "circonus.go",
    ],
    deps = [
        "//adapter/circonus/config:go_default_library",
        "//pkg/adapter:go_default_library",
        "@com_github_circonuslabs_circonus_gometrics//:go_default_library",
        "@com_github_hashicorp_go_multierror//:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    size = "small",
    srcs = ["circonus_test.go"],
    library = ":go_default_library",
    deps = [
        "//pkg/adapter/test:go_default_library",
        "@com_github_golang_protobuf//proto:go_default_library",
        "@com_github_googleapis_googleapis//:google/rpc",
    ],
)
