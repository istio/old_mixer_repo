package(default_visibility = ["//visibility:public"])

licenses(["notice"])

load("@io_bazel_rules_go//go:def.bzl", "go_prefix")

go_prefix("istio.io/mixer")

config_setting(
    name = "local",
    values = { "define": "istio=local" }
)
ISTIO_DEPS = [
    "//:mixer/api/v1",
    "//:istio/config/v1",
    "//:istio/config/v1/aspect/listChecker"
]

DEPS = select ({
    ":local": ["@local_istio_api" + d for d in ISTIO_DEPS],
    "//conditions:default": ["@com_github_istio_api" + d for d in ISTIO_DEPS]
	})
