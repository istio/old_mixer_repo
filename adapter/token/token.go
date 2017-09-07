package token

//sample file to experiment with the istio mixer features
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/golang/glog"

	"istio.io/mixer/adapter/token/config"
	"istio.io/mixer/pkg/adapter"
)

const (
	authHeaderKey = "authorizationHeader"
	existsKey     = "request.token.exists"
	encryptedKey  = "request.token.encrypted"
	typeKey       = "request.token.type"
	validKey      = "request.token.valid"
	signedKey     = "request.token.signed"
	signAlgKey    = "request.token.signAlg"
	claimsKey     = "request.token.claims"
)

type (
	// Builder implements all adapter.*Builder interfaces
	tokenBuilder struct{ adapter.DefaultBuilder }
	tokenAttrGen struct {
		cfg *Config
		//adapter environment
		env adapter.Env
		//context of key updating threads of all issuers supported by the server.
		pubKeyContext context.Context
		//cancel function of public key context
		pubKeyCancel context.CancelFunc
		//sync all key fetching
		pubkeySync sync.WaitGroup
		//sync server resources
		sync.RWMutex
	}
	tokenMetaData struct {
		encrypted bool
		ttype     string
		signed    bool
		signAlg   string
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

// Register registers the no-op adapter as every aspect.
func Register(r adapter.Registrar) {
	r.RegisterAttributesGeneratorBuilder(newBuilder())
}

// BuildAttributesGenerator creates an adapter.AttributesGenerator instance
func (*tokenBuilder) BuildAttributesGenerator(env adapter.Env, cfg adapter.Config) (adapter.AttributesGenerator, error) {
	config, err := NewConfig(cfg)
	if err != nil {
		return nil, errors.New("Failed generating token configuration")
	}
	ctx, cancel := context.WithCancel(context.Background())
	ta := &tokenAttrGen{env: env, cfg: config, pubKeyContext: ctx, pubKeyCancel: cancel}

	ta.fetchPublicKeys()

	return ta, nil
}

func (ta tokenAttrGen) Generate(inputAttributes map[string]interface{}) (map[string]interface{}, error) {
	tokenAttrs := make(map[string]interface{})

	//default values:
	tokenAttrs[existsKey] = false
	tokenAttrs[encryptedKey] = false
	tokenAttrs[typeKey] = ""
	tokenAttrs[validKey] = false
	tokenAttrs[signedKey] = false
	tokenAttrs[signAlgKey] = ""
	tokenAttrs[claimsKey] = make(map[string]string) //attribute ValueType STRING_MAP

	authHeader, exists := inputAttributes[authHeaderKey]
	ta.env.Logger().Warningf("auth: %v", authHeader)
	if !exists || authHeader == nil || authHeader == "" {
		ta.env.Logger().Infof("Request with no Authorization header made")
		return tokenAttrs, nil
	}
	tokenAttrs[existsKey] = true
	tokenAttrs[typeKey] = "Unknown"
	rawToken, err := extractTokenFromAuthHeader(authHeader.(string))
	if err != nil {
		ta.env.Logger().Infof("Request with malformed Authorization header made")
		return tokenAttrs, nil
	}

	metaData := &tokenMetaData{ttype: "Unknown", signAlg: "none", signed: false, encrypted: false}

	//currently only JWT supported:
	parser := &defaultJWTTokenParser{ta.cfg.Issuers}

	parsedToken, err := parser.Parse(rawToken, metaData)
	if err != nil {
		ta.env.Logger().Infof("Failed validating token: %v , reason: %v", rawToken, err.Error())
	} else {
		//convert to jwt.Token and use the Header iss to get claim renames
		tokenAttrs[validKey] = parsedToken.(*jwt.Token).Valid
		if tokenAttrs[validKey].(bool) {

			//extract claims:
			tokenClaims := parsedToken.(*jwt.Token).Claims.(jwt.MapClaims)
			tokenAttrs[claimsKey] = ta.extractClaimsFromJWT(tokenClaims)

			//claim renamings:
			if tokenAttrs != nil {
				issName := tokenClaims["iss"].(string)
				claimRenames := ta.cfg.Issuers[issName].GetClaimRenames()
				for k, v := range claimRenames {
					if _, exists := tokenAttrs[claimsKey].(map[string]string)[k]; !exists {
						ta.env.Logger().Infof("requested to rename claim named: %v, to: %v, but no such claim requested or exists in the token", k, v)

					}
					temp := tokenAttrs[claimsKey].(map[string]string)[k]
					delete(tokenAttrs[claimsKey].(map[string]string), k)
					tokenAttrs[claimsKey].(map[string]string)[v] = temp
				}
			}

		}
	}
	tokenAttrs[encryptedKey] = metaData.encrypted
	tokenAttrs[signedKey] = metaData.signed
	if tokenAttrs[signedKey].(bool) {
		tokenAttrs[signAlgKey] = metaData.signAlg
	}
	tokenAttrs[typeKey] = metaData.ttype

	return tokenAttrs, nil
}

func (ta tokenAttrGen) extractClaimsFromJWT(tokenClaims jwt.MapClaims) map[string]string {
	claims := make(map[string]string)
	//token is valid, meaning it has an iss claim that contains a name of a supported issuer
	issName := tokenClaims["iss"].(string)
	claimNames := ta.cfg.Issuers[issName].GetClaimNames()
	if claimNames != nil && len(claimNames) > 0 {
		for _, name := range claimNames {
			hierarchyKeys := strings.Split(name, ".")
			var nextHeirarchyValue map[string]interface{}
			for i, nextKey := range hierarchyKeys {
				var val interface{}
				var exists bool
				if i == 0 {
					val, exists = tokenClaims[nextKey]
				} else {
					val, exists = nextHeirarchyValue[nextKey]
				}
				if !exists {
					ta.env.Logger().Infof("requested claim name: %v, which wasn't provided with the given token", name)
					break
				}
				if reflect.ValueOf(val).Kind() == reflect.Map && reflect.TypeOf(val).Key() == reflect.TypeOf("string") {
					nextHeirarchyValue = val.(map[string]interface{})
				} else if i < len(hierarchyKeys)-1 { //still more nested keys remain, but next value is not a map with string key types - cannot nest further
					ta.env.Logger().Infof("requested claim name: %v, which wasn't provided with the given token", name)
					break
				}
				if i == len(hierarchyKeys)-1 {
					var claimString string
					if reflect.TypeOf(val).Kind() == reflect.Map || reflect.TypeOf(val).Kind() == reflect.Slice || reflect.TypeOf(val).Kind() == reflect.Array {
						byt, _ := json.Marshal(val)
						claimString = string(byt)
					} else {
						claimString = fmt.Sprint(val)
					}
					claims[name] = claimString
				}
			}

		}
	}
	return claims
}

func (ta *tokenAttrGen) Close() error {
	glog.Info("Shutting down the token attribute generator")
	ta.pubKeyCancel()
	ta.pubkeySync.Wait()
	return nil
}

//fetch the public keys of all supported issuers, and initialize fetching deamons.
func (ta *tokenAttrGen) fetchPublicKeys() {
	//initial fetching
	var wg sync.WaitGroup
	for _, issuer := range ta.cfg.Issuers {
		wg.Add(1)
		ta.env.ScheduleWork(
			func(issuer Issuer) func() {
				return func() {
					if err := issuer.UpdatePublicKeys(); err != nil {
						glog.Warningf("Error fetching public keys: " + err.Error())
					}
					wg.Done()
				}
			}(issuer))
	}
	wg.Wait()

	//periodically refresh public keys until context termination:
	for _, issuer := range ta.cfg.Issuers {
		ta.pubkeySync.Add(1)
		localTicker := time.NewTicker(ta.cfg.PubKeysInterval)
		ta.env.ScheduleDaemon(
			func(ctx context.Context, issuer Issuer) func() {
				return func() {
					for {
						select {
						case _, ok := <-ctx.Done():
							if !ok {
								ta.pubkeySync.Done()
								localTicker.Stop()
								return
							}
						case <-localTicker.C:
							if err := issuer.UpdatePublicKeys(); err != nil {
								glog.Warning("Error fetching public keys: " + err.Error())
							}
						}
					}
				}
			}(ta.pubKeyContext, issuer))
	}

}

func extractTokenFromAuthHeader(authHeader string) (string, error) {
	parts := strings.SplitN(authHeader, " ", 3)
	if len(parts) != 2 || (parts[0] != "Bearer" && parts[0] != "bearer") {
		return "", fmt.Errorf("Invalid authorization header. expected \"bearer <token>\" or \"Bearer <token>\"")
	}

	return parts[1], nil
}
