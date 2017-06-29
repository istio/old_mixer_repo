package template

import (
	"reflect"
	"testing"

	sample_report "istio.io/mixer/pkg/template/sample/report"
)

func TestGetTemplateInfo(t *testing.T) {
	for _, tst := range []struct {
		template string
		expected Info
		present  bool
	}{
		{sample_report.TemplateName, Info{&sample_report.ConstructorParam{}, inferTypeForSampleReport, configureTypeForSampleReport}, true},
		{"unknown template", Info{}, false},
	} {
		t.Run(tst.template, func(t *testing.T) {
			tdf := templateRepo{}
			k, rpresent := tdf.GetTemplateInfo(tst.template)
			if rpresent != tst.present ||
				!reflect.DeepEqual(k.CnstrDefConfig, tst.expected.CnstrDefConfig) ||
				!reflect.DeepEqual(reflect.TypeOf(k.InferTypeFn), reflect.TypeOf(tst.expected.InferTypeFn)) {
				t.Errorf("got GetConstructorDefaultConfig(%s) = {%v,%v,%v}, want {%v,%v,%v}", tst.template, k.CnstrDefConfig, reflect.TypeOf(k.InferTypeFn), rpresent,
					tst.expected.CnstrDefConfig, reflect.TypeOf(tst.expected.InferTypeFn), tst.present)
			}
		})
	}
}
