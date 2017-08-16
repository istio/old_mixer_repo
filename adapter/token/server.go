package token

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/pressly/chi"
	"github.ibm.com/hrl-microservices/clearharbor/token"
	"github.ibm.com/hrl-microservices/clearharbor/token/parsers"
)

const (
	validatePath              = "/validate"
	xAuthReqRedirect          = "X-AuthReq-Redirect"
	ingressClaimsHeader       = "X-Token-Claims"
	ingressFormatClaimsHeader = "X-Token-Format-Claims"
	authorizationheader       = "Authorization"
	claimHeaderPrefix         = "X-Token-Claim-"
)

// Server defines an interface for controlling server lifecycle
type Server interface {
	Start() error
	Shutdown(context.Context) error
}

type validationServer struct {
	cfg          *Config
	httpServer   *http.Server
	claimsFormat *template.Template
	//controls termination of key updating threads of all issuers supported by the server.
	pubkeyTerminator chan int
	//sync all key fetching threads
	pubkeySync sync.WaitGroup
	//sync server resources
	sync.RWMutex
}

// NewServer creates a new server based on the provided configuration options.
// Returns a valid Server interface on success or an error on failure
func NewValidationServer(config *Config) (*validationServer, error) {
	if config == nil {
		return nil, errors.New("null configuration provided")
	}
	if config.parser == nil {
		return nil, errors.New("Token parser must be provided")
	}

	if config.HTTPAddressSpec == "" {
		config.HTTPAddressSpec = fmt.Sprintf(":%d", defaultPort)
	}

	return &validationServer{cfg: config, pubkeyTerminator: make(chan int)}, nil
}

func (s *validationServer) Start() error {
	var err error

	if s.cfg.ClaimsFormat != "" {
		s.claimsFormat, err = template.New("clearharbor-claims").Parse(s.cfg.ClaimsFormat)
		if err != nil {
			return err
		}
	}

	s.setupHTTPRouting()
	log.Println("Starting the ClearHarbor service on ", s.cfg.HTTPAddressSpec)

	// initial retrieval of public keys
	s.fetchPublicKeys()

	//periodically refresh public keys until channel termination:
	for _, issuer := range s.cfg.Issuers {
		s.pubkeySync.Add(1)
		localTicker := time.NewTicker(s.cfg.PubKeysInterval)
		go func(terminator chan int, issuer token.Issuer) {
			for {
				select {
				case _, ok := <-s.pubkeyTerminator:
					if !ok {
						s.pubkeySync.Done()
						localTicker.Stop()
						return
					}
				case <-localTicker.C:
					if err := issuer.UpdateKeys(); err != nil {
						log.Println("Error fetching public keys: " + err.Error())
					}
				}
			}
		}(s.pubkeyTerminator, issuer)
	}

	return s.httpServer.ListenAndServe()

}

func (s *validationServer) Shutdown(ctx context.Context) error {
	log.Println("Shutting down the ClearHarbor service")

	//terminate all key fetching
	close(s.pubkeyTerminator)
	//wait for all key fetching threads to terminate
	s.pubkeySync.Wait()

	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}

	return nil
}

func (s *validationServer) setupHTTPRouting() {
	r := chi.NewRouter()
	if !s.cfg.DisableAccessLog {
		r.Use(logging)
	}

	r.Use(recoverer)
	r.Route("/", func(r chi.Router) {
		r.Get(validatePath, http.HandlerFunc(s.validate))
	})

	s.httpServer = &http.Server{
		Addr:         s.cfg.HTTPAddressSpec,
		ReadTimeout:  2 * time.Second, // reads from Ingress Proxy on localhost, so expect quick completion
		WriteTimeout: 4 * time.Second, // may need some more time to fetch expired keys from source
		Handler:      r,
	}
}

func (s *validationServer) fetchPublicKeys() {
	var wg sync.WaitGroup
	for _, issuer := range s.cfg.Issuers {
		wg.Add(1)
		go func(issuer token.Issuer) {
			if err := issuer.UpdateKeys(); err != nil {
				log.Println("Error fetching public keys: " + err.Error())
			}
			wg.Done()
		}(issuer)
	}
	wg.Wait()
}

func getAuthTokenFromRequest(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")

	if authHeader == "" { // missing auth
		return "", jwt.NewValidationError("No authorization header", parsers.ValidationErrorMissingAuthorizationHeader)
	}

	parts := strings.SplitN(authHeader, " ", 3)
	if len(parts) < 2 || (parts[0] != "Bearer" && parts[0] != "bearer") {
		return "", jwt.NewValidationError("Invalid authorization header", jwt.ValidationErrorMalformed)
	}

	return parts[1], nil
}

func (s *validationServer) validateAuthToken(r *http.Request) (*jwt.Token, error) {
	var resErr error
	resErr = jwt.NewValidationError("Could not verify", jwt.ValidationErrorUnverifiable)
	tokenString, tokenStringErr := getAuthTokenFromRequest(r)
	if tokenStringErr != nil {
		return nil, tokenStringErr
	}

	for i := 0; i < 2; i++ {
		parsedToken, err := s.cfg.parser.Parse(s.cfg.Issuers, tokenString)
		if err == nil { // token is valid. The user is authenticated
			return parsedToken, nil
		}
		//fmt.Println(err.Error())

		// See whether refreshing the public keys could help
		s.fetchPublicKeys()

		break
	}
	return nil, resErr
}

func response(w http.ResponseWriter, reqID string, headers map[string]string) func(int, map[string]string, interface{}) {
	return func(code int, additionalHeaders map[string]string, logContext interface{}) {
		if logContext != nil {
			log.Println("rid:", reqID, "-", http.StatusText(code), fmt.Sprintf("%v", logContext))
		} else {
			log.Println("rid:", reqID, "-", http.StatusText(code))
		}

		for k, v := range headers {
			w.Header().Set(k, v)
		}
		for k, v := range additionalHeaders {
			w.Header().Set(k, v)
		}

		w.WriteHeader(code)
		w.Write([]byte(http.StatusText(code)))
	}
}

func shouldRetry(_ *jwt.ValidationError) bool {
	// switch based on error type/behavior to redirect only in some cases?
	return true
}

func statusCodeHandler(status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	})
}

func (s *validationServer) insertHeadersFromToken(token *jwt.Token, w http.ResponseWriter, r *http.Request) {
	// handle header per claim
	requested := s.cfg.Claims
	if claimsHeader := r.Header.Get(ingressClaimsHeader); claimsHeader != "" { // per request overrides environment
		requested = strings.FieldsFunc(claimsHeader, func(c rune) bool { return c == ',' })
	}

	claims := token.Claims.(jwt.MapClaims)
	for _, name := range requested {
		t, err := template.New(name).Parse("{{." + name + "}}")
		if err != nil {
			continue
		}
		headerName := claimHeaderPrefix + strings.Replace(name, ".", "-", -1)
		value := bytes.NewBuffer([]byte{})
		if t.Execute(value, claims) != nil {
			continue
		}
		w.Header().Set(headerName, value.String())
	}

	// handle formatted claims as single header value
	claimsFormatHeader := r.Header.Get(ingressFormatClaimsHeader)
	if claimsFormatHeader == "" && s.claimsFormat == nil {
		return
	}

	value := bytes.NewBuffer([]byte{})
	if claimsFormatHeader != "" { // override
		t, err := template.New(claimsFormatHeader).Parse(claimsFormatHeader)
		if err != nil {
			log.Println(claimsFormatHeader, err.Error())
			return
		}
		if t.Execute(value, claims) != nil {
			return
		}
	} else {
		value := bytes.NewBuffer([]byte{})
		if s.claimsFormat.Execute(value, claims) != nil {
			return
		}
	}
	w.Header().Set(ingressFormatClaimsHeader, value.String())
}

func (s *validationServer) validate(w http.ResponseWriter, r *http.Request) {
	requestID, reqIDHeader := GetRequestID(r)
	reply := response(w, requestID, map[string]string{reqIDHeader: requestID})
	token, err := s.validateAuthToken(r)
	if err != nil {
		statusCode := http.StatusUnauthorized
		defaultScope := "openid"
		headers := map[string]string{
			"WWW-Authenticate": fmt.Sprintf("Bearer scope=\"%s\", error=\"%s\"", defaultScope, err.Error()),
		}
		failure, ok := err.(*jwt.ValidationError)
		if ok {
			headers[xAuthReqRedirect] = strconv.FormatBool(shouldRetry(failure))
		}
		reply(statusCode, headers, err)
		return
	}

	s.insertHeadersFromToken(token, w, r) // add claims headers
	reply(http.StatusOK, nil, nil)        // token is valid. The user is authenticated
}

/*
A middleware for error recovery
*/
func recoverer(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		requestID, reqIDHeader := GetRequestID(r)
		reply := response(w, requestID, map[string]string{reqIDHeader: requestID})
		defer func() {
			if reco := recover(); reco != nil {
				statusCode := http.StatusInternalServerError
				reply(statusCode, nil, reco)
			}
		}()
		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

/*
Retrieve a middleware for logging for the given validation server
*/
func logging(next http.Handler) http.Handler {
	loggingHandler := func(w http.ResponseWriter, r *http.Request) {
		var statusCode int
		requestID, _ := GetRequestID(r)
		wrr := httptest.NewRecorder()
		//copy headers
		wHeaders := w.Header()
		wrrHeaders := wrr.Header()
		for k, v := range wHeaders {
			for _, str := range v {
				wrrHeaders.Set(k, str)
			}

		}

		next.ServeHTTP(wrr, r)

		//copy back the headers
		for k, v := range wrrHeaders {
			for _, str := range v {
				wHeaders.Set(k, str)
			}
		}

		w.WriteHeader(wrr.Result().StatusCode)
		statusCode = wrr.Result().StatusCode
		log.Printf("%s %s %s %s (rid:%s) - %d", r.RemoteAddr, r.Method, r.URL.Path, r.Proto, requestID, statusCode)
	}

	return http.HandlerFunc(loggingHandler)
}
