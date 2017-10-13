load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["common.go"],
    importpath = "istio.io/mixer/adapter/stackdriver/helper",
    visibility = ["//visibility:public"],
    deps = [
        "//adapter/stackdriver/config:go_default_library",
        "@org_golang_google_api//option:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["common_test.go"],
    importpath = "istio.io/mixer/adapter/stackdriver/helper",
    library = ":go_default_library",
    deps = [
        "//adapter/stackdriver/config:go_default_library",
        "@org_golang_google_api//option:go_default_library",
    ],
)
