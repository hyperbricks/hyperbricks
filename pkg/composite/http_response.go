package composite

import (
	"fmt"
	"net/http"
	"strings"
)

// HTTPResponseConfig describes the browser response of a route or guard action.
// It is independent of the headers sent by an API component to its upstream.
type HTTPResponseConfig struct {
	Status  int               `mapstructure:"status" description:"Browser HTTP status (200–599); omit to retain the route or guard default"`
	Headers map[string]string `mapstructure:"headers" description:"HTTP response headers sent to the browser"`
}

func (response HTTPResponseConfig) Validate() error {
	if response.Status != 0 && (response.Status < 200 || response.Status > 599) {
		return fmt.Errorf("response.status must be between 200 and 599")
	}
	if err := ValidateHTTPHeaders(response.Headers); err != nil {
		return err
	}
	for name, value := range response.Headers {
		if !strings.EqualFold(name, "Vary") {
			continue
		}
		for _, field := range strings.Split(value, ",") {
			field = strings.TrimSpace(field)
			if field == "" || field == "*" {
				continue
			}
			if err := ValidateHTTPHeaders(map[string]string{field: ""}); err != nil {
				return fmt.Errorf("invalid Vary field: %w", err)
			}
		}
	}
	return nil
}

// ValidateHTTPHeaders also prevents ambiguous duplicate names after HTTP
// canonicalization. Header values remain literal and case-sensitive.
func ValidateHTTPHeaders(headers map[string]string) error {
	seen := make(map[string]bool, len(headers))
	for name, value := range headers {
		if name == "" {
			return fmt.Errorf("HTTP header name must not be empty")
		}
		for _, ch := range name {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || strings.ContainsRune("!#$%&'*+-.^_`|~", ch)) {
				return fmt.Errorf("invalid HTTP header name %q", name)
			}
		}
		canonical := http.CanonicalHeaderKey(name)
		if seen[canonical] {
			return fmt.Errorf("duplicate HTTP header name %q", name)
		}
		seen[canonical] = true
		for _, ch := range value {
			if (ch < 32 && ch != '\t') || ch == 127 {
				return fmt.Errorf("invalid value for HTTP header %q", name)
			}
		}
	}
	return nil
}
