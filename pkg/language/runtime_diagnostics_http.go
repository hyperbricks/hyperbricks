package language

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	runtimeDiagnosticsPath             = "/__hyperbricks/render-diagnostics"
	maxRuntimeDiagnosticsResponseBytes = 8 << 20
)

type RuntimeCredentialMode uint8

const (
	RuntimeCredentialsNone RuntimeCredentialMode = iota
	RuntimeCredentialsAutomatic
	RuntimeCredentialsExplicit
)

type RuntimeDiagnosticsAuth struct {
	Username string
	Password string
	Mode     RuntimeCredentialMode
}

type HTTPRuntimeDiagnosticsClient struct {
	endpoint *url.URL
	auth     RuntimeDiagnosticsAuth
	client   *http.Client
}

// NewHTTPRuntimeDiagnosticsClient configures an authenticated client for the
// existing development-only endpoint. Automatic credentials are accepted only
// for a literal loopback address or localhost. Explicit remote credentials
// additionally require HTTPS.
func NewHTTPRuntimeDiagnosticsClient(baseURL string, auth RuntimeDiagnosticsAuth, client *http.Client) (*HTTPRuntimeDiagnosticsClient, error) {
	endpoint, err := currentRuntimeDiagnosticsURL(baseURL)
	if err != nil {
		return nil, err
	}
	if err := validateRuntimeDiagnosticsAuth(endpoint, auth); err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &HTTPRuntimeDiagnosticsClient{endpoint: endpoint, auth: auth, client: client}, nil
}

// RuntimeDiagnosticsURLIsLoopback validates a runtime base URL and reports
// whether its literal host is localhost or a loopback IP address. DNS names
// are deliberately not resolved for this security decision.
func RuntimeDiagnosticsURLIsLoopback(baseURL string) (bool, error) {
	canonical, err := CanonicalRuntimeDiagnosticsURL(baseURL)
	if err != nil {
		return false, err
	}
	parsed, _ := url.Parse(canonical)
	return isRuntimeLoopback(parsed.Hostname()), nil
}

// CanonicalRuntimeDiagnosticsURL validates a runtime base and returns only its
// HTTP(S) origin. HyperBricks developer endpoints use absolute paths, so any
// input path, query, or fragment is both irrelevant and unsafe to echo in
// editor protocol messages.
func CanonicalRuntimeDiagnosticsURL(baseURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", fmt.Errorf("runtime diagnostics URL is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("runtime diagnostics URL must use http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("runtime diagnostics URL requires a host")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("runtime diagnostics URL must not contain credentials")
	}
	parsed.Path = ""
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func (client *HTTPRuntimeDiagnosticsClient) Endpoint() string {
	return client.endpoint.String()
}

func (client *HTTPRuntimeDiagnosticsClient) FetchCurrent(ctx context.Context) (RuntimeDiagnosticsSnapshot, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.endpoint.String(), nil)
	if err != nil {
		return RuntimeDiagnosticsSnapshot{}, fmt.Errorf("create runtime diagnostics request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if client.auth.Username != "" {
		request.SetBasicAuth(client.auth.Username, client.auth.Password)
	}

	httpClient := *client.client
	if client.auth.Username != "" {
		// Never let an authenticated request follow a redirect. This prevents a
		// loopback runtime (or a compromised proxy in front of an explicit remote
		// runtime) from redirecting credentials to another origin.
		httpClient.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return RuntimeDiagnosticsSnapshot{}, fmt.Errorf("fetch runtime diagnostics: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, maxRuntimeDiagnosticsResponseBytes+1))
	if err != nil {
		return RuntimeDiagnosticsSnapshot{}, fmt.Errorf("read runtime diagnostics: %w", err)
	}
	if len(body) > maxRuntimeDiagnosticsResponseBytes {
		return RuntimeDiagnosticsSnapshot{}, fmt.Errorf("runtime diagnostics response exceeds %d bytes", maxRuntimeDiagnosticsResponseBytes)
	}
	if response.StatusCode != http.StatusOK {
		return RuntimeDiagnosticsSnapshot{}, fmt.Errorf("runtime diagnostics request returned %s", response.Status)
	}
	var snapshot RuntimeDiagnosticsSnapshot
	if err := json.Unmarshal(body, &snapshot); err != nil {
		return RuntimeDiagnosticsSnapshot{}, fmt.Errorf("decode runtime diagnostics: %w", err)
	}
	if snapshot.CheckedRoutes < 0 || snapshot.TotalRoutes < 0 || snapshot.EvictedContexts < 0 || snapshot.CheckedRoutes > snapshot.TotalRoutes {
		return RuntimeDiagnosticsSnapshot{}, errors.New("runtime diagnostics response contains invalid route coverage")
	}
	return snapshot, nil
}

func currentRuntimeDiagnosticsURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("runtime diagnostics URL is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("runtime diagnostics URL must use http or https")
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("runtime diagnostics URL requires a host")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("runtime diagnostics URL must not contain credentials")
	}
	parsed.Path = runtimeDiagnosticsPath
	parsed.RawPath = ""
	parsed.RawQuery = "view=current"
	parsed.Fragment = ""
	return parsed, nil
}

func validateRuntimeDiagnosticsAuth(endpoint *url.URL, auth RuntimeDiagnosticsAuth) error {
	hasUsername := auth.Username != ""
	hasPassword := auth.Password != ""
	if hasUsername != hasPassword {
		return fmt.Errorf("runtime diagnostics credentials require both username and password")
	}
	if !hasUsername {
		if auth.Mode != RuntimeCredentialsNone && auth.Mode != RuntimeCredentialsAutomatic && auth.Mode != RuntimeCredentialsExplicit {
			return fmt.Errorf("unknown runtime diagnostics credential mode %d", auth.Mode)
		}
		return nil
	}
	switch auth.Mode {
	case RuntimeCredentialsAutomatic:
		if !isRuntimeLoopback(endpoint.Hostname()) {
			return fmt.Errorf("automatic runtime diagnostics credentials are limited to loopback URLs")
		}
	case RuntimeCredentialsExplicit:
		if !isRuntimeLoopback(endpoint.Hostname()) && endpoint.Scheme != "https" {
			return fmt.Errorf("explicit remote runtime diagnostics credentials require HTTPS")
		}
	case RuntimeCredentialsNone:
		return fmt.Errorf("runtime diagnostics credentials require an explicit credential mode")
	default:
		return fmt.Errorf("unknown runtime diagnostics credential mode %d", auth.Mode)
	}
	return nil
}

func isRuntimeLoopback(host string) bool {
	host = strings.TrimSuffix(strings.TrimSpace(host), ".")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if zone := strings.LastIndexByte(host, '%'); zone >= 0 {
		host = host[:zone]
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
