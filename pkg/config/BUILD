package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "cfg.pb.go",
        "runtime.go",
        "validater.go",
    ],
    deps = [
        "//pkg/aspect:go_default_library",
        "//pkg/attribute:go_default_library",
        "//pkg/expr:go_default_library",
        "@com_github_golang_protobuf//proto:go_default_library",
        "@com_github_mitchellh_mapstructure//:go_default_library",
        "@in_gopkg_yaml_v2//:go_default_library",
    ],
)

go_test(
    name = "config_test",
    size = "small",
    srcs = [
        "runtime_test.go",
        "validater_test.go",
    ],
    library = ":go_default_library",
    deps = [
        "@com_github_istio_api//:mixer/v1",
        "@com_github_istio_api//:mixer/v1/config/aspect",
    ],
)
