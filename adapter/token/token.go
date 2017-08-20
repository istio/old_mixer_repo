package token

//sample file to experiment with the istio mixer features
import (
	"regexp"
	"istio.io/mixer/adapter/token/config"
	"istio.io/mixer/pkg/adapter"
)

type (
	// Builder implements all adapter.*Builder interfaces
	tokenBuilder struct{ adapter.DefaultBuilder }
	aspect  struct{}
)

var (
	name = "token"
	desc = "token validation adapter"
	conf = &config.Params{}
)

func newBuilder() *tokenBuilder {
	return &tokenBuilder{
		adapter.NewDefaultBuilder(name, desc, conf),
	}
}

func (b *tokenBuilder) Close() error {
	return nil
}

func validIssuerName(name string) bool {
	return regexp.MustCompile("^[a-zA-z][a-zA-Z0-9_/:.-]+$").MatchString(name) //starts with a letter, and contains only URL characters
}

// Register registers the no-op adapter as every aspect.
func Register(r adapter.Registrar) {
	r.RegisterAttributesGeneratorBuilder(newBuilder())
}

// BuildAttributesGenerator creates an adapter.AttributesGenerator instance
func (*tokenBuilder) BuildAttributesGenerator(env adapter.Env, cfg adapter.Config) (adapter.AttributesGenerator, error) {
	return &aspect{}, nil
}

func (aspect) Generate(inputAttributes map[string]interface{}) (map[string]interface{}, error) {
	tokenAttrs := make(map[string]interface{})
	return tokenAttrs, nil
}

func (aspect) Close() error {
	return nil
}
