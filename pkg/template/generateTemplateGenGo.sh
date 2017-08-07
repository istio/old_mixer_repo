MIXERPATH=$GOPATH/src/istio.io/mixer
pushd $MIXERPATH

bazel build template/...
bazel build tools/...

bazel-bin/tools/codegen/cmd/mixgenbootstrap/mixgenbootstrap \
bazel-genfiles/template/sample/report/go_default_library_proto.descriptor_set:istio.io/mixer/template/sample/report \
bazel-genfiles/template/sample/check/go_default_library_proto.descriptor_set:istio.io/mixer/template/sample/check \
bazel-genfiles/template/sample/quota/go_default_library_proto.descriptor_set:istio.io/mixer/template/sample/quota \
bazel-genfiles/template/listEntry/go_default_library_proto.descriptor_set:istio.io/mixer/template/listEntry \
bazel-genfiles/template/logEntry/go_default_library_proto.descriptor_set:istio.io/mixer/template/logEntry \
bazel-genfiles/template/metric/go_default_library_proto.descriptor_set:istio.io/mixer/template/metric \
bazel-genfiles/template/quota/go_default_library_proto.descriptor_set:istio.io/mixer/template/quota \
bazel-genfiles/template/reportNothing/go_default_library_proto.descriptor_set:istio.io/mixer/template/reportNothing \
bazel-genfiles/template/checkNothing/go_default_library_proto.descriptor_set:istio.io/mixer/template/checkNothing \
-o $MIXERPATH/pkg/template/template.gen.go

bazel build ...
