package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["opa.go"],
    visibility = ["//visibility:public"],
    deps = [
        "//adapter/opa/config:go_default_library",
        "//pkg/adapter:go_default_library",
        "//pkg/status:go_default_library",
        "//template/authz:go_default_library",
        "@com_github_googleapis_googleapis//:google/rpc",
        "@com_github_open_policy_agent_opa//ast:go_default_library",
        "@com_github_open_policy_agent_opa//rego:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    size = "small",
    srcs = ["opa_test.go"],
    library = ":go_default_library",
    deps = [
        "//adapter/opa/config:go_default_library",
        "//pkg/adapter/test:go_default_library",
        "//template/authz:go_default_library",
        "@com_github_googleapis_googleapis//:google/rpc",
        "@io_istio_api//:mixer/v1/config/descriptor",
    ],
)
