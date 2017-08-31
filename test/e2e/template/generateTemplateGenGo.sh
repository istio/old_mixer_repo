MIXERPATH=$GOPATH/src/istio.io/mixer
pushd $MIXERPATH

bazel build test/e2e/template/report/...
bazel build tools/...

bazel-bin/tools/codegen/cmd/mixgenbootstrap/mixgenbootstrap \
bazel-genfiles/test/e2e/template/report/go_default_library_proto.descriptor_set:istio.io/mixer/test/e2e/template/report \
-o $MIXERPATH/test/e2e/template/template.gen.go

