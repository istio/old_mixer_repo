package token

//sample file to experiment with the istio mixer features
import (
	"fmt"
	"regexp"
	"github.com/asaskevich/govalidator"
	"istio.io/mixer/adapter/token/config"
	"istio.io/mixer/pkg/adapter"
)

type (
	// Builder implements all adapter.*Builder interfaces
	builder struct{ adapter.DefaultBuilder }
	aspect  struct{}
)

var (
	name = "token"
	desc = "token validation adapter"
	conf = &config.Params{}
)

func newBuilder() *builder {
	return &builder{
		adapter.NewDefaultBuilder(name, desc, conf),
	}
}

func (b *builder) Close() error {
	return nil
}

func validIssuerName(name string) bool {
	return regexp.MustCompile("^[a-zA-z][a-zA-Z0-9_/:.-]+$").MatchString(name) //starts with a letter, and contains only URL characters
}

func (*builder) ValidateConfig(c adapter.Config) (ce *adapter.ConfigErrors) {
	params := c.(*config.Params)
	if len(params.Issuers) == 0 {
		return // no issuers = default config
	}
	issuers := params.Issuers
	for _, issuer := range issuers {
		if issuer == nil {
			ce = ce.Appendf("Issuer", "is nil")
		} else {
			if len(issuer.Name) == 0 {
				ce = ce.Appendf("Issuer.Name", "field must be populated")
			} else if !validIssuerName(issuer.Name) {
				ce = ce.Appendf("Issuer.Name", "contains invalid characters")
			}
			fmt.Println(issuer.PubKeyUrl)
			if len(issuer.PubKeyUrl) == 0 {
				ce = ce.Appendf("Issuer.Url", "field must be populated")
			} else if !govalidator.IsURL(issuer.PubKeyUrl) {
				ce = ce.Appendf("Issuer.Url", "invalid: "+issuer.PubKeyUrl)
			}
			if issuer.ClaimNames != nil {
				duplicateCatcher := make(map[string]bool)
				for _, name := range issuer.ClaimNames {
					if _, exists := duplicateCatcher[name]; !exists {
						duplicateCatcher[name] = true
					} else {
						ce = ce.Appendf("Issuer.ClaimNames", "contains duplicates")
						break
					}
				}
			}
		}
	}
	return
}

// Register registers the no-op adapter as every aspect.
func Register(r adapter.Registrar) {
	r.RegisterAttributesGeneratorBuilder(newBuilder())
}

// BuildAttributesGenerator creates an adapter.AttributesGenerator instance
func (builder) BuildAttributesGenerator(env adapter.Env, cfg adapter.Config) (adapter.AttributesGenerator, error) {
	return &aspect{}, nil
}

func (aspect) Generate(inputAttributes map[string]interface{}) (map[string]interface{}, error) {
	tokenAttrs := make(map[string]interface{})
	return tokenAttrs, nil
}

func (aspect) Close() error {
	return nil
}
