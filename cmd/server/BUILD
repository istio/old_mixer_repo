package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_binary(
    name = "mixs",
    srcs = [
        "main.go",
        "version.go",
    ],
    linkstamp = "main",
    visibility = ["//visibility:public"],
    deps = [
        "//cmd/server/cmd:go_default_library",
        "@com_github_spf13_cobra//:go_default_library",
    ],
)
