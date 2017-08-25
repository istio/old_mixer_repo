package log

import (
	"context"

	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/template/logentry"
)

type (
	builder struct {
	}

	handler struct {
	}
)

var (
	_ logentry.HandlerBuilder = &builder{}
	_ logentry.Handler        = &handler{}
)

func NewBuilder() logentry.HandlerBuilder {
	return &builder{}
}

func (b *builder) ConfigureLogEntryHandler(entries map[string]*logentry.Type) error {
	return nil
}

func (b *builder) Build(c adapter.Config, env adapter.Env) (adapter.Handler, error) {
	return &handler{}, nil
}

func (h *handler) HandleLogEntry(ctx context.Context, values []*logentry.Instance) error {
	return nil
}

func (h *handler) Close() error {
	return nil
}
