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

const previewGatewayTimeout = 2 * time.Second

type previewGatewayResolveRequest struct {
	Host     string `json:"host"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	RawQuery string `json:"raw_query"`
}

type previewGatewayResolveResponse struct {
	Allowed         bool     `json:"allowed"`
	Target          string   `json:"target"`
	Project         string   `json:"project,omitempty"`
	Preview         string   `json:"preview,omitempty"`
	CacheTTLSeconds int      `json:"cache_ttl_seconds,omitempty"`
	Status          int      `json:"status,omitempty"`
	Message         string   `json:"message,omitempty"`
	SetCookies      []string `json:"set_cookies,omitempty"`
}

func handlePreviewGateway(w http.ResponseWriter, r *http.Request) bool {
	config := getHyperBricksConfiguration().Server.PreviewGateway
	if !previewGatewayMatches(config, r) {
		return false
	}

	resolution, err := resolvePreviewGatewayTarget(r.Context(), config, r)
	if err != nil {
		logging.GetLogger().Warnw("preview gateway resolver failed", "host", r.Host, "error", err)
		http.Error(w, "preview resolver failed", http.StatusBadGateway)
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
	if len(resolution.SetCookies) > 0 && r.URL.Query().Has("preview_token") {
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, previewGatewayCleanURL(r), http.StatusFound)
		return true
	}

	target, err := parseAndValidatePreviewTarget(resolution.Target)
	if err != nil {
		logging.GetLogger().Warnw("preview gateway rejected target", "host", r.Host, "target", resolution.Target, "error", err)
		http.Error(w, "preview target rejected", http.StatusBadGateway)
		return true
	}

	proxyPreviewRequest(w, r, target)
	return true
}

func previewGatewayCleanURL(r *http.Request) string {
	if r == nil || r.URL == nil {
		return "/"
	}
	cleanURL := *r.URL
	query := cleanURL.Query()
	query.Del("preview_token")
	cleanURL.RawQuery = query.Encode()
	if cleanURL.Path == "" {
		cleanURL.Path = "/"
	}
	return cleanURL.RequestURI()
}

func validatePreviewGatewayConfig(config shared.PreviewGatewayConfig) error {
	if !config.Enabled {
		return nil
	}
	if normalizePreviewDomain(config.Domain) == "" {
		return fmt.Errorf("preview gateway requires preview domain")
	}
	resolver := strings.TrimSpace(config.Resolver)
	if resolver == "" {
		return fmt.Errorf("preview gateway requires preview resolver")
	}
	parsed, err := url.Parse(resolver)
	if err != nil {
		return fmt.Errorf("preview resolver is invalid: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("preview resolver must use http or https")
	}
	if parsed.Host == "" {
		return fmt.Errorf("preview resolver host is empty")
	}
	return nil
}

func previewGatewayMatches(config shared.PreviewGatewayConfig, r *http.Request) bool {
	if !config.Enabled || r == nil {
		return false
	}
	domain := normalizePreviewDomain(config.Domain)
	if domain == "" || strings.TrimSpace(config.Resolver) == "" {
		return false
	}
	host := requestHostWithoutPort(r.Host)
	if host == "" {
		return false
	}
	host = strings.ToLower(host)
	if strings.EqualFold(host, domain) || !strings.HasSuffix(host, "."+strings.ToLower(domain)) {
		return false
	}
	label := strings.TrimSuffix(host, "."+strings.ToLower(domain))
	return strings.Contains(label, "--")
}

func normalizePreviewDomain(domain string) string {
	domain = strings.TrimSpace(strings.ToLower(domain))
	domain = strings.TrimPrefix(domain, "*.")
	domain = strings.TrimPrefix(domain, ".")
	return strings.TrimSuffix(domain, ".")
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

func resolvePreviewGatewayTarget(ctx context.Context, config shared.PreviewGatewayConfig, r *http.Request) (previewGatewayResolveResponse, error) {
	resolverURL := strings.TrimSpace(config.Resolver)
	if resolverURL == "" {
		return previewGatewayResolveResponse{}, fmt.Errorf("preview resolver is empty")
	}

	payload := previewGatewayResolveRequest{
		Host:     r.Host,
		Method:   r.Method,
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return previewGatewayResolveResponse{}, err
	}

	resolveCtx, cancel := context.WithTimeout(ctx, previewGatewayTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(resolveCtx, http.MethodPost, resolverURL, bytes.NewReader(body))
	if err != nil {
		return previewGatewayResolveResponse{}, err
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
		return previewGatewayResolveResponse{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return previewGatewayResolveResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return previewGatewayResolveResponse{
			Allowed: false,
			Status:  resp.StatusCode,
			Message: strings.TrimSpace(string(respBody)),
		}, nil
	}

	var resolution previewGatewayResolveResponse
	if err := json.Unmarshal(respBody, &resolution); err != nil {
		return previewGatewayResolveResponse{}, err
	}
	return resolution, nil
}

func apiHTTPClient() *http.Client {
	return &http.Client{Timeout: previewGatewayTimeout}
}

func parseAndValidatePreviewTarget(rawTarget string) (*url.URL, error) {
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
	if !previewTargetHostAllowed(target.Hostname()) {
		return nil, fmt.Errorf("target host is not loopback or private")
	}
	return target, nil
}

func previewTargetHostAllowed(host string) bool {
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

func proxyPreviewRequest(w http.ResponseWriter, r *http.Request, target *url.URL) {
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
		req.Header.Set("X-Hyperbricks-Preview-Host", originalHost)
		req.Header.Set("X-Forwarded-Proto", originalProto)
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		rewritePreviewLocationHeader(resp, target, originalHost, originalProto)
		return nil
	}
	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		logging.GetLogger().Warnw("preview gateway proxy failed", "host", originalHost, "target", target.String(), "error", err)
		http.Error(rw, "preview proxy failed", http.StatusBadGateway)
	}
	proxy.ServeHTTP(w, r)
}

func rewritePreviewLocationHeader(resp *http.Response, target *url.URL, originalHost string, originalProto string) {
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
