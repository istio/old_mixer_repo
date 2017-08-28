package token

import (
	"fmt"
	"github.com/dgrijalva/jwt-go"
)

// @todo
// validationErrorMissingAuthorizationHeader value relies on jwt.ValidationErrorClaimsInvalid being the last error
// code in jwt package. Should probably use our own error wrapper type with specific redirect indication instead
const (
	_                                                = iota // ignore first value by assigning to blank identifier
	ValidationErrorMissingAuthorizationHeader uint32 = jwt.ValidationErrorClaimsInvalid << iota
)


//TokenParser : represents a generic token parser of any kind. changing
type TokenParser interface {
	//Parse : parse a token in it's raw form, and output the parsed token, while optionally filling the metadata about the token
	Parse(rawToken interface{},metaDataToFill *tokenMetaData,parseArgs ...interface{}) (parsedToken interface{}, err error)
}

//A JWT token parser that works according to the JWT,JWS standards, and conforms to the TokenParser interface
type defaultJWTTokenParser struct{
	supportedIssuers map[string]Issuer
}
//given a raw jwt token (type string), returns the parsed token (type *jwt.Token)
func (j *defaultJWTTokenParser) Parse(rawToken interface{},metaData *tokenMetaData,parseArgs ...interface{}) (interface{}, error) {
	rt, ok := rawToken.(string)
	if !ok {
		return nil, fmt.Errorf("default JWT parser expects a raw token of type string, but recieved type %T instead",rawToken)
	}
	switch rt {
	case "":
		return nil, jwt.NewValidationError("Token is empty", ValidationErrorMissingAuthorizationHeader)
	default:
		metaData.ttype = "jwt"
		token, err := jwt.Parse(rt, func(token *jwt.Token) (interface{}, error) {
			if token.Method != nil && token.Method.Alg() != "none" {//"none" alg according to RFC
				metaData.signed = true
			}
			if metaData.signed {
				metaData.signAlg = token.Method.Alg()
			}
			kid, ok := token.Header["kid"].(string)
			if kid == "" || !ok {
				return nil, fmt.Errorf("kid is missing")
			}
			iss, exists := token.Claims.(jwt.MapClaims)["iss"]
			if iss == "" || !exists {
				return nil, fmt.Errorf("iss claim is missing")
			}
			exp , exists := token.Claims.(jwt.MapClaims)["exp"]
			if !exists || exp == "" {
				return nil, fmt.Errorf("exp claim is missing")
			}
			//if exp does exist, it's time validation is made by the jwt lib

			relevantIssuer, exists := j.supportedIssuers[iss.(string)]
			if !exists {
				return nil, fmt.Errorf("%v is not a supported issuer", iss)
			}

			if _, ok := token.Method.(*jwt.SigningMethodRSA); ok {
				//asymmetric key signing
				pk, err := relevantIssuer.GetPublicKey(kid)
				if err != nil {
					return nil, err
				}

				return pk, nil
			}
			return nil, fmt.Errorf("Unsupported signing method: %v. RSA supported.", token.Header["alg"])
		})


		//handle validation errors:
		if ve, ok := err.(*jwt.ValidationError); ok {
			if ve.Errors&jwt.ValidationErrorMalformed != 0 {
				metaData.encrypted = true //assuming that an authorization header with bearer prefix but malformed token implies an encrypted token
				metaData.ttype = "Unknown"
				return nil, fmt.Errorf("Token malformed")
			} else if ve.Errors&(jwt.ValidationErrorExpired) != 0 {
				// Token expired
				return token, fmt.Errorf("Token expired.")
			}
		}

		//handle non-validation errors:
		if err != nil {
			return nil, err
		}

		return token, nil
	}
}

