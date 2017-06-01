package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = glob(
        include = ["*.go"],
        exclude = ["*_test.go"],
    ),
    deps = [
        "//pkg/attribute:go_default_library",
        "@com_github_golang_glog//:go_default_library",
        "@com_github_opentracing_basictracer//:go_default_library",
        "@com_github_opentracing_opentracing_go//:go_default_library",
        "@com_github_opentracing_opentracing_go//ext:go_default_library",
        "@com_github_opentracing_opentracing_go//log:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//metadata:go_default_library",
        "@org_golang_google_grpc//tap:go_default_library",
        "@org_golang_x_net//context:go_default_library",
    ],
)

go_test(
    name = "small_tests",
    size = "small",
    srcs = glob(["*_test.go"]),
    library = ":go_default_library",
    deps = [
        "@com_github_istio_api//:mixer/v1",
        "@com_github_opentracing_opentracing_go//mocktracer:go_default_library",
    ],
)
