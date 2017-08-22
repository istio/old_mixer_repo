package svcctrl

import (
	"bytes"
	"context"
	"time"

	"github.com/pborman/uuid"

	sc "google.golang.org/api/servicecontrol/v1"

	"istio.io/mixer/adapter/svcctrl/config"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/template/metric"
)

type builder struct {
	createClientFn
}

type handler struct {
	serviceControlClient *sc.Service
	env                  adapter.Env
	configParams         *config.Params
}

func (b *builder) Build(c adapter.Config, env adapter.Env) (adapter.Handler, error) {
	client, err := b.createClientFn(env.Logger())
	if err == nil {
		return nil, err
	}
	params := c.(*config.Params)
	handler := &handler{
		serviceControlClient: client,
		env:                  env,
		configParams:         params,
	}
	return handler, nil
}

func (b *builder) ConfigureMetricHandler(instanceTypes map[string]*metric.Type) error {
	return nil
}

func (h *handler) HandleMetric(ctx context.Context, instances []*metric.Instance) error {
	buf := bytes.NewBufferString("mixer-metric-report-id-")
	_, err := buf.WriteString(uuid.New())
	if err != nil {
		return err
	}

	opId := buf.String()
	reportReq, err := handleMetric(time.Now().Format(time.RFC3339Nano), opId)
	if err != nil {
		return err
	}
	_, err = h.serviceControlClient.Services.Report(h.configParams.ServiceName, reportReq).Do()
	return err
}

func handleMetric(timeNow, opId string) (*sc.ReportRequest, error) {
	op := &sc.Operation{
		OperationId:   opId,
		OperationName: "reportMetrics",
		StartTime:     timeNow,
		EndTime:       timeNow,
		Labels: map[string]string{
			"cloud.googleapis.com/location": "global",
		},
	}

	value :=int64(1)
	metricValue := sc.MetricValue{
		StartTime:  timeNow,
		EndTime:    timeNow,
		Int64Value: &value,
	}

	op.MetricValueSets = []*sc.MetricValueSet{
		{
			MetricName:   "serviceruntime.googleapis.com/api/producer/request_count",
			MetricValues: []*sc.MetricValue{&metricValue},
		},
	}

	reportReq := &sc.ReportRequest{
		Operations: []*sc.Operation{op},
	}
	return reportReq, nil
}

func (h *handler) Close() error {
	h.serviceControlClient = nil
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
		CreateHandlerBuilder: func() adapter.HandlerBuilder {
			return &builder{
				createClientFn: createClient,
			}
		},
		DefaultConfig: &config.Params{
			ServiceName: "library-example.sandbox.googleapis.com",
		},
		ValidateConfig: func(msg adapter.Config) *adapter.ConfigErrors { return nil },
	}
}
