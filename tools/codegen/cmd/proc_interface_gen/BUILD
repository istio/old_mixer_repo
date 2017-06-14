package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_prefix", "go_library", "go_test", "go_binary")

go_binary(
    name = "procInterfaceGen",
    srcs = [
        "main.go",
    ],
    library = "//tools/codegen/pkg/proc_interface_gen:go_default_library",
    deps = [
        "@com_github_spf13_cobra//:go_default_library",
    ],
)
