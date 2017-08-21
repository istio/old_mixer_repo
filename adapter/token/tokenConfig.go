// Copyright (c) 2017 IBM Corp. Licensed Materials - Property of IBM.

package token

import (
	"crypto"
	"time"

	"github.com/asaskevich/govalidator"

	"istio.io/mixer/adapter/token/config"
	"istio.io/mixer/pkg/adapter"
)

const (
	minPubkeyInterval = 60 * time.Second
)

type TokenConfig struct { // structure should not be marshaled to JSON, not even using defaults
	Issuers         map[string]Issuer //supported issuers. tokens by other issuers will not be accepted.
	PubKeysInterval time.Duration
}

// NewConfig creates a configuration object with default values
func NewTokenConfig(c adapter.Config) (*TokenConfig, error) {
	cfg := &TokenConfig{
		Issuers:         map[string]Issuer{},
		PubKeysInterval: minPubkeyInterval,
	}
	params := c.(*config.Params)
	//extract issuers from config:
	for _, issuer := range params.Issuers {
		//currently only jwt is supported:
		iss := &defaultJWTIssuer{
			name:       issuer.Name,
			pubKeysURL: issuer.PubKeyUrl,
			pubKeys:    map[string]crypto.PublicKey{},
			claimNames: issuer.ClaimNames,
			claimRenames: issuer.ClaimRenames,
		}
		cfg.Issuers[iss.name] = iss
	}

	return cfg, nil
}

func (*tokenBuilder) ValidateConfig(c adapter.Config) (ce *adapter.ConfigErrors) {
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
