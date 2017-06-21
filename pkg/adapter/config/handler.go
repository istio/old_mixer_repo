package config

import (
	"io"

	"github.com/golang/protobuf/proto"
)

type (
	// Handler represents default functionality every Adapter must implement.
	Handler interface {
		io.Closer
	}

	// Builder represents a factory of Handler. Adapters register builders with Mixer
	// in order to allow Mixer to instantiate Handler on demand.
	Builder interface {
		io.Closer
		AdapterConfigValidator

		// Name returns the official name of the adapter produced by this builder.
		Name() string
		// Description returns a user-friendly description of the adapter produced by this builder.
		Description() string
	}

	// AdapterConfigValidator handles adapter configuration defaults and validation.
	AdapterConfigValidator interface {
		// DefaultConfig returns a default configuration struct for this
		// adapter. This will be used by the configuration system to establish
		// the shape of the block of configuration state passed to the Configure method.
		DefaultConfig() proto.Message
		// ValidateConfig determines whether the given configuration meets all correctness requirements.
		ValidateConfig(proto.Message) error
	}
)
