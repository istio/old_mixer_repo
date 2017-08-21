// Copyright (c) 2017 IBM Corp. Licensed Materials - Property of IBM.

package token

import (
	"crypto"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/asaskevich/govalidator"
)

/*
Issuer : Represents a token issuer, that issues tokens and provides public keys for validation of said tokens.
The tokens issued can be of any kind, as long as there are available token parsers for that type (see type TokenParser)
*/

type Issuer interface {
	//GetName : get the issuer's name
	GetName() string
	//GetPublicKey : get an issuer's public key by the key id. if no such key exists, KeyDoesNotExist error returned.
	GetPublicKey(kid string) (crypto.PublicKey, error)
	//UpdatePublicKeys : fetch once and update the public key cache of the issuer.
	UpdatePublicKeys() error
}

//Error when requesting an issuer for a key that does not exist
type KeyDoesNotExist struct{}

func (k KeyDoesNotExist) Error() string {
	return "Key does not exist"
}

//MockIssuer : a mock issuer that can be used for testing. as a testing issuer, it allows explicit control over it's key cache via SetPublicKey.
type MockIssuer struct {
	pubKeys map[string]crypto.PublicKey
}

func (m *MockIssuer) GetName() string { return "Mock" }
func (m *MockIssuer) GetPublicKey(kid string) (crypto.PublicKey, error) {
	if key, ok := m.pubKeys[kid]; ok {
		return key, nil
	}
	return nil, KeyDoesNotExist{}
}
func (m *MockIssuer) UpdateKeys() error { return nil }
func (m *MockIssuer) SetPublicKey(kid string, key crypto.PublicKey) {
	if m.pubKeys == nil {
		m.pubKeys = make(map[string]crypto.PublicKey)
	}
	m.pubKeys[kid] = key
}

//The default jwt token issuer, that maintains key according to the JOSE standard (jwk).
type defaultJWTIssuer struct {
	name         string
	pubKeysURL   string
	pubKeys      map[string]crypto.PublicKey // (kid -> public key)
	pubKeysTime  time.Time
	sync.RWMutex //sync accesses to key pools
}

//GetName : get the issuer's name
func (iss *defaultJWTIssuer) GetName() string {
	return iss.name
}

//GetPublicKey : get an issuer's public by the key id. if no such key exists, KeyDoesNotExist error returned.
func (iss *defaultJWTIssuer) GetPublicKey(kid string) (crypto.PublicKey, error) {
	iss.RLock()
	defer iss.RUnlock()
	if key := iss.pubKeys[kid]; key != nil {
		return key, nil
	}
	return nil, KeyDoesNotExist{}
}

//UpdateKeys : update the issuer's public key cache
func (iss *defaultJWTIssuer) UpdatePublicKeys() error {
	if iss.pubKeysURL == "" {
		return errors.New("Url for public keys for IAM must be provided")
	}
	if !govalidator.IsURL(iss.pubKeysURL) {
		return errors.New("Public keys url for issuer " + iss.GetName() + " IAM is not valid")
	}

	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Get(iss.pubKeysURL)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Failed to retrieve the public keys from %s with status: %d(%s)",
			iss.pubKeysURL, resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var keys []key
	var ks keySet

	if err := json.Unmarshal(body, &ks); err == nil { // an RFC compliant JWK Set object, extract key array
		keys = ks.Keys
	} else if err := json.Unmarshal(body, &keys); err != nil { // attempt to decode as JWK array directly
		return err
	}

	mkeys := make(map[string]crypto.PublicKey)
	for i, k := range keys {
		if k.Kid == "" {
			return fmt.Errorf("Failed to parse the public key %d: kid is missing", i)
		}

		pubkey, err := k.decodePublicKey()
		if err != nil {
			return fmt.Errorf("Failed to parse the public key %d: %s", i, err)
		}
		mkeys[k.Kid] = pubkey
	}

	iss.Lock() // lock RW lock ot update key cache
	defer iss.Unlock()

	iss.pubKeysTime = time.Now()
	if len(iss.pubKeys) != len(mkeys) || !reflect.DeepEqual(iss.pubKeys, mkeys) {
		iss.pubKeys = mkeys //updating key cache
	}

	return nil
}
