pushd $GOPATH/src/istio.io > /dev/null;

OUTPARAMS="Mgoogle/protobuf/duration.proto=github.com/golang/protobuf/ptypes/duration,Mmixer/v1/config/descriptor/value_type.proto=istio.io/api/mixer/v1/config/descriptor,Mgoogle/protobuf/descriptor.proto=github.com/golang/protobuf/protoc-gen-go/descriptor,Mgoogle/protobuf/struct.proto=github.com/golang/protobuf/ptypes/struct,Mmixer/pkg/templates/mixer/TemplateExtensions.proto=istio.io/mixer/pkg/templates/mixer:."

## RUN PROTOC ON ALL THE TEMPLATES
protoc mixer/pkg/templates/mixer/*.proto -I=. -I=api --go_out=${OUTPARAMS}
protoc mixer/pkg/templates/sample/report/ReportTesterTemplate.gen.proto mixer/pkg/templates/sample/report/ReportTesterTemplate.proto -I=. -I=api --go_out=${OUTPARAMS}

## SOME MAGIC TO MAKE COMPILER HAPPY
sed -i \
  -e 's/ValueType_VALUE_TYPE_UNSPECIFIED/VALUE_TYPE_UNSPECIFIED/g' \
  mixer/pkg/templates/sample/report/ReportTesterTemplate.gen.pb.go;

sed -i \
  -e 's/ValueType_VALUE_TYPE_UNSPECIFIED/VALUE_TYPE_UNSPECIFIED/g' \
  mixer/pkg/templates/sample/report/ReportTesterTemplate.pb.go;

## BUILD INDIVIDUAL GENERATED PROCESSORS

DIRS="sample/report"

for pkgdir in ${DIRS}; do
    pushd mixer/pkg/templates/${pkgdir} > /dev/null; \
    go build
    popd > /dev/null;
done

popd > /dev/null;

echo All generated protos and interface Go code builds.. Yay

