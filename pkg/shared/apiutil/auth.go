package apiutil

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/net/http/httpguts"
)

// AuthSettings contains the configuration that may authenticate an upstream
// API request. Exactly one authentication source may be configured.
type AuthSettings struct {
	ForwardToken string
	Headers      map[string]string
	Username     string
	Password     string
	JwtSecret    string
	JwtClaims    map[string]string
	HasBody      bool
}

// ValidateForwardTokenRaw validates forwardtoken before weak mapstructure
// decoding can coerce its value or accept a case-confusable field name.
func ValidateForwardTokenRaw(raw map[string]interface{}) error {
	for name, value := range raw {
		if !strings.EqualFold(name, "forwardtoken") {
			continue
		}
		if name != "forwardtoken" {
			return fmt.Errorf("authentication field must be named forwardtoken")
		}
		if _, ok := value.(string); !ok {
			return fmt.Errorf("forwardtoken must be a string")
		}
	}
	return nil
}

// ValidateAuthSettings rejects ambiguous authentication and credentials that
// could be sent over an insecure network connection. HTTP is permitted for a
// literal loopback IP only while running in development or debug mode.
func ValidateAuthSettings(endpoint *url.URL, mode string, settings AuthSettings) error {
	inspection, err := inspectAuthSettings(settings)
	if err != nil {
		return err
	}
	if err := validateEndpointURL(endpoint); err != nil {
		return err
	}
	if !inspection.hasCredentialMaterial {
		return nil
	}
	if strings.EqualFold(endpoint.Scheme, "https") {
		return nil
	}
	if strings.EqualFold(endpoint.Scheme, "http") && allowsLoopbackHTTP(mode) && isLiteralLoopback(endpoint.Hostname()) {
		return nil
	}
	return fmt.Errorf("credentials require HTTPS; HTTP is allowed only for literal loopback endpoints in development or debug mode")
}

// ApplyAuth copies configured upstream headers and applies the single selected
// authentication source. It never forwards the incoming Authorization header
// or any incoming cookie other than the explicitly named token cookie.
func ApplyAuth(outgoing, incoming *http.Request, settings AuthSettings) error {
	inspection, err := inspectAuthSettings(settings)
	if err != nil {
		return err
	}
	if outgoing == nil {
		return fmt.Errorf("outgoing request is required")
	}
	if outgoing.Header == nil {
		outgoing.Header = make(http.Header)
	}
	for name, value := range settings.Headers {
		outgoing.Header.Set(name, value)
	}

	switch inspection.source {
	case authSourceAuthorization, authSourceNone:
		return nil
	case authSourceJWT:
		token, err := signedJWT(settings.JwtSecret, settings.JwtClaims, time.Now())
		if err != nil {
			return fmt.Errorf("failed to sign upstream JWT")
		}
		outgoing.Header.Set("Authorization", "Bearer "+token)
		return nil
	case authSourceBasic:
		outgoing.SetBasicAuth(settings.Username, settings.Password)
		return nil
	case authSourceForwardToken:
		token, found, err := resolveForwardToken(incoming, settings.ForwardToken)
		if err != nil {
			return err
		}
		if found {
			outgoing.Header.Set("Authorization", "Bearer "+token)
		}
		return nil
	default:
		return fmt.Errorf("invalid upstream authentication configuration")
	}
}

type authSource uint8

const (
	authSourceNone authSource = iota
	authSourceAuthorization
	authSourceJWT
	authSourceBasic
	authSourceForwardToken
)

type authInspection struct {
	source                authSource
	hasCredentialMaterial bool
}

func inspectAuthSettings(settings AuthSettings) (authInspection, error) {
	authorizationConfigured, sensitiveHeaders, err := validateConfiguredHeaders(settings.Headers)
	if err != nil {
		return authInspection{}, err
	}

	hasUsername := settings.Username != ""
	hasPassword := settings.Password != ""
	if hasUsername != hasPassword {
		return authInspection{}, fmt.Errorf("basic authentication requires both username and password")
	}
	if len(settings.JwtClaims) > 0 && settings.JwtSecret == "" {
		return authInspection{}, fmt.Errorf("jwtclaims requires jwtsecret")
	}
	if settings.ForwardToken != "" {
		if err := (&http.Cookie{Name: settings.ForwardToken, Value: "token"}).Valid(); err != nil {
			return authInspection{}, fmt.Errorf("forwardtoken must name a valid cookie")
		}
	}

	sources := make([]authSource, 0, 4)
	if authorizationConfigured {
		sources = append(sources, authSourceAuthorization)
	}
	if settings.JwtSecret != "" {
		sources = append(sources, authSourceJWT)
	}
	if hasUsername && hasPassword {
		sources = append(sources, authSourceBasic)
	}
	if settings.ForwardToken != "" {
		sources = append(sources, authSourceForwardToken)
	}
	if len(sources) > 1 {
		return authInspection{}, fmt.Errorf("configure exactly one upstream authentication source: Authorization header, jwtsecret, basic credentials, or forwardtoken")
	}

	inspection := authInspection{hasCredentialMaterial: settings.HasBody || sensitiveHeaders || len(sources) > 0}
	if len(sources) == 1 {
		inspection.source = sources[0]
	}
	return inspection, nil
}

func validateConfiguredHeaders(headers map[string]string) (authorizationConfigured, sensitive bool, err error) {
	seen := make(map[string]struct{}, len(headers))
	for name, value := range headers {
		if !httpguts.ValidHeaderFieldName(name) {
			return false, false, fmt.Errorf("configured header name is invalid: %q", name)
		}
		if !httpguts.ValidHeaderFieldValue(value) {
			return false, false, fmt.Errorf("configured header %q has an invalid value", name)
		}
		canonical := strings.ToLower(name)
		if _, exists := seen[canonical]; exists {
			return false, false, fmt.Errorf("configured header %q is duplicated with different casing", name)
		}
		seen[canonical] = struct{}{}

		if strings.EqualFold(name, "Authorization") {
			if strings.TrimSpace(value) == "" {
				return false, false, fmt.Errorf("configured Authorization header must not be empty")
			}
			authorizationConfigured = true
		}
		if value != "" && !isNonCredentialHeader(name) {
			sensitive = true
		}
	}
	return authorizationConfigured, sensitive, nil
}

func isNonCredentialHeader(name string) bool {
	return strings.EqualFold(name, "Accept") || strings.EqualFold(name, "Content-Type")
}

func allowsLoopbackHTTP(mode string) bool {
	return mode == "development" || mode == "debug"
}

func isLiteralLoopback(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func validateEndpointURL(endpoint *url.URL) error {
	if endpoint == nil {
		return fmt.Errorf("endpoint URL is required")
	}
	if endpoint.User != nil {
		return fmt.Errorf("endpoint URL must not contain user information")
	}
	if endpoint.Opaque != "" || !endpoint.IsAbs() || endpoint.Host == "" || endpoint.Hostname() == "" {
		return fmt.Errorf("endpoint URL must be an absolute HTTP or HTTPS URL with a host")
	}
	if !strings.EqualFold(endpoint.Scheme, "http") && !strings.EqualFold(endpoint.Scheme, "https") {
		return fmt.Errorf("endpoint URL must use HTTP or HTTPS")
	}
	if strings.HasSuffix(endpoint.Host, ":") {
		return fmt.Errorf("endpoint URL contains an invalid port")
	}
	if port := endpoint.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return fmt.Errorf("endpoint URL contains an invalid port")
		}
	}
	return nil
}

func resolveForwardToken(request *http.Request, cookieName string) (string, bool, error) {
	if request == nil {
		return "", false, nil
	}
	var selected *http.Cookie
	for _, line := range request.Header.Values("Cookie") {
		cookies, err := http.ParseCookie(line)
		if err != nil {
			return "", false, fmt.Errorf("incoming Cookie header is invalid")
		}
		for _, cookie := range cookies {
			if cookie.Name != cookieName {
				continue
			}
			if selected != nil {
				return "", false, fmt.Errorf("incoming request contains duplicate %s cookies", cookieName)
			}
			selected = cookie
		}
	}
	if selected == nil || selected.Value == "" {
		return "", false, nil
	}
	if strings.ContainsAny(selected.Value, " \t") || !httpguts.ValidHeaderFieldValue("Bearer "+selected.Value) {
		return "", false, fmt.Errorf("forwarded token cookie contains invalid bytes")
	}
	return selected.Value, true, nil
}

func signedJWT(secret string, configuredClaims map[string]string, now time.Time) (string, error) {
	claims := jwt.MapClaims{}
	for name, value := range configuredClaims {
		claims[name] = value
	}
	if _, exists := claims["sub"]; !exists {
		claims["sub"] = "default_user"
	}
	if configuredExpiry, exists := configuredClaims["exp"]; exists {
		seconds, err := strconv.ParseInt(configuredExpiry, 10, 64)
		if err == nil {
			claims["exp"] = now.Unix() + seconds
		} else {
			claims["exp"] = now.Add(time.Hour).Unix()
		}
	} else {
		claims["exp"] = now.Add(time.Hour).Unix()
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
