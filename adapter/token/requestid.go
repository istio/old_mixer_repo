package token

import (
	"math/rand"
	"net/http"
	"time"
)

const (
	// RequestIDHeaderName defines the default expected header name
	RequestIDHeaderName = "Request-Id"

	letterBytes = "abcdef0123456789"

	letterIdxBits = 4                    // 4 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

var src = rand.NewSource(time.Now().UnixNano())

// GetRequestID extracts the request identifier from the request or generates one, if the header is missing
func GetRequestID(r *http.Request) (requestID string, headerName string) {
	headerName = RequestIDHeaderName
	requestID = r.Header.Get(headerName)

	if requestID == "" {
		headerName = "X-" + RequestIDHeaderName
		requestID = r.Header.Get(headerName)
	}
	if requestID == "" {
		headerName = RequestIDHeaderName
		requestID = randomString(16)
	}
	return requestID, headerName
}

func randomString(n int) string {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}
