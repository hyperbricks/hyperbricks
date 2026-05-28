package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/shared/apiutil"
	"github.com/mitchellh/mapstructure"
	"github.com/yosssi/gohtml"
)

const (
	liveCacheRenderedAtHeader = "X-Hyperbricks-Rendered-At"
	liveCacheExpiresAtHeader  = "X-Hyperbricks-Cache-Expires-At"
	requestIDHeader           = "X-Hyperbricks-Request-ID"
	renderErrorCountHeader    = "X-Hyperbricks-Render-Error-Count"
	maxRenderDiagnostics      = 200
)

func routeSupportsGuard(configType string) bool {
	switch configType {
	case composite.HyperMediaConfigGetName(), composite.FragmentConfigGetName(), composite.ApiFragmentRenderConfigGetName():
		return true
	default:
		return false
	}
}

func resolveRouteGuard(config map[string]interface{}) (composite.RouteGuardConfig, bool) {
	if config == nil {
		return composite.RouteGuardConfig{}, false
	}
	configType, _ := config["@type"].(string)
	if !routeSupportsGuard(configType) {
		return composite.RouteGuardConfig{}, false
	}
	rawGuard, ok := config["guard"]
	if !ok || rawGuard == nil {
		return composite.RouteGuardConfig{}, false
	}
	var guard composite.RouteGuardConfig
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           &guard,
		TagName:          "mapstructure",
	})
	if err != nil {
		logging.GetLogger().Warnw("guard resolver: decoder setup failed", "route", config["route"], "error", err)
		return composite.RouteGuardConfig{}, false
	}
	if err := decoder.Decode(rawGuard); err != nil {
		logging.GetLogger().Warnw("guard resolver: decode failed", "route", config["route"], "error", err)
		return composite.RouteGuardConfig{}, false
	}
	if !guard.Enabled {
		return composite.RouteGuardConfig{}, false
	}
	return guard, true
}

func requestUsesHTMX(r *http.Request) bool {
	if r == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("HX-Request")), "true")
}

func resolveGuardToken(r *http.Request, guard *composite.RouteGuardConfig) string {
	if r == nil {
		return ""
	}
	if guard != nil {
		if cookieName := strings.TrimSpace(guard.Auth.Cookie); cookieName != "" {
			if cookie, err := r.Cookie(cookieName); err == nil {
				return strings.TrimSpace(cookie.Value)
			}
		}
	}
	headerName := "Authorization"
	scheme := "Bearer"
	if guard != nil {
		if trimmed := strings.TrimSpace(guard.Auth.Header); trimmed != "" {
			headerName = trimmed
		}
		if trimmed := strings.TrimSpace(guard.Auth.Scheme); trimmed != "" {
			scheme = trimmed
		}
	}
	headerValue := strings.TrimSpace(r.Header.Get(headerName))
	if headerValue == "" {
		return ""
	}
	if strings.EqualFold(headerName, "Authorization") {
		prefix := scheme + " "
		if strings.HasPrefix(headerValue, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(headerValue, prefix))
		}
		return ""
	}
	if scheme != "" {
		prefix := scheme + " "
		if strings.HasPrefix(headerValue, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(headerValue, prefix))
		}
	}
	return headerValue
}

func guardPlaceholderValues(r *http.Request) map[string]string {
	values := map[string]string{}
	if r == nil {
		return values
	}
	for key, vals := range r.URL.Query() {
		if len(vals) > 0 {
			values[key] = vals[0]
		}
	}
	if err := r.ParseForm(); err == nil {
		for key, vals := range r.Form {
			if len(vals) > 0 {
				values[key] = vals[0]
			}
		}
	}
	return values
}

func applyGuardPlaceholders(input string, values map[string]string) string {
	result := input
	for key, value := range values {
		re := regexp.MustCompile(`\$\b` + regexp.QuoteMeta(key) + `\b`)
		result = re.ReplaceAllString(result, value)
	}
	return result
}

func missingGuardQueryKeys(r *http.Request, guard composite.RouteGuardConfig) []string {
	if len(guard.Require.Query) == 0 || r == nil {
		return nil
	}
	query := r.URL.Query()
	missing := make([]string, 0)
	for key, raw := range guard.Require.Query {
		if !resolveNoCacheValue(raw) {
			continue
		}
		if strings.TrimSpace(query.Get(key)) == "" {
			missing = append(missing, key)
		}
	}
	return missing
}

func guardDeniedResponse(r *http.Request, action composite.RouteGuardActionConfig, fallbackStatus int, fallbackContent string) RenderContent {
	headers := map[string]string{
		"Cache-Control": "no-store",
		"Vary":          "Cookie, Authorization, HX-Request",
	}
	status := action.Status
	if requestUsesHTMX(r) {
		target := strings.TrimSpace(action.HxRedirect)
		if target == "" {
			target = strings.TrimSpace(action.Redirect)
		}
		if target != "" {
			headers["HX-Redirect"] = target
		}
		if status == 0 {
			status = fallbackStatus
		}
	} else {
		target := strings.TrimSpace(action.Redirect)
		if target != "" {
			headers["Location"] = target
			if status == 0 {
				status = http.StatusSeeOther
			}
		}
		if status == 0 {
			status = fallbackStatus
		}
	}
	if fallbackContent == "" {
		fallbackContent = http.StatusText(status)
	}
	return RenderContent{
		Content:     fallbackContent,
		NoCache:     true,
		ContentType: "text/plain; charset=utf-8",
		Status:      status,
		Headers:     headers,
	}
}

func authorizeRouteGuard(r *http.Request, guard composite.RouteGuardConfig, token string) (int, error) {
	if guard.Authorize == nil || strings.TrimSpace(guard.Authorize.Endpoint) == "" {
		return http.StatusOK, nil
	}
	values := guardPlaceholderValues(r)
	endpoint := applyGuardPlaceholders(strings.TrimSpace(guard.Authorize.Endpoint), values)
	body := applyGuardPlaceholders(guard.Authorize.Body, values)
	method := strings.ToUpper(strings.TrimSpace(guard.Authorize.Method))
	if method == "" {
		if body != "" {
			method = http.MethodPost
		} else {
			method = http.MethodGet
		}
	}
	req, err := http.NewRequest(method, endpoint, strings.NewReader(body))
	if err != nil {
		return 0, err
	}
	for key, value := range guard.Authorize.Headers {
		req.Header.Set(strings.TrimSpace(key), applyGuardPlaceholders(value, values))
	}
	if body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" && req.Header.Get("Authorization") == "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := apiutil.NewHTTPClient().Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

func evaluateRouteGuard(config map[string]interface{}, r *http.Request) (*RenderContent, string) {
	guard, enabled := resolveRouteGuard(config)
	if !enabled {
		return nil, resolveGuardToken(r, nil)
	}
	token := resolveGuardToken(r, &guard)
	if guard.Require.Authenticated && token == "" {
		response := guardDeniedResponse(r, guard.OnUnauthenticated, http.StatusUnauthorized, "authentication required")
		return &response, token
	}
	if missing := missingGuardQueryKeys(r, guard); len(missing) > 0 {
		response := guardDeniedResponse(r, guard.OnForbidden, http.StatusForbidden, "missing required query keys")
		return &response, token
	}
	status, err := authorizeRouteGuard(r, guard, token)
	if err != nil {
		response := RenderContent{
			Content:     "guard authorization failed",
			NoCache:     true,
			ContentType: "text/plain; charset=utf-8",
			Status:      http.StatusBadGateway,
			Headers: map[string]string{
				"Cache-Control": "no-store",
				"Vary":          "Cookie, Authorization, HX-Request",
			},
		}
		return &response, token
	}
	switch {
	case status >= 200 && status < 300:
		return nil, token
	case status == http.StatusUnauthorized:
		response := guardDeniedResponse(r, guard.OnUnauthenticated, http.StatusUnauthorized, "authentication required")
		return &response, token
	case status == http.StatusForbidden || status == http.StatusNotAcceptable:
		response := guardDeniedResponse(r, guard.OnForbidden, http.StatusForbidden, "forbidden")
		return &response, token
	default:
		response := RenderContent{
			Content:     "guard authorization rejected the request",
			NoCache:     true,
			ContentType: "text/plain; charset=utf-8",
			Status:      http.StatusBadGateway,
			Headers: map[string]string{
				"Cache-Control": "no-store",
				"Vary":          "Cookie, Authorization, HX-Request",
			},
		}
		return &response, token
	}
}

func resolveBeautify(config map[string]interface{}, defaultValue bool) bool {
	raw, ok := config["beautify"]
	if !ok {
		return defaultValue
	}

	switch value := raw.(type) {
	case bool:
		return value
	case string:
		parsed, err := strconv.ParseBool(value)
		if err == nil {
			return parsed
		}
	}

	return defaultValue
}

func resolveNoCacheValue(raw interface{}) bool {
	switch value := raw.(type) {
	case bool:
		return value
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		if err == nil {
			return parsed
		}
	}

	return false
}

func resolveConfiguredNoCache(config map[string]interface{}) bool {
	if config == nil {
		return false
	}

	if rawNoCache, ok := config["nocache"]; ok && resolveNoCacheValue(rawNoCache) {
		return true
	}

	configType, _ := config["@type"].(string)
	if configType == composite.ApiFragmentRenderConfigGetName() {
		return true
	}
	_, guardEnabled := resolveRouteGuard(config)
	return guardEnabled
}

func routeConfiguredNoCache(route string) bool {
	if route == "favicon.ico" {
		return false
	}

	config, found := getConfig(route)
	if !found {
		var fallbackFound bool
		config, fallbackFound = getConfig("404")
		if !fallbackFound {
			return false
		}
	}

	return resolveConfiguredNoCache(config)
}

func extractResponseHeaders(raw map[string]interface{}) map[string]string {
	value, ok := raw["headers"]
	if !ok || value == nil {
		return nil
	}

	headers := make(map[string]string)

	switch typed := value.(type) {
	case map[string]string:
		for key, val := range typed {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			headers[key] = strings.TrimSpace(val)
		}
	case map[string]interface{}:
		for key, val := range typed {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			headers[key] = strings.TrimSpace(fmt.Sprintf("%v", val))
		}
	case map[interface{}]interface{}:
		for key, val := range typed {
			keyStr, ok := key.(string)
			if !ok {
				continue
			}
			keyStr = strings.TrimSpace(keyStr)
			if keyStr == "" {
				continue
			}
			headers[keyStr] = strings.TrimSpace(fmt.Sprintf("%v", val))
		}
	}

	if len(headers) == 0 {
		return nil
	}
	return headers
}

func extractResponseCookies(raw map[string]interface{}) []string {
	value, ok := raw["cookies"]
	if !ok || value == nil {
		return nil
	}

	var cookies []string
	addCookie := func(val string) {
		val = strings.TrimSpace(val)
		if val == "" {
			return
		}
		cookies = append(cookies, val)
	}

	switch typed := value.(type) {
	case []string:
		for _, val := range typed {
			addCookie(val)
		}
	case []interface{}:
		for _, val := range typed {
			if val == nil {
				continue
			}
			addCookie(fmt.Sprintf("%v", val))
		}
	case string:
		addCookie(typed)
	default:
		addCookie(fmt.Sprintf("%v", typed))
	}

	if len(cookies) == 0 {
		return nil
	}
	return cookies
}

func applyResponseHeaders(headers map[string]string, writer http.ResponseWriter) {
	for key, val := range headers {
		if strings.TrimSpace(key) == "" {
			continue
		}
		writer.Header().Set(key, val)
	}
}

func applyResponseCookies(cookies []string, writer http.ResponseWriter) {
	for _, cookie := range cookies {
		cookie = strings.TrimSpace(cookie)
		if cookie == "" {
			continue
		}
		writer.Header().Add("Set-Cookie", cookie)
	}
}

func headerContentType(headers map[string]string) string {
	for key, val := range headers {
		if strings.EqualFold(key, "Content-Type") {
			return val
		}
	}
	return ""
}

func applyLiveCacheMetadataHeaders(headers map[string]string, renderedAt string, expiresAt string, etag string) map[string]string {
	if headers == nil {
		headers = make(map[string]string)
	}
	headers[liveCacheRenderedAtHeader] = renderedAt
	headers[liveCacheExpiresAtHeader] = expiresAt
	if etag != "" {
		headers["ETag"] = etag
	}
	return headers
}

func liveCacheETag(content string) string {
	if content == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(content))
	return `"hb-` + hex.EncodeToString(sum[:]) + `"`
}

func resolveLiveCacheKey(route string, r *http.Request) (string, bool) {
	if r == nil {
		return route, true
	}

	signature, hasVariant, err := requestVariantSignature(r)
	if err != nil {
		logging.GetLogger().Warnw("Failed to build live-mode cache key", "route", route, "error", err)
		return "", false
	}

	if !hasVariant {
		return route, true
	}

	return route + "|" + signature, true
}

func requestVariantSignature(r *http.Request) (string, bool, error) {
	var variant bytes.Buffer
	hasVariant := false

	if r.Method != "" && r.Method != http.MethodGet && r.Method != http.MethodHead {
		hasVariant = true
		variant.WriteString("method=")
		variant.WriteString(r.Method)
		variant.WriteByte('\n')
	}

	if r.URL != nil {
		if query := r.URL.Query().Encode(); query != "" {
			hasVariant = true
			variant.WriteString("query=")
			variant.WriteString(query)
			variant.WriteByte('\n')
		}
	}

	if auth := strings.TrimSpace(r.Header.Get("Authorization")); auth != "" {
		hasVariant = true
		variant.WriteString("authorization=")
		variant.WriteString(auth)
		variant.WriteByte('\n')
	}

	if cookie := strings.TrimSpace(r.Header.Get("Cookie")); cookie != "" {
		hasVariant = true
		variant.WriteString("cookie=")
		variant.WriteString(cookie)
		variant.WriteByte('\n')
	}

	body, err := cloneRequestBody(r)
	if err != nil {
		return "", false, err
	}
	if len(body) > 0 {
		hasVariant = true
		variant.WriteString("body=")
		variant.Write(body)
		variant.WriteByte('\n')
	}

	if !hasVariant {
		return "", false, nil
	}

	sum := sha256.Sum256(variant.Bytes())
	return hex.EncodeToString(sum[:]), true, nil
}

func cloneRequestBody(r *http.Request) ([]byte, error) {
	if r == nil || r.Body == nil || r.Body == http.NoBody {
		return nil, nil
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	if err := r.Body.Close(); err != nil {
		return nil, err
	}

	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func routeNeedsAPIRequestContext(node interface{}) bool {
	switch typed := node.(type) {
	case map[string]interface{}:
		if configType, ok := typed["@type"].(string); ok {
			if configType == component.APIConfigGetName() || configType == composite.ApiFragmentRenderConfigGetName() {
				return true
			}
		}
		for _, value := range typed {
			if routeNeedsAPIRequestContext(value) {
				return true
			}
		}
	case []interface{}:
		for _, value := range typed {
			if routeNeedsAPIRequestContext(value) {
				return true
			}
		}
	}
	return false
}

func resolveRoute(route string, routing shared.RoutingConfig) (string, bool) {
	routing = normalizeRoutingConfig(routing)
	route = strings.Trim(route, "/")
	if route == "" {
		if _, ok := getConfig("index"); ok {
			return "index", true
		}
		for _, indexFile := range routing.IndexFiles {
			if _, ok := getConfig(indexFile); ok {
				return indexFile, true
			}
		}
		return "", false
	}

	if _, ok := getConfig(route); ok {
		return route, true
	}

	if !routing.CleanURLs {
		return "", false
	}

	ext := strings.TrimPrefix(strings.ToLower(path.Ext(route)), ".")
	if ext != "" {
		for _, allowed := range routing.Extensions {
			if ext != strings.ToLower(allowed) {
				continue
			}
			trimmed := strings.TrimSuffix(route, "."+ext)
			if _, ok := getConfig(trimmed); ok {
				return trimmed, true
			}
			break
		}
	} else {
		base := path.Base(route)
		if !strings.Contains(base, ".") {
			for _, allowed := range routing.Extensions {
				allowed = strings.ToLower(strings.TrimPrefix(allowed, "."))
				if allowed == "" {
					continue
				}
				candidate := route + "." + allowed
				if _, ok := getConfig(candidate); ok {
					return candidate, true
				}
			}
		}
	}

	return "", false
}

func renderStaticContentFromConfig(config map[string]interface{}, routeOverride string, ctx context.Context) string {
	hbConfig := getHyperBricksConfiguration()

	configCopy := make(map[string]interface{}, len(config))
	for key, value := range config {
		configCopy[key] = value
	}

	route := ""
	if strings.TrimSpace(routeOverride) != "" {
		route = strings.TrimSpace(routeOverride)
		configCopy["route"] = route
	} else if routeValue, ok := configCopy["route"].(string); ok {
		route = routeValue
	}

	if ctx == nil {
		ctx = context.Background()
	}
	if route != "" {
		ctx = context.WithValue(ctx, shared.CurrentRoute, route)
	}

	var htmlContent strings.Builder

	renderOutput, _ := rm.Render(configCopy["@type"].(string), configCopy, ctx)

	htmlContent.WriteString(renderOutput)
	var output strings.Builder

	if resolveBeautify(configCopy, hbConfig.Server.Beautify) {
		output.WriteString(gohtml.Format(htmlContent.String()))
	} else {
		output.WriteString(htmlContent.String())
	}

	return output.String()
}

// func renderStaticContent(route string, ctx context.Context) string {
// 	_config, found := getConfig(route)

// 	if !found {
// 		__config, _found := getConfig("404")
// 		if _found {
// 			logging.GetLogger().Info("Redirecting to 404", " from ", route)
// 			_config = __config
// 		} else {
// 			if route == "favicon.ico" {
// 				return ""
// 			}
// 			logging.GetLogger().Info("Config not found for route: ", route)
// 			return fmt.Sprintf("Expected Hyperbricks '%s' was not found.", route)
// 		}
// 	}

// 	return renderStaticContentFromConfig(_config, "", ctx)
// }

type RenderContent struct {
	Content     string
	NoCache     bool
	ContentType string
	Status      int
	Headers     map[string]string
	Cookies     []string
	RequestID   string
	ErrorCount  int
	Handled     *shared.HandledResponse
}

func renderContent(w http.ResponseWriter, route string, r *http.Request, requestID string) RenderContent {
	hbConfig := getHyperBricksConfiguration()
	nocache := false
	status := http.StatusOK

	_config, found := getConfig(route)
	headers := map[string]string(nil)
	cookies := []string(nil)

	if !found {
		__config, _found := getConfig("404")
		if _found {
			logging.GetLogger().Info("Redirecting to 404", " from ", route)
			_config = __config
			status = http.StatusNotFound
		} else {
			if route == "favicon.ico" {
				return RenderContent{
					Content:     "",
					NoCache:     false,
					ContentType: "",
					Status:      http.StatusNoContent,
					RequestID:   requestID,
				}
			}
			logging.GetLogger().Info("Config not found for route: ", route)
			return RenderContent{
				Content:     fmt.Sprintf("Expected Hyperbricks '%s' was not found.", route),
				NoCache:     false,
				ContentType: "",
				Status:      http.StatusNotFound,
				RequestID:   requestID,
			}
		}
	}

	guardResponse, jwtToken := evaluateRouteGuard(_config, r)
	if guardResponse != nil {
		return *guardResponse
	}

	nocache = resolveConfiguredNoCache(_config)
	var contentType = ""
	if ct, ok := _config["content_type"].(string); ok {
		contentType = ct
	}
	if configType, ok := _config["@type"].(string); ok && configType == composite.HyperMediaConfigGetName() {
		headers = extractResponseHeaders(_config)
		cookies = extractResponseCookies(_config)
	}
	if contentType == "" && len(headers) > 0 {
		contentType = headerContentType(headers)
	}

	configCopy := make(map[string]interface{})
	for key, value := range _config {
		configCopy[key] = value
	}

	// TO DO: Clean this  up if possible use context (see ctx definition)
	if configCopy["@type"].(string) == composite.FragmentConfigGetName() {
		configCopy["hx_response"] = w
	}

	if configCopy["@type"].(string) == composite.ApiFragmentRenderConfigGetName() {
		configCopy["hx_response"] = w
	}

	// ============ START OF API CONTEXT AND TOKEN CAPTURE ============
	var requestBodyReader io.ReadCloser = http.NoBody
	needsAPIRequestContext := routeNeedsAPIRequestContext(configCopy)
	if needsAPIRequestContext {
		requestBodyBytes, err := cloneRequestBody(r)
		if err != nil {
			fmt.Println("Failed to clone request body:", err)
		} else if requestBodyBytes != nil {
			requestBodyReader = io.NopCloser(bytes.NewReader(requestBodyBytes))
		}

		// Parse form data before using r.Form.
		if err := r.ParseForm(); err != nil {
			fmt.Println("Failed to parse form data:", err)
		}
	}

	// Store JWT token in request context
	ctx := context.WithValue(r.Context(), shared.JwtKey, jwtToken)
	ctx = context.WithValue(ctx, shared.RequestBody, requestBodyReader) // Store a readable body copy in context
	ctx = context.WithValue(ctx, shared.FormData, r.Form)               // Store form data in context
	ctx = context.WithValue(ctx, shared.Request, r)
	ctx = context.WithValue(ctx, shared.CurrentRoute, route)

	//TO-DO: I know this is 'not how to do this', but because it stays within the concurrent proof HTTP lifecycle it is a practical solution for passing the ResponseWriter around
	ctx = context.WithValue(ctx, shared.ResponseWriter, w)
	handledCapture := &shared.HandledResponseCapture{}
	ctx = context.WithValue(ctx, shared.HandledResponseCaptureKey, handledCapture)
	// ============ END OF API CONTEXT AND TOKEN CAPTURE ============

	var htmlContent strings.Builder

	renderOutput, renderErrors := rm.Render(configCopy["@type"].(string), configCopy, ctx)
	renderErrors = append(renderErrors, getRouteSourceErrors(route)...)

	htmlContent.WriteString(renderOutput)
	var output strings.Builder

	if resolveBeautify(configCopy, hbConfig.Server.Beautify) {
		output.WriteString(gohtml.Format(htmlContent.String()))
	} else {
		output.WriteString(htmlContent.String())
	}

	if hbConfig.Mode != shared.LIVE_MODE {
		recordRenderDiagnostics(requestID, route, renderErrors)
	}

	if handledCapture != nil && handledCapture.Response != nil {
		return renderHandledContent(requestID, status, contentType, headers, cookies, nocache, len(renderErrors), handledCapture.Response)
	}

	return RenderContent{
		Content:     output.String(),
		NoCache:     nocache,
		ContentType: contentType,
		Status:      status,
		Headers:     headers,
		Cookies:     cookies,
		RequestID:   requestID,
		ErrorCount:  len(renderErrors),
	}

}

func renderHandledContent(requestID string, defaultStatus int, defaultContentType string, defaultHeaders map[string]string, defaultCookies []string, defaultNoCache bool, errorCount int, handled *shared.HandledResponse) RenderContent {
	if handled == nil {
		return RenderContent{
			NoCache:    true,
			RequestID:  requestID,
			ErrorCount: errorCount,
		}
	}

	status := handled.Status
	if status == 0 {
		status = defaultStatus
	}
	if status == 0 {
		status = http.StatusOK
	}

	contentType := strings.TrimSpace(handled.ContentType)
	if contentType == "" {
		contentType = strings.TrimSpace(defaultContentType)
	}

	headers := cloneStringMap(defaultHeaders)
	if headers == nil && len(handled.Headers) > 0 {
		headers = make(map[string]string, len(handled.Headers))
	}
	for key, value := range handled.Headers {
		headers[key] = value
	}

	cookies := append([]string(nil), defaultCookies...)
	if len(handled.Cookies) > 0 {
		cookies = append(cookies, handled.Cookies...)
	}

	return RenderContent{
		NoCache:     true,
		ContentType: contentType,
		Status:      status,
		Headers:     headers,
		Cookies:     cookies,
		RequestID:   requestID,
		ErrorCount:  errorCount,
		Handled:     cloneHandledResponseData(handled),
	}
}

func cloneStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func cloneHandledResponseData(response *shared.HandledResponse) *shared.HandledResponse {
	if response == nil {
		return nil
	}

	cloned := &shared.HandledResponse{
		Status:      response.Status,
		ContentType: response.ContentType,
		NoCache:     response.NoCache,
	}
	if len(response.Body) > 0 {
		cloned.Body = append([]byte(nil), response.Body...)
	}
	if len(response.Headers) > 0 {
		cloned.Headers = cloneStringMap(response.Headers)
	}
	if len(response.Cookies) > 0 {
		cloned.Cookies = append([]string(nil), response.Cookies...)
	}
	return cloned
}

// errorTemplate is the embedded Go template as a string
// {{safe "<!--  Begin Frontend Errors [development.frontend_errors = true] in package.hyperbricks.yaml  -->"}}
// {{safe "<!-- No Errors -->"}}{{end}}
const errorTemplate = `{{if .HasErrors}}
	<script>
	{{range .Errors}} document.getElementById("error_list").innerHTML += '<li><span class="error_message">\n' +
				'	<div class="error_error">{{.Err}}</div>\n' +
				'	type <span class="error_type error_mark"></span> at file\n' +
				'	<span class="error_file error_mark">{{.File}}</span> at \n' +
				'	<span class="error_path error_mark"> {{.Path}}.{{.Key}}</span> \n' +
				'	</span>\n' +
				'</li>\n';
		{{end}}
		document.getElementById("error_panel").style.display = "flex";
	</script>
{{else}}
	{{safe "<!-- No Errors -->"}}
{{end}}`

// ErrorData holds the errors and a flag to determine if there are any
type ErrorData struct {
	HasErrors bool
	Errors    []ComponentErrorTemplate
}

// HandleRenderErrors processes errors and returns a string with formatted errors
func FrontEndErrorRender(renderErrors []error) string {
	var errorsList []ComponentErrorTemplate

	for _, err := range renderErrors {
		if componentError, ok := err.(shared.ComponentError); ok {
			errorsList = append(errorsList, ComponentErrorTemplate{
				Hash: componentError.Hash,
				File: componentError.File,
				Type: componentError.Type,
				Path: componentError.Path,
				Key:  componentError.Key,
				Err:  componentError.Err,
			})
		} else {
			errorsList = append(errorsList, ComponentErrorTemplate{
				File: "Unknown",
				Type: "Unknown",
				Path: "Unknown",
				Key:  "Unknown",
				Err:  fmt.Sprintf("%v", err),
			})
		}
	}

	data := ErrorData{
		HasErrors: len(errorsList) > 0,
		Errors:    errorsList,
	}

	// Parse the embedded template
	tmpl, err := template.New("errorTemplate").Funcs(template.FuncMap{
		"safe": func(s string) template.HTML { return template.HTML(s) },
	}).Parse(errorTemplate)
	if err != nil {
		log.Println("Error parsing template:", err)
		return ""
	}

	// Render the template to a string
	var output bytes.Buffer
	if err := tmpl.Execute(&output, data); err != nil {
		log.Println("Error rendering template:", err)
		return ""
	}

	return output.String()
}

func HandleRenderErrors(renderErrors []error) string {
	if len(renderErrors) == 0 {
		return ""
	}
	errors := ""
	for e := range renderErrors {

		componentError, ok := renderErrors[e].(shared.ComponentError)
		if ok {
			errors += "<!-- Error " + fmt.Sprintf(`in file: %s at %s.%s|%v`, componentError.File, componentError.Path, componentError.Key, componentError.Err) + " -->\n"
		} else {
			e := error(renderErrors[e])
			errors += "<!-- Error:" + fmt.Sprintf("%v", e) + " -->\n"
		}
	}

	if errors != "" {
		return errors
	}
	return ""
}

func nextRenderRequestID() string {
	sequence := atomic.AddInt64(&renderDiagnosticsSeq, 1)
	return "hb-" + strconv.FormatInt(sequence, 10)
}

func recordRenderDiagnostics(requestID string, route string, renderErrors []error) {
	if len(renderErrors) == 0 {
		return
	}

	diagnostics := RenderDiagnostics{
		RequestID: requestID,
		Route:     route,
		CreatedAt: time.Now().UTC(),
		Errors:    collectRenderDiagnostics(renderErrors),
	}

	renderDiagnosticsMutex.Lock()
	renderDiagnostics[requestID] = diagnostics
	renderDiagnosticsOrder = append(renderDiagnosticsOrder, requestID)
	for len(renderDiagnosticsOrder) > maxRenderDiagnostics {
		oldest := renderDiagnosticsOrder[0]
		renderDiagnosticsOrder = renderDiagnosticsOrder[1:]
		delete(renderDiagnostics, oldest)
	}
	renderDiagnosticsMutex.Unlock()

	if shouldLogRenderDiagnosticsAsError(renderErrors) {
		logging.GetLogger().Errorw("Render diagnostics recorded", "request_id", requestID, "route", route, "error_count", len(diagnostics.Errors))
	}
}

func shouldLogRenderDiagnosticsAsError(renderErrors []error) bool {
	for _, err := range renderErrors {
		componentError, ok := err.(shared.ComponentError)
		if ok && strings.EqualFold(componentError.Level, "WARNING") && !componentError.Rejected {
			continue
		}
		return true
	}
	return false
}

func collectRenderDiagnostics(renderErrors []error) []ComponentErrorTemplate {
	diagnostics := make([]ComponentErrorTemplate, 0, len(renderErrors))
	for _, err := range renderErrors {
		if componentError, ok := err.(shared.ComponentError); ok {
			diagnostics = append(diagnostics, ComponentErrorTemplate{
				Hash: componentError.Hash,
				File: componentError.File,
				Type: componentError.Type,
				Path: componentError.Path,
				Key:  componentError.Key,
				Err:  componentError.Err,
			})
			continue
		}

		diagnostics = append(diagnostics, ComponentErrorTemplate{
			File: "Unknown",
			Type: "Unknown",
			Path: "Unknown",
			Key:  "Unknown",
			Err:  fmt.Sprintf("%v", err),
		})
	}
	return diagnostics
}

func ServeContent(w http.ResponseWriter, r *http.Request) {
	hbConfig := getHyperBricksConfiguration()
	requestID := nextRenderRequestID()

	route := strings.Trim(r.URL.Path, "/")
	if resolvedRoute, ok := resolveRoute(route, hbConfig.Server.Routing); ok {
		route = resolvedRoute
	} else if route == "" {
		route = "index"
	}

	logging.GetLogger().Debugw("Received request for route", "route", route)
	if hbConfig.Mode == shared.LIVE_MODE {
		cacheEntry := handleLiveMode(w, route, r, requestID)
		if writeNotModifiedResponse(w, route, requestID, r, cacheEntry) {
			return
		}
		if !writeRenderResponse(w, route, requestID, cacheEntry.Content, cacheEntry.ContentLength, cacheEntry.Handled, cacheEntry.Headers, cacheEntry.Cookies, cacheEntry.ContentType, cacheEntry.Status, cacheEntry.ErrorCount) {
			return
		}
	} else {
		renderContent := handleDeveloperMode(w, route, r, requestID)
		if !writeRenderResponse(w, route, requestID, renderContent.Content, "", renderContent.Handled, renderContent.Headers, renderContent.Cookies, renderContent.ContentType, renderContent.Status, renderContent.ErrorCount) {
			return
		}
	}
}

func writeNotModifiedResponse(w http.ResponseWriter, route string, requestID string, r *http.Request, cacheEntry CacheEntry) bool {
	if !canUseNotModified(r, cacheEntry) {
		return false
	}
	applyResponseHeaders(cacheEntry.Headers, w)
	w.Header().Set(requestIDHeader, requestID)
	w.Header().Set(renderErrorCountHeader, strconv.Itoa(cacheEntry.ErrorCount))
	w.Header().Del("Content-Length")
	w.WriteHeader(http.StatusNotModified)
	logging.GetLogger().Debugw("Served not modified request", "route", route)
	return true
}

func canUseNotModified(r *http.Request, cacheEntry CacheEntry) bool {
	if r == nil || cacheEntry.ETag == "" || cacheEntry.Status != http.StatusOK || cacheEntry.Handled != nil || len(cacheEntry.Cookies) > 0 {
		return false
	}
	if r.Method != "" && r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	return headerHasETag(r.Header.Values("If-None-Match"), cacheEntry.ETag)
}

func headerHasETag(values []string, etag string) bool {
	for _, value := range values {
		for _, candidate := range strings.Split(value, ",") {
			candidate = comparableETag(candidate)
			if candidate == "*" || candidate == etag {
				return true
			}
		}
	}
	return false
}

func comparableETag(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "W/") {
		value = strings.TrimSpace(strings.TrimPrefix(value, "W/"))
	}
	return value
}

func writeRenderResponse(w http.ResponseWriter, route string, requestID string, content string, contentLength string, handled *shared.HandledResponse, headers map[string]string, cookies []string, contentType string, status int, errorCount int) bool {
	applyResponseHeaders(headers, w)
	applyResponseCookies(cookies, w)
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	} else {
		w.Header().Set("Content-Type", "text/html")
	}
	w.Header().Set(requestIDHeader, requestID)
	w.Header().Set(renderErrorCountHeader, strconv.Itoa(errorCount))
	if status == 0 {
		status = http.StatusOK
	}

	if handled != nil {
		if responseAllowsBody(status) {
			w.Header().Set("Content-Length", strconv.Itoa(len(handled.Body)))
		}
		body := handled.Body
		w.WriteHeader(status)
		if len(body) == 0 {
			logging.GetLogger().Debugw("Served request", "route", route)
			return true
		}
		if _, err := w.Write(body); err != nil {
			logging.GetLogger().Errorw("Error writing response", "route", route, "error", err)
			return false
		}
		logging.GetLogger().Debugw("Served request", "route", route)
		return true
	}

	if responseAllowsBody(status) {
		if contentLength == "" {
			contentLength = strconv.Itoa(len(content))
		}
		w.Header().Set("Content-Length", contentLength)
	}
	w.WriteHeader(status)
	if content == "" {
		logging.GetLogger().Debugw("Served request", "route", route)
		return true
	}
	if _, err := io.WriteString(w, content); err != nil {
		logging.GetLogger().Errorw("Error writing response", "route", route, "error", err)
		return false
	}
	logging.GetLogger().Debugw("Served request", "route", route)
	return true
}

func responseAllowsBody(status int) bool {
	return status != http.StatusNoContent && status != http.StatusNotModified
}

// RENDER WITHOUT CACHE
func handleDeveloperMode(w http.ResponseWriter, route string, r *http.Request, requestID string) RenderContent {
	logging.GetLogger().Debugw("Developer mode active. Rendering fresh content:", route)
	return renderContent(w, route, r, requestID)
}

// RENDER WITH CACHE
func handleLiveMode(w http.ResponseWriter, route string, r *http.Request, requestID string) CacheEntry {

	hbConfig := getHyperBricksConfiguration()
	cacheDuration := hbConfig.Live.CacheTime

	if routeConfiguredNoCache(route) {
		logging.GetLogger().Debugw("Skipping live cache for nocache route", "route", route)
		renderContent := renderContent(w, route, r, requestID)
		now := time.Now()
		return CacheEntry{
			Content:     renderContent.Content,
			Timestamp:   now,
			ContentType: renderContent.ContentType,
			Status:      renderContent.Status,
			Headers:     renderContent.Headers,
			Cookies:     renderContent.Cookies,
			ErrorCount:  renderContent.ErrorCount,
			Handled:     cloneHandledResponseData(renderContent.Handled),
		}
	}

	cacheKey, cacheable := resolveLiveCacheKey(route, r)

	var (
		cacheEntry CacheEntry
		found      bool
	)
	if cacheable {
		htmlCacheMutex.RLock()
		cacheEntry, found = htmlCache[cacheKey]
		htmlCacheMutex.RUnlock()
	}

	if found && time.Since(cacheEntry.Timestamp) <= cacheDuration.Duration {
		logging.GetLogger().Debugw("Cache hit for route", "route", route)
		return cacheEntry
	}

	if found {
		logging.GetLogger().Infof("Cache expired for route %s. Re-rendering content.", route)
	} else {
		logging.GetLogger().Debugf("Cache missing for route %s. Rendering content.", route)
	}

	//Calculate expiration time
	var now = time.Now()
	expirationTime := now.Add(cacheDuration.Duration).Format("2006-01-02 15:04:05 (-07:00)")
	renderTime := time.Now().Format("2006-01-02 15:04:05 (-07:00)")

	renderContent := renderContent(w, route, r, requestID)
	etag := ""
	contentLength := ""
	if !renderContent.NoCache {
		if cacheable && renderContent.Content != "" {
			etag = liveCacheETag(renderContent.Content)
			contentLength = strconv.Itoa(len(renderContent.Content))
			renderContent.Headers = applyLiveCacheMetadataHeaders(renderContent.Headers, renderTime, expirationTime, etag)
			htmlCacheMutex.Lock()
			htmlCache[cacheKey] = CacheEntry{
				Content:       renderContent.Content,
				ContentLength: contentLength,
				ETag:          etag,
				Timestamp:     now,
				ContentType:   renderContent.ContentType,
				Status:        renderContent.Status,
				Headers:       renderContent.Headers,
				Cookies:       renderContent.Cookies,
				ErrorCount:    renderContent.ErrorCount,
			}
			htmlCacheMutex.Unlock()
			logging.GetLogger().Debugw("Updated cache for route", "route", route)
		}
	}
	return CacheEntry{
		Content:       renderContent.Content,
		ContentLength: contentLength,
		ETag:          etag,
		Timestamp:     now,
		ContentType:   renderContent.ContentType,
		Status:        renderContent.Status,
		Headers:       renderContent.Headers,
		Cookies:       renderContent.Cookies,
		ErrorCount:    renderContent.ErrorCount,
		Handled:       cloneHandledResponseData(renderContent.Handled),
	}
}
