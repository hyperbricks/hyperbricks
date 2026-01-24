package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/yosssi/gohtml"
)

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

	renderOutput, renderErrors := rm.Render(configCopy["@type"].(string), configCopy, ctx)

	htmlContent.WriteString(renderOutput)
	var output strings.Builder

	if resolveBeautify(configCopy, hbConfig.Server.Beautify) {
		output.WriteString(gohtml.Format(htmlContent.String()))
	} else {
		output.WriteString(htmlContent.String())
	}

	// only render errors in debug or development mode...
	if hbConfig.Mode != shared.LIVE_MODE {
		if hbConfig.Development.FrontendErrors {
			output.WriteString(FrontEndErrorRender(renderErrors))
		} else {
			output.WriteString(HandleRenderErrors(renderErrors))
		}
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
}

func renderContent(w http.ResponseWriter, route string, r *http.Request) RenderContent {
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
				}
			}
			logging.GetLogger().Info("Config not found for route: ", route)
			return RenderContent{
				Content:     fmt.Sprintf("Expected Hyperbricks '%s' was not found.", route),
				NoCache:     false,
				ContentType: "",
				Status:      http.StatusNotFound,
			}
		}
	}

	if _, ok := _config["nocache"].(string); ok {
		nocache = true
		// logging.GetLogger().Debugf("NoCache = true: %s from %s", val, route)
	}
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

		// No Caching!!! This is very important because of secret user-specific tokens
		nocache = true
	}

	// ============ START OF API CONTEXT AND TOKEN CAPTURE ============
	// Extract JWT token from the Authorization header for authentication if needed
	authHeader := r.Header.Get("Authorization")

	var jwtToken string
	if strings.HasPrefix(authHeader, "Bearer ") {
		jwtToken = strings.TrimPrefix(authHeader, "Bearer ")
	}

	// Parse form data before using r.Form
	if err := r.ParseForm(); err != nil {
		fmt.Println("Failed to parse form data:", err)
	}

	// Store JWT token in request context
	ctx := context.WithValue(r.Context(), shared.JwtKey, jwtToken)
	ctx = context.WithValue(ctx, shared.RequestBody, r.Body) // Store body data in context
	ctx = context.WithValue(ctx, shared.FormData, r.Form)    // Store form data in context
	ctx = context.WithValue(ctx, shared.Request, r)
	ctx = context.WithValue(ctx, shared.CurrentRoute, route)

	//TO-DO: I know this is 'not how to do this', but because it stays within the concurrent proof HTTP lifecycle it is a practical solution for passing the ResponseWriter around
	ctx = context.WithValue(ctx, shared.ResponseWriter, w)
	// ============ END OF API CONTEXT AND TOKEN CAPTURE ============

	var htmlContent strings.Builder

	renderOutput, renderErrors := rm.Render(configCopy["@type"].(string), configCopy, ctx)

	htmlContent.WriteString(renderOutput)
	var output strings.Builder

	if resolveBeautify(configCopy, hbConfig.Server.Beautify) {
		output.WriteString(gohtml.Format(htmlContent.String()))
	} else {
		output.WriteString(htmlContent.String())
	}

	// only render errors in debug or development mode...
	if hbConfig.Mode != shared.LIVE_MODE {
		if hbConfig.Development.FrontendErrors {
			output.WriteString(FrontEndErrorRender(renderErrors))
		} else {
			output.WriteString(HandleRenderErrors(renderErrors))
		}
	}

	return RenderContent{
		Content:     output.String(),
		NoCache:     nocache,
		ContentType: contentType,
		Status:      status,
		Headers:     headers,
		Cookies:     cookies,
	}

}

// ComponentErrorTemplate represents the structure for rendering errors
type ComponentErrorTemplate struct {
	Hash string
	Type string
	File string
	Path string
	Key  string
	Err  string
}

// errorTemplate is the embedded Go template as a string
// {{safe "<!--  Begin Frontend Errors [development.frontend_errors = true] in package.hyperbricks  -->"}}
// {{safe "<!-- No Errors -->"}}{{end}}
const errorTemplate = `{{if .HasErrors}}
	<script>
	{{range .Errors}} document.getElementById("error_list").innerHTML += '<li><span class="error_message">\n' +
				'	<div class="error_error">{{.Err}}</div>\n' +
				'	type <span class="error_type error_mark"></span> at file\n' +
				'	<span class="error_file error_mark">{{.File}}.hyperbricks</span> at \n' +
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
	errors := "\n"
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

func ServeContent(w http.ResponseWriter, r *http.Request) {
	hbConfig := getHyperBricksConfiguration()

	route := strings.Trim(r.URL.Path, "/")
	if resolvedRoute, ok := resolveRoute(route, hbConfig.Server.Routing); ok {
		route = resolvedRoute
	} else if route == "" {
		route = "index"
	}

	logging.GetLogger().Debugw("Received request for route", "route", route)

	var htmlContent strings.Builder
	if hbConfig.Mode == shared.LIVE_MODE {
		cacheEntry := handleLiveMode(w, route, r)
		htmlContent.WriteString(cacheEntry.Content)
		applyResponseHeaders(cacheEntry.Headers, w)
		applyResponseCookies(cacheEntry.Cookies, w)
		if cacheEntry.ContentType != "" {
			w.Header().Set("Content-Type", cacheEntry.ContentType)
		} else {
			w.Header().Set("Content-Type", "text/html")
		}
		status := cacheEntry.Status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
	} else {
		renderContent := handleDeveloperMode(w, route, r)
		htmlContent.WriteString(renderContent.Content)
		applyResponseHeaders(renderContent.Headers, w)
		applyResponseCookies(renderContent.Cookies, w)
		if renderContent.ContentType != "" {
			w.Header().Set("Content-Type", renderContent.ContentType)
		} else {
			w.Header().Set("Content-Type", "text/html")
		}
		status := renderContent.Status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)

	}

	if _, err := fmt.Fprint(w, htmlContent.String()); err != nil {
		logging.GetLogger().Errorw("Error writing response", "route", route, "error", err)
	} else {
		logging.GetLogger().Debugw("Served request", "route", route)
	}
}

// RENDER WITHOUT CACHE
func handleDeveloperMode(w http.ResponseWriter, route string, r *http.Request) RenderContent {
	logging.GetLogger().Debugw("Developer mode active. Rendering fresh content:", route)
	return renderContent(w, route, r)
}

// RENDER WITH CACHE
func handleLiveMode(w http.ResponseWriter, route string, r *http.Request) CacheEntry {

	hbConfig := getHyperBricksConfiguration()
	cacheDuration := hbConfig.Live.CacheTime

	htmlCacheMutex.RLock()
	cacheEntry, found := htmlCache[route]
	htmlCacheMutex.RUnlock()

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

	renderContent := renderContent(w, route, r)
	if !renderContent.NoCache {
		renderContent.Content += fmt.Sprintf("\n<!-- Rendered at: %s -->", renderTime)
		renderContent.Content += fmt.Sprintf("\n<!-- Cache expires at: %s -->", expirationTime)
		if renderContent.Content != "" {
			htmlCacheMutex.Lock()
			htmlCache[route] = CacheEntry{
				Content:     renderContent.Content,
				Timestamp:   now,
				ContentType: renderContent.ContentType,
				Status:      renderContent.Status,
				Headers:     renderContent.Headers,
				Cookies:     renderContent.Cookies,
			}
			htmlCacheMutex.Unlock()
			logging.GetLogger().Debugw("Updated cache for route", "route", route)
		}
	}
	return CacheEntry{
		Content:     renderContent.Content,
		Timestamp:   now,
		ContentType: renderContent.ContentType,
		Status:      renderContent.Status,
		Headers:     renderContent.Headers,
		Cookies:     renderContent.Cookies,
	}
}
