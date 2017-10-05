package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_binary(
    name = "mixs",
    srcs = [
        "main.go",
    ],
    linkstamp = "istio.io/mixer/pkg/version",
    visibility = ["//visibility:public"],
    deps = [
        "//adapter:go_default_library",
        "//cmd/server/cmd:go_default_library",
        "//cmd/shared:go_default_library",
        "//pkg/adapter:go_default_library",
        "//pkg/template:go_default_library",
        "//template:go_default_library",
        "@com_github_spf13_cobra//:go_default_library",
    ],
)

go_library(
    name = "go_default_library",
    srcs = ["main.go"],
    deps = [
        "//adapter:go_default_library",
        "//cmd/server/cmd:go_default_library",
        "//cmd/shared:go_default_library",
        "//pkg/adapter:go_default_library",
        "//pkg/template:go_default_library",
        "//template:go_default_library",
    ],
)

go_binary(
    name = "server",
    library = ":go_default_library",
)
