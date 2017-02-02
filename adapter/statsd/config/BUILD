load("@org_pubref_rules_protobuf//go:rules.bzl", "go_proto_library")

go_proto_library(
    name = "go_default_library",
    imports = [
        "external/com_github_google_protobuf/src",
    ],
    inputs = [
        "@com_github_google_protobuf//:well_known_protos",
    ],
    protos = [
        "config.proto",
    ],
    verbose = 0,
    visibility = ["//adapter/statsd:__pkg__"],
    deps = [
        "@com_github_golang_protobuf//ptypes/duration:go_default_library",
    ],
)
