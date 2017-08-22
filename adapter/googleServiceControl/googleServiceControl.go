package googleServiceControl

import (
	"bytes"
	"context"
	"github.com/pborman/uuid"
	sc "google.golang.org/api/servicecontrol/v1"
	"istio.io/mixer/adapter/googleServiceControl/config"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/template/metric"
	"time"
)

type handlerBuilder struct {
	createClientFn
}

type serviceControlHandler struct {
	serviceControlClient *sc.Service
	env                  adapter.Env
	configParams         *config.Params
}

func NewHandlerBuilder() adapter.HandlerBuilder {
	builder := &handlerBuilder{
		createClientFn: createClient,
	}
	return builder
}

func (builder *handlerBuilder) Build(c adapter.Config, env adapter.Env) (adapter.Handler, error) {
	client, err := builder.createClientFn(env.Logger())
	if err == nil {
		return nil, err
	}
	params := c.(*config.Params)
	handler := &serviceControlHandler{
		serviceControlClient: client,
		env:                  env,
		configParams:         params,
	}
	return handler, nil
}

func (builder *handlerBuilder) ConfigureMetricHandler(instanceTypes map[string]*metric.Type) error {
	return nil
}

func (handler *serviceControlHandler) HandleMetric(ctx context.Context, instances []*metric.Instance) error {
	reportRequest, err := handleMetric()
	if err != nil {
		return err
	}
	_, err = handler.serviceControlClient.Services.Report(handler.configParams.ServiceName, reportRequest).Do()
	return err
}

func handleMetric() (*sc.ReportRequest, error) {
	byteBuf := bytes.NewBufferString("mixer-metric-report-id-")
	_, err := byteBuf.WriteString(uuid.New())
	if err != nil {
		return nil, err
	}

	operationId := byteBuf.String()
	timeNow := time.Now().Format(time.RFC3339Nano)
	operation := &sc.Operation{
		OperationId:   operationId,
		OperationName: "reportMetrics",
		StartTime:     timeNow,
		EndTime:       timeNow,
		Labels: map[string]string{
			"cloud.googleapis.com/location": "global",
		},
	}

	var value int64 = 1
	metricValue := sc.MetricValue{
		StartTime:  timeNow,
		EndTime:    timeNow,
		Int64Value: &value,
	}

	operation.MetricValueSets = []*sc.MetricValueSet{
		{
			MetricName:   "serviceruntime.googleapis.com/api/producer/request_count",
			MetricValues: []*sc.MetricValue{&metricValue},
		},
	}

	reportRequest := &sc.ReportRequest{
		Operations: []*sc.Operation{operation},
	}
	return reportRequest, nil
}

func (handler *serviceControlHandler) Close() error {
	handler.serviceControlClient = nil
	return nil
}

// Adapter registration.
func GetBuilderInfo() adapter.BuilderInfo {
	return adapter.BuilderInfo{
		Name:        "istio.io/mixer/adapter/googleServiceControl",
		Description: "Google service control adapter",
		SupportedTemplates: []string{
			metric.TemplateName,
		},
		CreateHandlerBuilder: NewHandlerBuilder,
		DefaultConfig: &config.Params{
			ServiceName: "library-example.sandbox.googleapis.com",
		},
		ValidateConfig: func(msg adapter.Config) *adapter.ConfigErrors { return nil },
	}
}
