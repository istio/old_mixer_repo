load("@io_bazel_rules_go//go:def.bzl", "go_library")

def _mixer_adapter_library_gen(name, packages, out):
  args = ""
  descriptors = []
  for k1, v in packages.items():
    l = "$(location %s)" % (k1)
    args += " %s:%s " % (l, v)
    descriptors.append(k1)

  native.genrule(
      name = name+"_gen",
      srcs = descriptors,
      outs = [out],
      cmd = "$(location //tools/codegen/cmd/mixgenadapter) " + args + " -o $(location %s)" % (out),
      tools = ["//tools/codegen/cmd/mixgenadapter"],
  )

DEPS_FOR_ALL_TMPLS = [
    "//pkg/adapter:go_default_library",
    "@com_github_gogo_protobuf//proto:go_default_library",
    "@com_github_golang_glog//:go_default_library",
    "@com_github_gogo_protobuf//types:go_default_library",
    "@com_github_istio_api//:mixer/v1/config/descriptor",  # keep
]

def mixer_adapter_library(name, packages, deps):
  _mixer_adapter_library_gen("adapter_file_gen", packages, "adapter.gen.go")

  go_library(
      name = name,
      srcs = ["adapter.gen.go"],
      deps = deps + DEPS_FOR_ALL_TMPLS,
  )
