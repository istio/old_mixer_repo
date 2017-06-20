package config

import (
	"io"

	"github.com/golang/protobuf/proto"
)

type (
	// Handler represents default functionality every Adapter must implement.
	Handler interface {
		io.Closer
		AdapterConfigValidatorAndConfigurer

		// Name returns the official name of the aspects produced by this builder.
		Name() string
		// Description returns a user-friendly description of the aspects produced by this builder.
		Description() string
	}

	// ConfigValidator handles adapter configuration defaults and validation.
	AdapterConfigValidatorAndConfigurer interface {
		// DefaultConfig returns a default configuration struct for this
		// adapter. This will be used by the configuration system to establish
		// the shape of the block of configuration state passed to the Configure method.
		DefaultConfig() proto.Message
		// ValidateConfig determines whether the given configuration meets all correctness requirements.
		ValidateConfig(proto.Message) error
		// Configure is invoked by Mixer to pass an instance of the default configuration to the adapter implementation.
		Configure(proto.Message) error
	}
)
