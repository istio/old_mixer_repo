package token

//sample file to experiment with the istio mixer features
import (
	"regexp"
	"sync"
	"istio.io/mixer/adapter/token/config"
	"istio.io/mixer/pkg/adapter"
	"errors"
	"time"
	"github.com/golang/glog"
)

type (
	// Builder implements all adapter.*Builder interfaces
	tokenBuilder struct{ adapter.DefaultBuilder }
	tokenAttrGen struct {
		cfg *TokenConfig
		//adapter environment
		env adapter.Env
		//controls termination of key updating threads of all issuers supported by the server.
		pubkeyTerminator chan int
		//sync all key fetching threads
		pubkeySync sync.WaitGroup
		//sync server resources
		sync.RWMutex
	}
)

var (
	name = "token"
	desc = "token attribute generating adapter"
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
	config,err := NewTokenConfig(cfg)
	if err != nil{
		return nil, errors.New("Failed generating token configuration")
	}

	tag := &tokenAttrGen{env: env, cfg: config, pubkeyTerminator: make(chan int)}

	// initial retrieval of public keys from all supported issuers
	tag.fetchPublicKeys()

	//periodically refresh public keys until channel termination:
	for _, issuer := range tag.cfg.Issuers {
		tag.pubkeySync.Add(1)
		localTicker := time.NewTicker(tag.cfg.PubKeysInterval)
		env.ScheduleDaemon(
				func(terminator chan int, issuer Issuer) func() {
					return func() {
						for {
							select {
							case _, ok := <-tag.pubkeyTerminator:
								if !ok {
									tag.pubkeySync.Done()
									localTicker.Stop()
									return
								}
							case <-localTicker.C:
								if err := issuer.UpdateKeys(); err != nil {
									glog.Warning("Error fetching public keys: " + err.Error())
								}
							}
						}
					}
				}(tag.pubkeyTerminator, issuer)	)
	}
	return tag, nil
}

func (tokenAttrGen) Generate(inputAttributes map[string]interface{}) (map[string]interface{}, error) {
	tokenAttrs := make(map[string]interface{})
	return tokenAttrs, nil
}

func (tag *tokenAttrGen) Close() error {
	glog.Info("Shutting down the token attribute generator")
	//terminate all key fetching
	close(tag.pubkeyTerminator)
	//wait for all key fetching threads to terminate
	tag.pubkeySync.Wait()
	return nil
}

//fetch the public keys of all supported issuers.
func (tag *tokenAttrGen) fetchPublicKeys() {
	var wg sync.WaitGroup
	for _, issuer := range tag.cfg.Issuers {
		wg.Add(1)
		tag.env.ScheduleWork(
			func(issuer Issuer) func() {
				return func() {
					if err := issuer.UpdateKeys(); err != nil {
						glog.Warningf("Error fetching public keys: " + err.Error())
					}
					wg.Done()
				}
			}(issuer))
	}
	wg.Wait()
}
