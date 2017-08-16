// Copyright (c) 2017 IBM Corp. Licensed Materials - Property of IBM.

package token

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.ibm.com/hrl-microservices/clearharbor/token"
	"github.ibm.com/hrl-microservices/clearharbor/token/parsers"
)

const (
	apiPortEnv          = "API_PORT"
	disableAccessLogEnv = "DISABLE_ACCESS_LOG"
	claimsEnvVarName    = "TOKEN_CLAIMS"
	claimsFormatEnv     = "TOKEN_CLAIMS_FORMAT"
	pubKeysIntervalEnv  = "PUBKEY_INTERVAL"
	defaultPort         = 8080
	minPubkeyInterval   = 30 * time.Second
)

// Config encapsulates REST server configuration parameters
type Config struct { // structure should not be marshaled to JSON, not even using defaults
	Issuers          map[string]token.Issuer //supported issuers. subset of the global issuer pool
	HTTPAddressSpec  string                  `json:"-"`
	DisableAccessLog bool                    `json:"-"`
	ClaimsFormat     string                  `json:"-"`
	Claims           []string                `json:"-"`
	PubKeysInterval  time.Duration
	parser           parsers.JWTTokenParser //currently, only jwt supported
}

// NewConfig creates a configuration object with default values
func NewConfig() (*Config, error) {
	cfg := &Config{
		Issuers:         token.IssuersPool,
		HTTPAddressSpec: fmt.Sprintf(":%d", defaultPort),
		parser:          &parsers.MockJWTParser{},
	}

	portstr := os.Getenv(apiPortEnv)
	if portstr != "" {
		port, err := strconv.Atoi(portstr)
		if err != nil {
			return nil, fmt.Errorf("Invalid API port: %s", err)
		}

		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("Invalid API port number: %d", port)
		}
		cfg.HTTPAddressSpec = fmt.Sprintf(":%d", port)
	}

	disableLogStr := os.Getenv(disableAccessLogEnv)
	if disableLogStr != "" {
		disableLog, err := strconv.ParseBool(disableLogStr)
		if err != nil {
			return nil, fmt.Errorf("Invalid DISABLE_ACCESS_LOG parameter: %s", err)
		}
		cfg.DisableAccessLog = disableLog
	}

	cfg.ClaimsFormat = os.Getenv(claimsFormatEnv)
	if cfg.ClaimsFormat != "" {
		_, err := template.New(claimsFormatEnv).Parse(cfg.ClaimsFormat)
		if err != nil {
			return nil, fmt.Errorf("Invalid format for %s: %s - %s", claimsFormatEnv, cfg.ClaimsFormat, err)
		}
	}

	cfg.Claims = make([]string, 0)
	if claims, exists := os.LookupEnv(claimsEnvVarName); exists {
		cfg.Claims = strings.FieldsFunc(claims, func(c rune) bool { return c == ',' }) // comma delimited values
	}

	intervalstr := os.Getenv(pubKeysIntervalEnv)
	if intervalstr != "" {
		interval, err := time.ParseDuration(intervalstr)
		if err != nil {
			return nil, fmt.Errorf("Invalid PUBKEY_INTERVAL: %s", err)
		}
		if interval < minPubkeyInterval { //is issuer independant, the minimum interval time is global for all issuers.
			interval = minPubkeyInterval
		}
		cfg.PubKeysInterval = interval
	} else {
		cfg.PubKeysInterval = minPubkeyInterval
	}

	return cfg, nil
}
