# gazelle:ignore
package(default_visibility = ["//visibility:public"])

load("//tools/codegen:adapter.bzl", "mixer_adapter_library")

mixer_adapter_library(
    name = "go_default_library",
    packages = {
        "//test/e2e/advance/template/report:go_default_library_proto.descriptor_set": "istio.io/mixer/test/e2e/advance/template/report",
    },
    deps = [
        "//test/e2e/advance/template/report:go_default_library",
    ],
)
