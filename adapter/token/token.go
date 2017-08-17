package token

//sample file to experiment with the istio mixer features
import (
	"istio.io/mixer/adapter/token/config"
	"istio.io/mixer/pkg/adapter"
)

type (
	// Builder implements all adapter.*Builder interfaces
	Builder struct{ adapter.DefaultBuilder }
	aspect  struct{}
)

var (
	name = "token"
	desc = "token validation adapter"
	conf = &config.Params{}
)

// Register registers the no-op adapter as every aspect.
func Register(r adapter.Registrar) {
	b := Builder{adapter.NewDefaultBuilder(name, desc, conf)}
	r.RegisterAttributesGeneratorBuilder(b)
}

// BuildAttributesGenerator creates an adapter.AttributesGenerator instance
func (Builder) BuildAttributesGenerator(env adapter.Env, cfg adapter.Config) (adapter.AttributesGenerator, error) {
	return &aspect{}, nil
}

func (aspect) Generate(inputAttributes map[string]interface{}) (map[string]interface{}, error) {
	tokenAttrs := make(map[string]interface{})
	return tokenAttrs, nil
}

func (aspect) Close() error {
	return nil
}
