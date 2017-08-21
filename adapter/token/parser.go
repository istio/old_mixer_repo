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
	//Parse : parse a token in it's raw form, and outputs the parsed token
	Parse(rawToken interface{},parseArgs ...interface{}) (parsedToken interface{}, err error)
}

//A JWT token parser that works according to the JWT,JWS standards, and conforms to the TokenParser interface
type defaultJWTTokenParser struct{
	supportedIssuers map[string]Issuer
}
//given a raw jwt token (type string), returns the parsed token (type *jwt.Token)
func (j *defaultJWTTokenParser) Parse(rawToken interface{},parseArgs ...interface{}) (interface{}, error) {
	rt, ok := rawToken.(string)
	if !ok {
		return nil, fmt.Errorf("default JWT parser expects a raw token of type string, but recieved type %T instead",rawToken)
	}
	switch rt {
	case "":
		return nil, jwt.NewValidationError("Token is empty", ValidationErrorMissingAuthorizationHeader)
	default:
		token, err := jwt.Parse(rt, func(token *jwt.Token) (interface{}, error) {
			kid, ok := token.Header["kid"].(string)
			if kid == "" || !ok {
				return nil, fmt.Errorf("kid is missing")
			}
			iss, exists := token.Claims.(jwt.MapClaims)["iss"]
			if iss == "" || !exists {
				return nil, fmt.Errorf("iss claim is missing")
			}

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

		claims := token.Claims.(jwt.MapClaims) //MapClaims is the default claims type when using regular jwt-go Parse
		//handle validation errors:
		if !token.Valid {
			if ve, ok := err.(*jwt.ValidationError); ok {
				if ve.Errors&jwt.ValidationErrorMalformed != 0 {
					return nil, fmt.Errorf("Token malformed")
				} else if ve.Errors&(jwt.ValidationErrorExpired) != 0 {
					// Token expired
					return nil, fmt.Errorf("Token expired. token expirationTime: %v", claims["exp"])
				}
			}
		}
		//handle non-validation errors:
		if err != nil {
			return nil, err
		}

		return token, nil
	}
}


const (
	mockTokenValid    = "valid"
	mockTokenEmpty    = "empty"
	mockTokenExpired  = "expired"
	mockTokenMismatch = "too many parts"
)

//MockParser : a mock jwt token parser used for testing.
type mockJWTParser struct{}

func (mp *mockJWTParser) Parse(rawToken interface{},parseArgs ...interface{}) (interface{}, error) {
	switch rawToken {
	case mockTokenValid:
		return jwt.New(jwt.SigningMethodNone), nil
	case mockTokenEmpty:
		return nil, jwt.NewValidationError("Token is empty", ValidationErrorMissingAuthorizationHeader)
	case mockTokenExpired:
		return nil, jwt.NewValidationError("Token has expired", jwt.ValidationErrorExpired)
	case mockTokenMismatch:
		return nil, jwt.NewValidationError("Token mismtach", jwt.ValidationErrorMalformed)
	}
	return nil, jwt.NewValidationError("Token is undefined", jwt.ValidationErrorUnverifiable)
}

