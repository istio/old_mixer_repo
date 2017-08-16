package token

//sample file to experiment with the istio mixer features
import(
	"istio.io/mixer/pkg/adapter"
	"fmt"
	"github.com/golang/glog"
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
	env.Logger().Warningf("indigoblue warning!")
	fmt.Println("indigored warning from fmt!")
	return &aspect{}, nil
}

func (aspect) Generate(inputAttributes map[string]interface{}) (map[string]interface{}, error) {
	tokenAttrs := make(map[string]interface{})
	tokenAttrs["sample_attr"] = "indigoBlue"
	glog.Info("indigoblue warning from glog!")
	return tokenAttrs, nil
}

func (aspect) Close() error {
	fmt.Println("closed the sample adapter")
	return nil
}
