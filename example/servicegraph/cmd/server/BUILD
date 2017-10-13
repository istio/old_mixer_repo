package(default_visibility = ["//example/servicegraph:__subpackages__"])

load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["main.go"],
    importpath = "istio.io/mixer/example/servicegraph/cmd/server",
    visibility = ["//visibility:private"],
    deps = [
        "//example/servicegraph:go_default_library",
        "//example/servicegraph/dot:go_default_library",
        "//example/servicegraph/promgen:go_default_library",
    ],
)

alias(
    name = "servicegraph",
    actual = ":server",
)

go_binary(
    name = "server",
    importpath = "istio.io/mixer/example/servicegraph/cmd/server",
    library = ":go_default_library",
)
