package apiutil

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func TestValidateForwardTokenRaw(t *testing.T) {
	tests := []struct {
		name    string
		raw     map[string]interface{}
		wantErr bool
	}{
		{name: "absent", raw: map[string]interface{}{"endpoint": "https://api.example.test"}},
		{name: "string", raw: map[string]interface{}{"forwardtoken": "session"}},
		{name: "empty string", raw: map[string]interface{}{"forwardtoken": ""}},
		{name: "number", raw: map[string]interface{}{"forwardtoken": 7}, wantErr: true},
		{name: "boolean", raw: map[string]interface{}{"forwardtoken": true}, wantErr: true},
		{name: "nil", raw: map[string]interface{}{"forwardtoken": nil}, wantErr: true},
		{name: "case confusable", raw: map[string]interface{}{"ForwardToken": "session"}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateForwardTokenRaw(test.raw)
			if (err != nil) != test.wantErr {
				t.Fatalf("ValidateForwardTokenRaw() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}

func TestValidateAuthSettingsRejectsEveryPairOfAuthSources(t *testing.T) {
	tests := []struct {
		name     string
		settings AuthSettings
	}{
		{
			name:     "Authorization and JWT",
			settings: AuthSettings{Headers: map[string]string{"Authorization": "Bearer configured"}, JwtSecret: "secret"},
		},
		{
			name:     "Authorization and Basic",
			settings: AuthSettings{Headers: map[string]string{"Authorization": "Bearer configured"}, Username: "user", Password: "pass"},
		},
		{
			name:     "Authorization and forwarded token",
			settings: AuthSettings{Headers: map[string]string{"Authorization": "Bearer configured"}, ForwardToken: "session"},
		},
		{
			name:     "JWT and Basic",
			settings: AuthSettings{JwtSecret: "secret", Username: "user", Password: "pass"},
		},
		{
			name:     "JWT and forwarded token",
			settings: AuthSettings{JwtSecret: "secret", ForwardToken: "session"},
		},
		{
			name:     "Basic and forwarded token",
			settings: AuthSettings{Username: "user", Password: "pass", ForwardToken: "session"},
		},
	}

	endpoint := parseAuthTestURL(t, "https://api.example.test/resource")
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateAuthSettings(endpoint, "live", test.settings); err == nil || !strings.Contains(err.Error(), "exactly one") {
				t.Fatalf("ValidateAuthSettings() error = %v, want mutually exclusive auth error", err)
			}
		})
	}
}

func TestValidateAuthSettingsRejectsMalformedSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings AuthSettings
		want     string
	}{
		{name: "username without password", settings: AuthSettings{Username: "user"}, want: "both username and password"},
		{name: "password without username", settings: AuthSettings{Password: "pass"}, want: "both username and password"},
		{name: "claims without secret", settings: AuthSettings{JwtClaims: map[string]string{"sub": "user"}}, want: "requires jwtsecret"},
		{name: "invalid cookie name", settings: AuthSettings{ForwardToken: "session token"}, want: "valid cookie"},
		{name: "empty Authorization", settings: AuthSettings{Headers: map[string]string{"Authorization": "  "}}, want: "must not be empty"},
		{name: "invalid header name", settings: AuthSettings{Headers: map[string]string{"Bad Header": "value"}}, want: "name is invalid"},
		{name: "invalid header value", settings: AuthSettings{Headers: map[string]string{"X-Token": "first\r\nInjected: yes"}}, want: "invalid value"},
		{name: "duplicate header casing", settings: AuthSettings{Headers: map[string]string{"X-Token": "first", "x-token": "second"}}, want: "duplicated"},
		{name: "duplicate Authorization casing", settings: AuthSettings{Headers: map[string]string{"Authorization": "Bearer first", "authorization": "Bearer second"}}, want: "duplicated"},
	}

	endpoint := parseAuthTestURL(t, "https://api.example.test")
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateAuthSettings(endpoint, "live", test.settings)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ValidateAuthSettings() error = %v, want text %q", err, test.want)
			}
		})
	}
}

func TestValidateAuthSettingsEnforcesCredentialTransport(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		mode     string
		settings AuthSettings
		wantErr  bool
	}{
		{name: "HTTPS Authorization", endpoint: "https://api.example.test", mode: "live", settings: AuthSettings{Headers: map[string]string{"Authorization": "Bearer secret"}}, wantErr: false},
		{name: "public HTTP without credentials", endpoint: "http://api.example.test", mode: "live", settings: AuthSettings{}, wantErr: false},
		{name: "standard headers are not credentials", endpoint: "http://api.example.test", mode: "live", settings: AuthSettings{Headers: map[string]string{"Accept": "application/json", "Content-Type": "application/json"}}, wantErr: false},
		{name: "empty custom header carries no credentials", endpoint: "http://api.example.test", mode: "live", settings: AuthSettings{Headers: map[string]string{"X-API-Key": ""}}, wantErr: false},
		{name: "request body requires HTTPS", endpoint: "http://api.example.test", mode: "development", settings: AuthSettings{HasBody: true, Headers: map[string]string{"Content-Type": "application/json"}}, wantErr: true},
		{name: "request body allowed on development loopback", endpoint: "http://127.0.0.1:8080", mode: "development", settings: AuthSettings{HasBody: true, Headers: map[string]string{"Content-Type": "application/json"}}, wantErr: false},
		{name: "custom header requires HTTPS", endpoint: "http://api.example.test", mode: "development", settings: AuthSettings{Headers: map[string]string{"X-API-Key": "secret"}}, wantErr: true},
		{name: "Cookie header requires HTTPS", endpoint: "http://api.example.test", mode: "development", settings: AuthSettings{Headers: map[string]string{"Cookie": "session=secret"}}, wantErr: true},
		{name: "Proxy Authorization requires HTTPS", endpoint: "http://api.example.test", mode: "development", settings: AuthSettings{Headers: map[string]string{"Proxy-Authorization": "Basic secret"}}, wantErr: true},
		{name: "development IPv4 loopback", endpoint: "http://127.0.0.1:8080", mode: "development", settings: AuthSettings{ForwardToken: "session"}, wantErr: false},
		{name: "debug IPv6 loopback", endpoint: "http://[::1]:8080", mode: "debug", settings: AuthSettings{Username: "user", Password: "pass"}, wantErr: false},
		{name: "live IPv4 loopback", endpoint: "http://127.0.0.1:8080", mode: "live", settings: AuthSettings{JwtSecret: "secret"}, wantErr: true},
		{name: "unknown mode loopback", endpoint: "http://127.0.0.1:8080", mode: "test", settings: AuthSettings{JwtSecret: "secret"}, wantErr: true},
		{name: "localhost is not literal", endpoint: "http://localhost:8080", mode: "development", settings: AuthSettings{JwtSecret: "secret"}, wantErr: true},
		{name: "remote HTTP in development", endpoint: "http://192.0.2.4", mode: "development", settings: AuthSettings{JwtSecret: "secret"}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateAuthSettings(parseAuthTestURL(t, test.endpoint), test.mode, test.settings)
			if (err != nil) != test.wantErr {
				t.Fatalf("ValidateAuthSettings() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}

func TestValidateAuthSettingsRequiresAbsoluteHTTPSEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint *url.URL
		want     string
	}{
		{name: "nil", endpoint: nil, want: "required"},
		{name: "relative", endpoint: parseAuthTestURL(t, "/relative"), want: "absolute"},
		{name: "missing host", endpoint: parseAuthTestURL(t, "https:/missing-host"), want: "absolute"},
		{name: "opaque", endpoint: &url.URL{Scheme: "https", Opaque: "api.example.test"}, want: "absolute"},
		{name: "unsupported scheme", endpoint: parseAuthTestURL(t, "ftp://api.example.test/file"), want: "HTTP or HTTPS"},
		{name: "zero port", endpoint: parseAuthTestURL(t, "https://api.example.test:0"), want: "invalid port"},
		{name: "large port", endpoint: parseAuthTestURL(t, "https://api.example.test:65536"), want: "invalid port"},
		{name: "empty port", endpoint: parseAuthTestURL(t, "https://api.example.test:"), want: "invalid port"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateAuthSettings(test.endpoint, "live", AuthSettings{})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ValidateAuthSettings() error = %v, want text %q", err, test.want)
			}
		})
	}
}

func TestValidateAuthSettingsRejectsEndpointUserInfo(t *testing.T) {
	for _, endpoint := range []string{
		"https://user@api.example.test/resource",
		"https://user:password@api.example.test/resource",
	} {
		err := ValidateAuthSettings(parseAuthTestURL(t, endpoint), "live", AuthSettings{})
		if err == nil || !strings.Contains(err.Error(), "must not contain user information") {
			t.Fatalf("ValidateAuthSettings(%q) error = %v", endpoint, err)
		}
		if strings.Contains(err.Error(), "user") && strings.Contains(err.Error(), "password") {
			t.Fatalf("validation error exposed URL credentials: %q", err)
		}
	}
}

func TestApplyAuthUsesOnlyConfiguredSource(t *testing.T) {
	tests := []struct {
		name              string
		settings          AuthSettings
		incoming          func() *http.Request
		wantAuthorization string
		wantAccept        string
	}{
		{
			name:              "configured Authorization",
			settings:          AuthSettings{Headers: map[string]string{"Authorization": "Bearer configured", "Accept": "application/json"}},
			incoming:          requestWithHeaders(http.Header{"Authorization": {"Bearer browser"}, "Cookie": {"session=cookie"}}),
			wantAuthorization: "Bearer configured",
			wantAccept:        "application/json",
		},
		{
			name:              "complete Basic",
			settings:          AuthSettings{Username: "configured-user", Password: "configured-password"},
			incoming:          requestWithHeaders(http.Header{"Authorization": {"Bearer browser"}}),
			wantAuthorization: "Basic " + base64.StdEncoding.EncodeToString([]byte("configured-user:configured-password")),
		},
		{
			name:              "named forwarded token",
			settings:          AuthSettings{ForwardToken: "session"},
			incoming:          requestWithHeaders(http.Header{"Authorization": {"Bearer browser"}, "Cookie": {"other=ignored; session=cookie-token"}}),
			wantAuthorization: "Bearer cookie-token",
		},
		{
			name:     "browser Authorization is not implicit",
			settings: AuthSettings{},
			incoming: requestWithHeaders(http.Header{"Authorization": {"Bearer browser"}}),
		},
		{
			name:     "unselected cookie is not forwarded",
			settings: AuthSettings{ForwardToken: "session"},
			incoming: requestWithHeaders(http.Header{"Cookie": {"other=cookie-token"}}),
		},
		{
			name:     "empty selected cookie is not forwarded",
			settings: AuthSettings{ForwardToken: "session"},
			incoming: requestWithHeaders(http.Header{"Cookie": {"session="}}),
		},
		{
			name:     "nil incoming request has no token",
			settings: AuthSettings{ForwardToken: "session"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outgoing := httptest.NewRequest(http.MethodGet, "https://api.example.test/resource", nil)
			var incoming *http.Request
			if test.incoming != nil {
				incoming = test.incoming()
			}
			if err := ApplyAuth(outgoing, incoming, test.settings); err != nil {
				t.Fatalf("ApplyAuth() error = %v", err)
			}
			if got := outgoing.Header.Get("Authorization"); got != test.wantAuthorization {
				t.Fatalf("Authorization = %q, want %q", got, test.wantAuthorization)
			}
			if got := outgoing.Header.Get("Accept"); got != test.wantAccept {
				t.Fatalf("Accept = %q, want %q", got, test.wantAccept)
			}
		})
	}
}

func TestApplyAuthSignsJWTWithPreservedClaimSemantics(t *testing.T) {
	outgoing := httptest.NewRequest(http.MethodGet, "https://api.example.test", nil)
	before := time.Now().Unix()
	err := ApplyAuth(outgoing, nil, AuthSettings{
		JwtSecret: "signing-secret",
		JwtClaims: map[string]string{"aud": "api", "exp": "300"},
	})
	if err != nil {
		t.Fatalf("ApplyAuth() error = %v", err)
	}

	raw := strings.TrimPrefix(outgoing.Header.Get("Authorization"), "Bearer ")
	parsed, err := jwt.Parse(raw, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			t.Fatalf("JWT signing method = %v, want HS256", token.Method.Alg())
		}
		return []byte("signing-secret"), nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("signed JWT is invalid: token=%v error=%v", parsed, err)
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatalf("claims type = %T, want jwt.MapClaims", parsed.Claims)
	}
	if claims["sub"] != "default_user" || claims["aud"] != "api" {
		t.Fatalf("claims = %#v, want default sub and configured aud", claims)
	}
	expiry, ok := claims["exp"].(float64)
	if !ok || int64(expiry) < before+299 || int64(expiry) > time.Now().Unix()+301 {
		t.Fatalf("exp claim = %#v, want about five minutes from now", claims["exp"])
	}
}

func TestApplyAuthRejectsDuplicateOrMalformedCookieBeforeSettingAuthorization(t *testing.T) {
	tests := []struct {
		name   string
		cookie string
		want   string
	}{
		{name: "duplicate selected cookie", cookie: "session=first; session=second", want: "duplicate"},
		{name: "malformed Cookie header", cookie: "session=good; malformed", want: "Cookie header is invalid"},
		{name: "header injection bytes", cookie: "session=good\r\nAuthorization: Bearer injected", want: "Cookie header is invalid"},
		{name: "space cannot enter bearer header", cookie: `session="two words"`, want: "invalid bytes"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			incoming := requestWithHeaders(http.Header{"Cookie": {test.cookie}})()
			outgoing := httptest.NewRequest(http.MethodGet, "https://api.example.test", nil)
			err := ApplyAuth(outgoing, incoming, AuthSettings{ForwardToken: "session"})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ApplyAuth() error = %v, want text %q", err, test.want)
			}
			if got := outgoing.Header.Get("Authorization"); got != "" {
				t.Fatalf("Authorization was set after rejected cookie: %q", got)
			}
		})
	}
}

func requestWithHeaders(headers http.Header) func() *http.Request {
	return func() *http.Request {
		request := httptest.NewRequest(http.MethodGet, "https://browser.example.test", nil)
		request.Header = headers.Clone()
		return request
	}
}

func parseAuthTestURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	endpoint, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", raw, err)
	}
	return endpoint
}
