package template

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/empty"

	pb "istio.io/api/mixer/v1/config/descriptor"
	sample_report "istio.io/mixer/pkg/template/sample/report"
)

func fillProto(cfg string, o interface{}) error {
	//data []byte, m map[string]interface{}, err error
	var m map[string]interface{}
	var data []byte
	var err error

	if err = yaml.Unmarshal([]byte(cfg), &m); err != nil {
		return err
	}

	if data, err = json.Marshal(m); err != nil {
		return err
	}

	err = yaml.Unmarshal(data, o)
	return err
}

func TestInferTypeForSampleReport(t *testing.T) {
	for _, tst := range []struct {
		name              string
		cnstrCnfg         string
		cnstrParamPb      interface{}
		typeEvalRet       pb.ValueType
		typeEvalError     error
		expectedValueType pb.ValueType
		expectedErr       string
	}{
		{
			name: "SimpleValid",
			cnstrCnfg: `
value: response.size
dimensions:
  source: source.ip
`,
			cnstrParamPb:      &sample_report.ConstructorParam{},
			typeEvalRet:       pb.INT64,
			typeEvalError:     nil,
			expectedValueType: pb.INT64,
			expectedErr:       "",
		},
		{
			name:         "NotValidConstructorParam",
			cnstrCnfg:    ``,
			cnstrParamPb: &empty.Empty{}, // cnstr type mismatch
			expectedErr:  "is not of type",
		},
		{
			name: "ErrorFromTypeEvaluator",
			cnstrCnfg: `
value: response.size
dimensions:
  source: source.ip
`,
			cnstrParamPb:  &sample_report.ConstructorParam{},
			typeEvalError: fmt.Errorf("some expression x.y.z is invalid"),
			expectedErr:   "some expression x.y.z is invalid",
		},
	} {
		t.Run(tst.name, func(t *testing.T) {
			cp := tst.cnstrParamPb
			_ = fillProto(tst.cnstrCnfg, cp)
			typeEvalFn := func(expr string) (pb.ValueType, error) { return tst.typeEvalRet, tst.typeEvalError }
			cv, cerr := inferTypeForSampleReport(cp.(proto.Message), typeEvalFn)

			if tst.expectedErr == "" {
				if cerr != nil {
					t.Errorf("got err %v\nwant <nil>", cerr)
				}
				if tst.expectedValueType != cv.(*sample_report.Type).Value {
					t.Errorf("got inferTypeForSampleReport(\n%s\n).value=%v\nwant %v", tst.cnstrCnfg, cv.(*sample_report.Type).Value, tst.expectedValueType)
				}
			} else {
				if cerr == nil || !strings.Contains(cerr.Error(), tst.expectedErr) {
					t.Errorf("got error %v\nwant %v", cerr, tst.expectedErr)
				}
			}
		})
	}
}
