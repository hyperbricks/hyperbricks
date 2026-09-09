package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
)

const staticSnapshotTimeout = 30 * time.Second

type staticSnapshotTarget struct {
	RequestPath string
	OutputPath  string
	Query       url.Values
	Headers     map[string]string
	Host        string
	Source      string
}

type staticSnapshotRuntime struct {
	server  *http.Server
	baseURL string
	errs    chan error
}

func snapshotStaticRoutes(configs map[string]map[string]interface{}, renderDir string) error {
	targets, err := collectStaticSnapshotTargets(configs)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		logging.GetLogger().Warn("No static snapshot targets found")
		return nil
	}

	runtime, err := startStaticSnapshotRuntime()
	if err != nil {
		return err
	}
	defer func() {
		if err := runtime.close(); err != nil {
			logging.GetLogger().Warnw("Static snapshot runtime shutdown failed", "error", err)
		}
	}()

	client := &http.Client{
		Timeout: staticSnapshotTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	logger := logging.GetLogger()
	logger.Infow("Static snapshot runtime started", "base_url", runtime.baseURL, "target_count", len(targets))
	for _, target := range targets {
		logger.Infow("Snapshotting static route", "request", target.requestURI(), "output", target.OutputPath, "source", target.Source)
		body, err := fetchStaticSnapshotTarget(client, runtime.baseURL, target)
		if err != nil {
			return err
		}
		if err := writeStaticSnapshotFile(renderDir, target.OutputPath, body); err != nil {
			return err
		}
	}
	return nil
}

func startStaticSnapshotRuntime() (staticSnapshotRuntime, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return staticSnapshotRuntime{}, fmt.Errorf("start static snapshot listener: %w", err)
	}

	server := &http.Server{
		Handler: buildRuntimeHandler(nil),
	}
	errs := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errs <- err
		}
		close(errs)
	}()

	return staticSnapshotRuntime{
		server:  server,
		baseURL: "http://" + listener.Addr().String(),
		errs:    errs,
	}, nil
}

func (runtime staticSnapshotRuntime) close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := runtime.server.Shutdown(ctx); err != nil {
		return err
	}
	select {
	case err := <-runtime.errs:
		return err
	default:
		return nil
	}
}

func collectStaticSnapshotTargets(configs map[string]map[string]interface{}) ([]staticSnapshotTarget, error) {
	targets := make([]staticSnapshotTarget, 0, len(configs))
	configTargets, err := staticSnapshotTargetsFromPackageConfig()
	if err != nil {
		return nil, err
	}
	targets = append(targets, configTargets...)

	routes := make([]string, 0, len(configs))
	for route := range configs {
		routes = append(routes, route)
	}
	sort.Strings(routes)

	for _, route := range routes {
		target, ok, err := staticSnapshotTargetFromRoute(route, configs[route])
		if err != nil {
			return nil, err
		}
		if ok {
			targets = append(targets, target)
		}
	}

	return dedupeStaticSnapshotTargets(targets)
}

func staticSnapshotTargetFromRoute(route string, config map[string]interface{}) (staticSnapshotTarget, bool, error) {
	if config == nil {
		return staticSnapshotTarget{}, false, nil
	}

	requestRoute := strings.TrimSpace(route)
	if configuredRoute, ok := config["route"].(string); ok && strings.TrimSpace(configuredRoute) != "" {
		requestRoute = strings.TrimSpace(configuredRoute)
	}
	if requestRoute == "" {
		return staticSnapshotTarget{}, false, nil
	}

	requestPath := normalizeStaticRequestPath(requestRoute)
	output := defaultStaticOutputPath(requestPath)
	if staticPath, ok := config["static"].(string); ok && strings.TrimSpace(staticPath) != "" {
		output = strings.TrimSpace(staticPath)
	}
	outputPath, err := normalizeStaticOutputPath(output)
	if err != nil {
		return staticSnapshotTarget{}, false, fmt.Errorf("static route %s output: %w", route, err)
	}

	return staticSnapshotTarget{
		RequestPath: requestPath,
		OutputPath:  outputPath,
		Query:       url.Values{},
		Source:      "route:" + route,
	}, true, nil
}

func staticSnapshotTargetsFromPackageConfig() ([]staticSnapshotTarget, error) {
	staticConfig, err := packageStaticConfigMap()
	if err != nil {
		return nil, err
	}
	if len(staticConfig) == 0 {
		return nil, nil
	}

	targets := []staticSnapshotTarget{}
	for _, key := range []string{"routes", "variants"} {
		parsed, err := staticSnapshotTargetsFromList(staticConfig[key], "static."+key)
		if err != nil {
			return nil, err
		}
		targets = append(targets, parsed...)
	}

	if crawlRaw, exists := staticConfig["crawl"]; exists {
		crawl, ok := crawlRaw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("hyperbricks.static.crawl must be a mapping containing routes or variants lists")
		}
		for _, key := range []string{"routes", "variants"} {
			parsed, err := staticSnapshotTargetsFromList(crawl[key], "static.crawl."+key)
			if err != nil {
				return nil, err
			}
			targets = append(targets, parsed...)
		}
	}

	return targets, nil
}

func packageStaticConfigMap() (map[string]interface{}, error) {
	root, ok := parser.HbConfig["hyperbricks"].(map[string]interface{})
	if !ok {
		return nil, nil
	}
	staticRaw, ok := root["static"]
	if !ok {
		return nil, nil
	}
	staticConfig, ok := staticRaw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("hyperbricks.static must be a mapping containing routes or variants lists")
	}
	return staticConfig, nil
}

func staticSnapshotTargetsFromList(raw interface{}, source string) ([]staticSnapshotTarget, error) {
	if raw == nil {
		return nil, nil
	}
	items, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be a list", source)
	}

	targets := make([]staticSnapshotTarget, 0, len(items))
	for index, item := range items {
		entry, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be an object", source, index)
		}
		target, err := staticSnapshotTargetFromConfigEntry(entry, fmt.Sprintf("%s[%d]", source, index))
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, nil
}

func staticSnapshotTargetFromConfigEntry(entry map[string]interface{}, source string) (staticSnapshotTarget, error) {
	rawPath := firstString(entry, "path", "route", "url")
	if rawPath == "" {
		return staticSnapshotTarget{}, fmt.Errorf("%s.path is required", source)
	}

	requestPath, query, host, err := parseStaticSnapshotRequestPath(rawPath)
	if err != nil {
		return staticSnapshotTarget{}, fmt.Errorf("%s.path: %w", source, err)
	}
	appendStaticSnapshotQuery(query, entry["query"])
	appendStaticSnapshotQuery(query, entry["queryparams"])

	output := firstString(entry, "output", "static", "file")
	if output == "" {
		if len(query) > 0 {
			return staticSnapshotTarget{}, fmt.Errorf("%s.output is required when query parameters are configured", source)
		}
		output = defaultStaticOutputPath(requestPath)
	}
	outputPath, err := normalizeStaticOutputPath(output)
	if err != nil {
		return staticSnapshotTarget{}, fmt.Errorf("%s.output: %w", source, err)
	}

	if configuredHost := strings.TrimSpace(firstString(entry, "host")); configuredHost != "" {
		host = configuredHost
	}

	return staticSnapshotTarget{
		RequestPath: requestPath,
		OutputPath:  outputPath,
		Query:       query,
		Headers:     stringMapFromRaw(entry["headers"]),
		Host:        host,
		Source:      source,
	}, nil
}

func parseStaticSnapshotRequestPath(rawPath string) (string, url.Values, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawPath))
	if err != nil {
		return "", nil, "", err
	}
	host := ""
	if parsed.IsAbs() {
		host = parsed.Host
	}
	requestPath := parsed.Path
	if requestPath == "" {
		requestPath = "/"
	}
	return normalizeStaticRequestPath(requestPath), parsed.Query(), host, nil
}

func appendStaticSnapshotQuery(values url.Values, raw interface{}) {
	switch typed := raw.(type) {
	case map[string]string:
		for key, value := range typed {
			values.Add(key, value)
		}
	case map[string]interface{}:
		for key, value := range typed {
			appendStaticSnapshotQueryValue(values, key, value)
		}
	}
}

func appendStaticSnapshotQueryValue(values url.Values, key string, raw interface{}) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	switch typed := raw.(type) {
	case []interface{}:
		for _, value := range typed {
			values.Add(key, fmt.Sprint(value))
		}
	case []string:
		for _, value := range typed {
			values.Add(key, value)
		}
	default:
		values.Add(key, fmt.Sprint(raw))
	}
}

func dedupeStaticSnapshotTargets(targets []staticSnapshotTarget) ([]staticSnapshotTarget, error) {
	seen := map[string]staticSnapshotTarget{}
	out := make([]staticSnapshotTarget, 0, len(targets))
	for _, target := range targets {
		if target.OutputPath == "" {
			continue
		}
		existing, exists := seen[target.OutputPath]
		if exists {
			if existing.identity() == target.identity() {
				continue
			}
			if existing.requestURI() == target.requestURI() && isPackageStaticSnapshotSource(existing.Source) && isRouteStaticSnapshotSource(target.Source) {
				continue
			}
			return nil, fmt.Errorf("static output %q is configured more than once (%s and %s)", target.OutputPath, existing.Source, target.Source)
		}
		seen[target.OutputPath] = target
		out = append(out, target)
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].OutputPath < out[j].OutputPath
	})
	return out, nil
}

func fetchStaticSnapshotTarget(client *http.Client, baseURL string, target staticSnapshotTarget) ([]byte, error) {
	targetURL, err := target.url(baseURL)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create static snapshot request %s: %w", target.requestURI(), err)
	}
	if target.Host != "" {
		req.Host = target.Host
	}
	for key, value := range target.Headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch static snapshot %s: %w", target.requestURI(), err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read static snapshot %s: %w", target.requestURI(), err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("static snapshot %s returned %s: %s", target.requestURI(), resp.Status, previewStaticSnapshotBody(body))
	}
	if errorCount := parseRenderErrorCount(resp.Header.Get(renderErrorCountHeader)); errorCount > 0 {
		return nil, fmt.Errorf("static snapshot %s produced %d render error(s); request id %s", target.requestURI(), errorCount, resp.Header.Get(requestIDHeader))
	}
	if resp.Header.Get("Set-Cookie") != "" {
		logging.GetLogger().Warnw("Static snapshot response set cookies", "request", target.requestURI(), "output", target.OutputPath)
	}
	if strings.Contains(strings.ToLower(resp.Header.Get("Cache-Control")), "no-store") {
		logging.GetLogger().Warnw("Static snapshot response is marked no-store", "request", target.requestURI(), "output", target.OutputPath)
	}
	return body, nil
}

func writeStaticSnapshotFile(renderDir string, outputPath string, body []byte) error {
	targetPath, err := staticSnapshotOutputFile(renderDir, outputPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create static snapshot directory for %s: %w", outputPath, err)
	}
	if err := os.WriteFile(targetPath, body, 0o644); err != nil {
		return fmt.Errorf("write static snapshot %s: %w", outputPath, err)
	}
	return nil
}

func staticSnapshotOutputFile(renderDir string, outputPath string) (string, error) {
	normalized, err := normalizeStaticOutputPath(outputPath)
	if err != nil {
		return "", err
	}
	targetPath := filepath.Join(renderDir, filepath.FromSlash(normalized))
	cleanRoot := filepath.Clean(renderDir)
	cleanTarget := filepath.Clean(targetPath)
	if cleanTarget != cleanRoot && !strings.HasPrefix(cleanTarget, cleanRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("static output %q escapes render directory", outputPath)
	}
	return targetPath, nil
}

func normalizeStaticRequestPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "/"
	}
	cleaned := path.Clean("/" + strings.TrimPrefix(value, "/"))
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

func normalizeStaticOutputPath(value string) (string, error) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "/")
	if strings.HasSuffix(value, "/") {
		value += "index.html"
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == "" {
		return "", fmt.Errorf("static output path is required")
	}
	if strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return "", fmt.Errorf("static output path cannot escape render directory")
	}
	return cleaned, nil
}

func defaultStaticOutputPath(requestPath string) string {
	routing := normalizeRoutingConfig(getHyperBricksConfiguration().Server.Routing)
	cleaned := normalizeStaticRequestPath(requestPath)
	if cleaned == "/" {
		return routing.IndexFiles[0]
	}
	output := strings.TrimPrefix(cleaned, "/")
	if path.Ext(output) != "" {
		return output
	}
	return output + "." + routing.Extensions[0]
}

func (target staticSnapshotTarget) url(baseURL string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse static snapshot base URL: %w", err)
	}
	parsed.Path = target.RequestPath
	parsed.RawQuery = target.Query.Encode()
	return parsed.String(), nil
}

func (target staticSnapshotTarget) requestURI() string {
	u := url.URL{Path: target.RequestPath, RawQuery: target.Query.Encode()}
	return u.RequestURI()
}

func (target staticSnapshotTarget) identity() string {
	var builder strings.Builder
	builder.WriteString(target.requestURI())
	builder.WriteString("\nhost=")
	builder.WriteString(target.Host)
	if len(target.Headers) == 0 {
		return builder.String()
	}
	keys := make([]string, 0, len(target.Headers))
	for key := range target.Headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		builder.WriteString("\nheader:")
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(target.Headers[key])
	}
	return builder.String()
}

func isRouteStaticSnapshotSource(source string) bool {
	return strings.HasPrefix(source, "route:")
}

func isPackageStaticSnapshotSource(source string) bool {
	return !isRouteStaticSnapshotSource(source)
}

func firstString(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if raw, ok := values[key]; ok {
			value := strings.TrimSpace(fmt.Sprint(raw))
			if value != "" && value != "<nil>" {
				return value
			}
		}
	}
	return ""
}

func stringMapFromRaw(raw interface{}) map[string]string {
	switch typed := raw.(type) {
	case map[string]string:
		out := make(map[string]string, len(typed))
		for key, value := range typed {
			key = strings.TrimSpace(key)
			if key != "" {
				out[key] = value
			}
		}
		return out
	case map[string]interface{}:
		out := make(map[string]string, len(typed))
		for key, value := range typed {
			key = strings.TrimSpace(key)
			if key != "" {
				out[key] = fmt.Sprint(value)
			}
		}
		return out
	default:
		return nil
	}
}

func parseRenderErrorCount(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return parsed
}

func previewStaticSnapshotBody(body []byte) string {
	preview := strings.TrimSpace(string(body))
	if len(preview) > 300 {
		preview = preview[:300] + "..."
	}
	return preview
}
