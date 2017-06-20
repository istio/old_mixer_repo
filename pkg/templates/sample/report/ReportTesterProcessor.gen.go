package istio_mixer_adapter_sample_report

import "istio.io/mixer/pkg/adapter/config"

const TemplateName = "istio.mixer.adapter.sample.report.Sample"

// Instance represent the runtime structure that will be passed to the ReportSample method in the handlers.
type Instance struct {
	Name       string
	Value      interface{}
	Dimensions map[string]interface{}
}

// SampleProcessor represent the Go interface that handlers must implement if it wants to process the template
// named `istio.mixer.adapter.sample.report.Sample`
type SampleProcessor interface {
	config.Handler
	ConfigureSample(map[string]*Type /*Constructor:instance_name to Type mapping. Note type name will not be passed at all*/) error
	ReportSample([]*Instance /*The type is inferred from the Instance.name and the mapping of instance to types passed during the config time*/) error
}
