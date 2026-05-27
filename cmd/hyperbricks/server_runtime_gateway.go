package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

const runtimeGatewayTimeout = 2 * time.Second

type runtimeGatewayResolveRequest struct {
	Host     string `json:"host"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	RawQuery string `json:"raw_query"`
}

type runtimeGatewayResolveResponse struct {
	Allowed         bool     `json:"allowed"`
	Target          string   `json:"target"`
	Project         string   `json:"project,omitempty"`
	Variant         string   `json:"variant,omitempty"`
	CacheTTLSeconds int      `json:"cache_ttl_seconds,omitempty"`
	Status          int      `json:"status,omitempty"`
	Message         string   `json:"message,omitempty"`
	SetCookies      []string `json:"set_cookies,omitempty"`
}

func handleRuntimeGateway(w http.ResponseWriter, r *http.Request) bool {
	config := getHyperBricksConfiguration().Server.RuntimeGateway
	if !runtimeGatewayMatches(config, r) {
		return false
	}

	resolution, err := resolveRuntimeGatewayTarget(r.Context(), config, r)
	if err != nil {
		logging.GetLogger().Warnw("runtime gateway resolver failed", "host", r.Host, "error", err)
		http.Error(w, "runtime resolver failed", http.StatusBadGateway)
		return true
	}

	if !resolution.Allowed {
		status := resolution.Status
		if status == 0 {
			status = http.StatusForbidden
		}
		message := strings.TrimSpace(resolution.Message)
		if message == "" {
			message = http.StatusText(status)
		}
		w.Header().Set("Cache-Control", "no-store")
		http.Error(w, message, status)
		return true
	}

	for _, cookie := range resolution.SetCookies {
		cookie = strings.TrimSpace(cookie)
		if cookie != "" {
			w.Header().Add("Set-Cookie", cookie)
		}
	}
	if len(resolution.SetCookies) > 0 && r.URL.Query().Has("runtime_token") {
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, runtimeGatewayCleanURL(r), http.StatusFound)
		return true
	}

	target, err := parseAndValidateRuntimeTarget(resolution.Target)
	if err != nil {
		logging.GetLogger().Warnw("runtime gateway rejected target", "host", r.Host, "target", resolution.Target, "error", err)
		http.Error(w, "runtime target rejected", http.StatusBadGateway)
		return true
	}

	proxyRuntimeRequest(w, r, target)
	return true
}

func runtimeGatewayCleanURL(r *http.Request) string {
	if r == nil || r.URL == nil {
		return "/"
	}
	cleanURL := *r.URL
	query := cleanURL.Query()
	query.Del("runtime_token")
	cleanURL.RawQuery = query.Encode()
	if cleanURL.Path == "" {
		cleanURL.Path = "/"
	}
	return cleanURL.RequestURI()
}

func validateRuntimeGatewayConfig(config shared.RuntimeGatewayConfig) error {
	if !config.Enabled {
		return nil
	}
	if len(runtimeGatewayDomains(config)) == 0 && len(runtimeGatewayHostSuffixes(config)) == 0 {
		return fmt.Errorf("runtime gateway requires runtime domain or host suffix")
	}
	resolver := strings.TrimSpace(config.Resolver)
	if resolver == "" {
		return fmt.Errorf("runtime gateway requires runtime resolver")
	}
	parsed, err := url.Parse(resolver)
	if err != nil {
		return fmt.Errorf("runtime resolver is invalid: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("runtime resolver must use http or https")
	}
	if parsed.Host == "" {
		return fmt.Errorf("runtime resolver host is empty")
	}
	return nil
}

func runtimeGatewayDomains(config shared.RuntimeGatewayConfig) []string {
	values := make([]string, 0, 1+len(config.Domains))
	appendDomain := func(value string) {
		for _, item := range strings.Split(value, ",") {
			domain := normalizeRuntimeDomain(item)
			if domain == "" {
				continue
			}
			duplicate := false
			for _, existing := range values {
				if existing == domain {
					duplicate = true
					break
				}
			}
			if duplicate {
				continue
			}
			values = append(values, domain)
		}
	}

	appendDomain(config.Domain)
	for _, domain := range config.Domains {
		appendDomain(domain)
	}
	return values
}

func runtimeGatewayHostSuffixes(config shared.RuntimeGatewayConfig) []string {
	values := make([]string, 0, 1+len(config.HostSuffixes))
	appendSuffix := func(value string) {
		for _, item := range strings.Split(value, ",") {
			suffix := normalizeRuntimeHostSuffix(item)
			if suffix == "" {
				continue
			}
			duplicate := false
			for _, existing := range values {
				if existing == suffix {
					duplicate = true
					break
				}
			}
			if duplicate {
				continue
			}
			values = append(values, suffix)
		}
	}

	appendSuffix(config.HostSuffix)
	for _, suffix := range config.HostSuffixes {
		appendSuffix(suffix)
	}
	return values
}

func runtimeGatewayMatches(config shared.RuntimeGatewayConfig, r *http.Request) bool {
	if !config.Enabled || r == nil {
		return false
	}
	domains := runtimeGatewayDomains(config)
	suffixes := runtimeGatewayHostSuffixes(config)
	if (len(domains) == 0 && len(suffixes) == 0) || strings.TrimSpace(config.Resolver) == "" {
		return false
	}
	host := requestHostWithoutPort(r.Host)
	if host == "" {
		return false
	}
	host = strings.ToLower(host)
	for _, domain := range domains {
		if strings.EqualFold(host, domain) {
			continue
		}
		if strings.HasSuffix(host, "."+strings.ToLower(domain)) {
			return true
		}
	}
	for _, suffix := range suffixes {
		if len(host) > len(suffix) && strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

func normalizeRuntimeDomain(domain string) string {
	domain = strings.TrimSpace(strings.ToLower(domain))
	domain = strings.TrimPrefix(domain, "*.")
	domain = strings.TrimPrefix(domain, ".")
	return strings.TrimSuffix(domain, ".")
}

func normalizeRuntimeHostSuffix(suffix string) string {
	suffix = strings.TrimSpace(strings.ToLower(suffix))
	suffix = strings.TrimPrefix(suffix, "*")
	suffix = strings.TrimSuffix(suffix, ".")
	if suffix == "." {
		return ""
	}
	return suffix
}

func requestHostWithoutPort(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	host = strings.Trim(host, "[]")
	return strings.TrimSuffix(strings.ToLower(host), ".")
}

func resolveRuntimeGatewayTarget(ctx context.Context, config shared.RuntimeGatewayConfig, r *http.Request) (runtimeGatewayResolveResponse, error) {
	resolverURL := strings.TrimSpace(config.Resolver)
	if resolverURL == "" {
		return runtimeGatewayResolveResponse{}, fmt.Errorf("runtime resolver is empty")
	}

	payload := runtimeGatewayResolveRequest{
		Host:     r.Host,
		Method:   r.Method,
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return runtimeGatewayResolveResponse{}, err
	}

	resolveCtx, cancel := context.WithTimeout(ctx, runtimeGatewayTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(resolveCtx, http.MethodPost, resolverURL, bytes.NewReader(body))
	if err != nil {
		return runtimeGatewayResolveResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cookie := strings.TrimSpace(r.Header.Get("Cookie")); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	if auth := strings.TrimSpace(r.Header.Get("Authorization")); auth != "" {
		req.Header.Set("Authorization", auth)
	}
	if requestID := strings.TrimSpace(r.Header.Get(requestIDHeader)); requestID != "" {
		req.Header.Set(requestIDHeader, requestID)
	}

	resp, err := apiHTTPClient().Do(req)
	if err != nil {
		return runtimeGatewayResolveResponse{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return runtimeGatewayResolveResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return runtimeGatewayResolveResponse{
			Allowed: false,
			Status:  resp.StatusCode,
			Message: strings.TrimSpace(string(respBody)),
		}, nil
	}

	var resolution runtimeGatewayResolveResponse
	if err := json.Unmarshal(respBody, &resolution); err != nil {
		return runtimeGatewayResolveResponse{}, err
	}
	return resolution, nil
}

func apiHTTPClient() *http.Client {
	return &http.Client{Timeout: runtimeGatewayTimeout}
}

func parseAndValidateRuntimeTarget(rawTarget string) (*url.URL, error) {
	target, err := url.Parse(strings.TrimSpace(rawTarget))
	if err != nil {
		return nil, err
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, fmt.Errorf("unsupported target scheme %q", target.Scheme)
	}
	if target.Host == "" {
		return nil, fmt.Errorf("target host is empty")
	}
	if !runtimeTargetHostAllowed(target.Hostname()) {
		return nil, fmt.Errorf("target host is not loopback or private")
	}
	return target, nil
}

func runtimeTargetHostAllowed(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate()
}

func proxyRuntimeRequest(w http.ResponseWriter, r *http.Request, target *url.URL) {
	originalHost := r.Host
	originalProto := "http"
	if r.TLS != nil {
		originalProto = "https"
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		if strings.TrimSpace(target.RawQuery) != "" {
			req.URL.Path = target.Path
			req.URL.RawQuery = target.RawQuery
		} else {
			req.URL.Path = singleJoiningSlash(target.Path, r.URL.Path)
			req.URL.RawQuery = r.URL.RawQuery
		}
		req.URL.RawPath = ""
		req.Host = target.Host
		req.Header.Set("X-Forwarded-Host", originalHost)
		req.Header.Set("X-Hyperbricks-Runtime-Host", originalHost)
		req.Header.Set("X-Forwarded-Proto", originalProto)
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		rewriteRuntimeLocationHeader(resp, target, originalHost, originalProto)
		return nil
	}
	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		logging.GetLogger().Warnw("runtime gateway proxy failed", "host", originalHost, "target", target.String(), "error", err)
		http.Error(rw, "runtime proxy failed", http.StatusBadGateway)
	}
	proxy.ServeHTTP(w, r)
}

func rewriteRuntimeLocationHeader(resp *http.Response, target *url.URL, originalHost string, originalProto string) {
	location := strings.TrimSpace(resp.Header.Get("Location"))
	if location == "" || target == nil || originalHost == "" {
		return
	}
	parsed, err := url.Parse(location)
	if err != nil || !parsed.IsAbs() {
		return
	}
	if !strings.EqualFold(parsed.Scheme, target.Scheme) || !strings.EqualFold(parsed.Host, target.Host) {
		return
	}
	if originalProto != "http" && originalProto != "https" {
		originalProto = "http"
	}
	parsed.Scheme = originalProto
	parsed.Host = originalHost
	resp.Header.Set("Location", parsed.String())
}

func singleJoiningSlash(basePath, requestPath string) string {
	if basePath == "" || basePath == "/" {
		if requestPath == "" {
			return "/"
		}
		return requestPath
	}
	baseSlash := strings.HasSuffix(basePath, "/")
	requestSlash := strings.HasPrefix(requestPath, "/")
	switch {
	case baseSlash && requestSlash:
		return basePath + requestPath[1:]
	case !baseSlash && !requestSlash:
		return basePath + "/" + requestPath
	default:
		return basePath + requestPath
	}
}
