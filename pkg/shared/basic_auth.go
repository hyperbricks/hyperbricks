package shared

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net/http"
)

const (
	DeveloperInterfaceRealm              = "HyperBricks Developer"
	DeveloperInterfaceUnavailableMessage = "Developer interface unavailable: credentials are not configured"
)

func ApplyBasicAuth(request *http.Request, credentials CredentialsConfig, label string) error {
	if request == nil {
		return fmt.Errorf("%s request is nil", label)
	}
	if err := credentials.Validate(label); err != nil {
		return err
	}
	request.SetBasicAuth(credentials.User, credentials.Password)
	return nil
}

// BasicAuth protects an entire HTTP service. Missing server credentials lock
// the service with 503; configured credentials produce a conventional Basic
// challenge for missing or invalid request credentials.
func BasicAuth(next http.Handler, credentials CredentialsConfig, realm string) http.Handler {
	return BasicAuthWithUnavailable(next, credentials, realm, "service authentication is not configured")
}

// BasicAuthWithUnavailable protects a handler and uses unavailableMessage for
// the fail-closed response when no complete account was configured.
func BasicAuthWithUnavailable(next http.Handler, credentials CredentialsConfig, realm, unavailableMessage string) http.Handler {
	if err := credentials.Validate("service"); err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			secureAuthErrorHeaders(w)
			http.Error(w, unavailableMessage, http.StatusServiceUnavailable)
		})
	}

	expectedUser := sha256.Sum256([]byte(credentials.User))
	expectedPassword := sha256.Sum256([]byte(credentials.Password))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !basicAuthMatches(r, expectedUser, expectedPassword) {
			secureAuthErrorHeaders(w)
			w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`", charset="UTF-8"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireBasicAuth writes the configured locked/challenge response and returns
// false when the request may not enter a protected handler.
func RequireBasicAuth(w http.ResponseWriter, r *http.Request, credentials CredentialsConfig, realm, unavailableMessage string) bool {
	authorized := false
	BasicAuthWithUnavailable(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		authorized = true
	}), credentials, realm, unavailableMessage).ServeHTTP(w, r)
	return authorized
}

// BasicAuthAuthorized checks an already received request without writing a
// response. It is used only to decide whether developer-only decoration may be
// added to an otherwise public response.
func BasicAuthAuthorized(r *http.Request, credentials CredentialsConfig) bool {
	if r == nil || !credentials.Complete() {
		return false
	}
	expectedUser := sha256.Sum256([]byte(credentials.User))
	expectedPassword := sha256.Sum256([]byte(credentials.Password))
	return basicAuthMatches(r, expectedUser, expectedPassword)
}

// DeveloperInterfaceAuthorized reports whether the request stored in a render
// context has the current module's dashboard credentials. Renderers use this
// to keep developer-only panels out of ordinary public responses.
func DeveloperInterfaceAuthorized(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	request, _ := ctx.Value(Request).(*http.Request)
	credentials := GetHyperBricksConfiguration().Development.Dashboard.Credentials
	return BasicAuthAuthorized(request, credentials)
}

func basicAuthMatches(r *http.Request, expectedUser, expectedPassword [sha256.Size]byte) bool {
	if r == nil {
		return false
	}
	user, password, ok := r.BasicAuth()
	actualUser := sha256.Sum256([]byte(user))
	actualPassword := sha256.Sum256([]byte(password))
	userOK := subtle.ConstantTimeCompare(actualUser[:], expectedUser[:])
	passwordOK := subtle.ConstantTimeCompare(actualPassword[:], expectedPassword[:])
	return ok && userOK&passwordOK == 1
}

func secureAuthErrorHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}
