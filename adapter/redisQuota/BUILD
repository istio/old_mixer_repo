package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "driver.go",
        "driver_impl.go",
        "redisQuota.go",
    ],
    deps = [
        "//adapter/redisQuota/config:go_default_library",
        "//pkg/adapter:go_default_library",
        "//pkg/pool:go_default_library",
        "@com_github_gogo_protobuf//types:go_default_library",
        "@com_github_redis_client//pool:go_default_library",
        "@com_github_redis_client//redis:go_default_library",
    ],
)
