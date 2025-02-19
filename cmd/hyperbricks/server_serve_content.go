package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/hyperbricks/hyperbricks/internal/component"
	"github.com/hyperbricks/hyperbricks/internal/composite"
	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/yosssi/gohtml"
)

func renderStaticContent(route string, ctx context.Context) string {
	hbConfig := getHyperBricksConfiguration()

	_config, found := getConfig(route)
	if !found {
		logging.GetLogger().Info("Config not found for route", "route", route)
		return fmt.Sprintf("Expected Hyperbricks '%s' was not found.", route)
	}

	configCopy := make(map[string]interface{})
	for key, value := range _config {
		configCopy[key] = value
	}

	if configCopy["@type"].(string) == composite.FragmentConfigGetName() {
		configCopy["hx_response"] = nil
	}

	htmlContent, renderErrors := rm.Render(configCopy["@type"].(string), configCopy, ctx)

	htmlContent = FrontEndErrorRender(renderErrors) + htmlContent

	if hbConfig.Server.Beautify {
		htmlContent = gohtml.Format(htmlContent)
	}

	return htmlContent
}

func renderContent(w http.ResponseWriter, route string, r *http.Request) (string, bool) {
	hbConfig := getHyperBricksConfiguration()
	nocache := false

	_config, found := getConfig(route)

	if !found {
		__config, _found := getConfig("404")
		if _found {
			logging.GetLogger().Info("Redirecting to 404", " from ", route)
			_config = __config
		} else {
			if route == "favicon.ico" {
				return "", false
			}
			logging.GetLogger().Info("Config not found for route: ", route)
			return fmt.Sprintf("Expected Hyperbricks '%s' was not found.", route), false
		}
	}

	if _, ok := _config["nocache"].(string); ok {
		nocache = true
		// logging.GetLogger().Debugf("NoCache = true: %s from %s", val, route)
	}

	configCopy := make(map[string]interface{})
	for key, value := range _config {
		configCopy[key] = value
	}

	if configCopy["@type"].(string) == composite.FragmentConfigGetName() {
		configCopy["hx_response"] = w
	}

	// Extract JWT token from the Authorization header for authentication if needed
	authHeader := r.Header.Get("Authorization")
	var jwtToken string
	if strings.HasPrefix(authHeader, "Bearer ") {
		jwtToken = strings.TrimPrefix(authHeader, "Bearer ")
	}

	// Store JWT token in request context
	ctx := context.WithValue(r.Context(), shared.JwtKey, jwtToken)

	var htmlContent strings.Builder

	renderOutput, renderErrors := rm.Render(configCopy["@type"].(string), configCopy, ctx)

	htmlContent.WriteString(renderOutput)
	var output strings.Builder

	if hbConfig.Server.Beautify {
		output.WriteString(gohtml.Format(htmlContent.String()))
	}

	// only render errors in debug or development mode...
	if hbConfig.Mode != shared.LIVE_MODE {
		if hbConfig.Development.FrontendErrors {
			output.WriteString(FrontEndErrorRender(renderErrors))
		} else {
			output.WriteString(HandleRenderErrors(renderErrors))
		}
	}
	return output.String(), nocache

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
				'	<span class="error_path error_mark">[{{.Hash}}] {{.Path}}.{{.Key}}</span> \n' +
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

	route := strings.TrimPrefix(r.URL.Path, "/")
	if route == "" {
		route = "index"
	}

	rm.GetRenderComponent("<MENU>").(*component.MenuRenderer).CurrentRoute = route
	logging.GetLogger().Debugw("Received request for route", "route", route)

	var htmlContent strings.Builder
	if hbConfig.Mode == shared.LIVE_MODE {
		content := handleLiveMode(w, route, r)
		htmlContent.WriteString(content)
	} else {
		content := handleDeveloperMode(w, route, r)
		htmlContent.WriteString(content)
	}

	//w.Header().Set("HX-Trigger", "Deleted")
	w.Header().Set("Content-Type", "text/html")
	if _, err := fmt.Fprint(w, htmlContent.String()); err != nil {
		logging.GetLogger().Errorw("Error writing response", "route", route, "error", err)
	} else {
		logging.GetLogger().Debugw("Served request", "route", route)
	}
}

// RENDER WITHOUT CACHE
func handleDeveloperMode(w http.ResponseWriter, route string, r *http.Request) string {
	logging.GetLogger().Debugw("Developer mode active. Rendering fresh content:", route)
	htmlContent, _ := renderContent(w, route, r)
	return htmlContent
}

// RENDER WITH CACHE
func handleLiveMode(w http.ResponseWriter, route string, r *http.Request) string {

	hbConfig := getHyperBricksConfiguration()
	cacheDuration := hbConfig.Live.CacheTime

	htmlCacheMutex.RLock()
	cacheEntry, found := htmlCache[route]
	htmlCacheMutex.RUnlock()

	if found && time.Since(cacheEntry.Timestamp) <= cacheDuration.Duration {
		logging.GetLogger().Debugw("Cache hit for route", "route", route)
		return cacheEntry.Content
	}

	if found {
		logging.GetLogger().Infof("Cache expired for route %s. Re-rendering content.", route)
	} else {
		logging.GetLogger().Debugf("Cache missing for route %s. Rendering content.", route)
	}

	htmlContent, nocache := renderContent(w, route, r)
	if !nocache {
		//Calculate expiration time
		var now = time.Now()
		expirationTime := now.Add(cacheDuration.Duration).Format("2006-01-02 15:04:05 (-07:00)")
		renderTime := time.Now().Format("2006-01-02 15:04:05 (-07:00)")

		htmlContent += fmt.Sprintf("\n<!-- Rendered at: %s -->", renderTime)
		htmlContent += fmt.Sprintf("\n<!-- Cache expires at: %s -->", expirationTime)
		if htmlContent != "" {
			htmlCacheMutex.Lock()
			htmlCache[route] = CacheEntry{
				Content:   htmlContent,
				Timestamp: now,
			}
			htmlCacheMutex.Unlock()
			logging.GetLogger().Debugw("Updated cache for route", "route", route)
		}
	}
	return htmlContent
}
