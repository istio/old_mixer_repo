load("@org_pubref_rules_protobuf//gogo:rules.bzl", "gogoslick_proto_library")

gogoslick_proto_library(
    name = "go_default_library",
    importmap = {
        "gogoproto/gogo.proto": "github.com/gogo/protobuf/gogoproto",
        "mixer/v1/config/descriptor/attribute_descriptor.proto": "istio.io/api/mixer/v1/config/descriptor",
    },
    imports = [
        "../../../../external/com_github_gogo_protobuf",
        "external/com_github_google_protobuf/src",
        "external/com_github_istio_api",
    ],
    inputs = [
        "@com_github_istio_api//:mixer/v1/config/descriptor_protos",
        "@com_github_google_protobuf//:well_known_protos",
        "@com_github_gogo_protobuf//gogoproto:go_default_library_protos",
    ],
    protos = [
        "accessLogs.proto",
        "applicationLogs.proto",
        "attrgen.proto",
        "denials.proto",
        "lists.proto",
        "metrics.proto",
        "quotas.proto",
    ],
    verbose = 0,
    visibility = [
        "//pkg/aspect:__pkg__",
        "//pkg/config:__subpackages__",
    ],
    deps = [
        "@com_github_gogo_protobuf//gogoproto:go_default_library",
        "@com_github_gogo_protobuf//sortkeys:go_default_library",
        "@com_github_istio_api//:mixer/v1/config/descriptor",
    ],
)
