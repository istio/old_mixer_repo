load("@io_bazel_rules_go//go:def.bzl", "go_library")

def _handler_config_gen(name, packages, out):
  args = ""
  descriptors = []
  for k1, v in packages.items():
    l = "$(location %s)" % (k1)
    args += " %s:%s " % (l, v)
    descriptors.append(k1)

  native.genrule(
      name = name+"_gen",
      outs = [out],
      cmd = "$(location //tools/codegen/cmd/mixgenbootstrap) " + args + " --out_handlerconfig $(location %s)" % (out),
      tools = ["//tools/codegen/cmd/mixgenbootstrap"] + descriptors,
  )

DEPS = [
    "//pkg/adapter:go_default_library",
]

def handler_config_library(name, packages, deps):
  _handler_config_gen("handler_config_file_gen", packages, "handlerConfig.gen.go")

  go_library(
      name = name,
      srcs = ["handlerConfig.gen.go"],
      deps = deps + DEPS,
  )

