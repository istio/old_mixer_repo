package token

//sample file to experiment with the istio mixer features
import (
	"errors"
	"regexp"
	"sync"
	"time"
	"github.com/golang/glog"
	"istio.io/mixer/adapter/token/config"
	"istio.io/mixer/pkg/adapter"
	"strings"
	"github.com/dgrijalva/jwt-go"
	"fmt"
	"reflect"
)

const (
	authHeader_key = "authorizationHeader"
	exists_key = "request.token.exists"
	encrypted_key = "request.token.encrypted"
	type_key = "request.token.type"
	valid_key = "request.token.valid"
	signed_key = "request.token.signed"
	signAlg_key = "request.token.signAlg"
	claims_key = "request.token.claims"
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
		//sync all key fetching
		pubkeySync sync.WaitGroup
		//sync server resources
		sync.RWMutex
	}
	tokenMetaData struct{
		encrypted bool
		ttype string
		signed bool
		signAlg string
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
	config, err := NewTokenConfig(cfg)
	if err != nil {
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
							if err := issuer.UpdatePublicKeys(); err != nil {
								glog.Warning("Error fetching public keys: " + err.Error())
							}
						}
					}
				}
			}(tag.pubkeyTerminator, issuer))
	}

	return tag, nil
}

func (tag tokenAttrGen) Generate(inputAttributes map[string]interface{}) (map[string]interface{}, error) {
	tokenAttrs := make(map[string]interface{})

	//default values:
	tokenAttrs[exists_key]=false
	tokenAttrs[encrypted_key]=false
	tokenAttrs[type_key]=""
	tokenAttrs[valid_key]=false
	tokenAttrs[signed_key]=false
	tokenAttrs[signAlg_key]=""
	tokenAttrs[claims_key]=make(map[string]string) //attribute ValueType STRING_MAP

	authHeader, exists := inputAttributes[authHeader_key]
	tag.env.Logger().Warningf("auth: %v",authHeader)
	if !exists || authHeader==nil || authHeader=="" {
		tag.env.Logger().Infof("Request with no Authorization header made")
		return tokenAttrs, nil
	}
	tokenAttrs[exists_key] = true
	tokenAttrs[type_key] = "Unknown"
	rawToken, err := extractTokenFromAuthHeader(authHeader.(string))
	if err != nil {
		tag.env.Logger().Infof("Request with malformed Authorization header made")
		return tokenAttrs, nil
	}

	metaData := &tokenMetaData{ttype:"Unknown",signAlg:"none",signed:false,encrypted:false}

	//currently only JWT supported:
	parser := &defaultJWTTokenParser{tag.cfg.Issuers}

	parsedToken, err := parser.Parse(rawToken,metaData)
	if err != nil{
		tag.env.Logger().Infof("Failed validating token: %v , reason: %v",rawToken,err.Error())
	} else {
		//convert to jwt.Token and use the Header iss to get claim renames
		tokenAttrs[valid_key] = parsedToken.(*jwt.Token).Valid
		if tokenAttrs[valid_key].(bool) {
			//extract claims:
			tokenClaims := parsedToken.(*jwt.Token).Claims.(jwt.MapClaims)
			//token is valid, meaning it has an iss claim that contains a name of a supported issuer
			issName := tokenClaims["iss"].(string)
			claimNames := tag.cfg.Issuers[issName].GetClaimNames()
			if claimNames != nil && len(claimNames)>0 {
				for _,name := range claimNames{
					hierarchyKeys := strings.Split(name,".")
					var nextHeirarchyValue map[string]interface{}
					for i,nextKey := range hierarchyKeys {
						var val interface{}
						var exists bool
						if i == 0 {
							val,exists = tokenClaims[nextKey]
						} else {
							val,exists = nextHeirarchyValue[nextKey]
						}
						if !exists{
							tag.env.Logger().Infof("requested claim name: %v, which wasn't provided with the given token",name)
							break
						}
						if reflect.ValueOf(val).Kind() == reflect.Map && reflect.TypeOf(val).Key() == reflect.TypeOf("string") {
							nextHeirarchyValue = val.(map[string]interface{})
						} else if i < len(hierarchyKeys)-1 { //still more nested keys remain, but next value is not a map with string key types - cannot nest further
							tag.env.Logger().Infof("requested claim name: %v, which wasn't provided with the given token",name)
							break
						}
						if i == len(hierarchyKeys)-1{
							tokenAttrs[claims_key].(map[string]string)[name] = fmt.Sprint(val)
						}
					}

				}
			}


			//claim renamings:
			if tokenAttrs != nil {
				claimRenames := tag.cfg.Issuers[issName].GetClaimRenames()
				for k,v := range claimRenames {
					if _, exists := tokenAttrs[claims_key].(map[string]string)[k] ; !exists{
						tag.env.Logger().Infof("requested to rename claim named: %v, to: %v, but no such claim requested or exists in the token",k,v)

					}
					temp := tokenAttrs[claims_key].(map[string]string)[k]
					delete(tokenAttrs[claims_key].(map[string]string),k)
					tokenAttrs[claims_key].(map[string]string)[v] = temp
				}
			}
		}
	}
	tokenAttrs[encrypted_key] = metaData.encrypted
	tokenAttrs[signed_key] = metaData.signed
	if tokenAttrs[signed_key].(bool) {
		tokenAttrs[signAlg_key] = metaData.signAlg
	}
	tokenAttrs[type_key] = metaData.ttype

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
					if err := issuer.UpdatePublicKeys(); err != nil {
						glog.Warningf("Error fetching public keys: " + err.Error())
					}
					wg.Done()
				}
			}(issuer))
	}
	wg.Wait()
}

func extractTokenFromAuthHeader(authHeader string) (string,error) {
	parts := strings.SplitN(authHeader, " ", 3)
	if len(parts) != 2 || (parts[0] != "Bearer" && parts[0] != "bearer") {
		return "", fmt.Errorf("Invalid authorization header. expected \"bearer <token>\" or \"Bearer <token>\"")
	}

	return parts[1], nil
}
