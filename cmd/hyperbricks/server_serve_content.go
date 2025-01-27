package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hyperbricks/hyperbricks/internal/component"
	"github.com/hyperbricks/hyperbricks/internal/composite"
	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/yosssi/gohtml"
)

func renderStaticContent(route string) string {
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

	if configCopy["@type"].(string) == composite.HxApiConfigGetName() {
		// Extract hxdata
		hxdata := make(map[string]interface{})

		// adding to mapstructure
		configCopy["hx_form_data"] = hxdata
		configCopy["hx_response"] = make(map[string]interface{})

	}

	if configCopy["@type"].(string) == composite.FragmentConfigGetName() {
		configCopy["hx_response"] = make(map[string]interface{})
	}

	htmlContent, renderErrors := rm.Render(configCopy["@type"].(string), configCopy)

	htmlContent = HandleRenderErrors(renderErrors) + htmlContent

	if hbConfig.Server.Beautify {
		htmlContent = gohtml.Format(htmlContent)
	}

	cacheTime := time.Now().Format(time.RFC3339)
	htmlContent += fmt.Sprintf("\n<!-- Cached at: %s -->", cacheTime)
	return htmlContent
}

func renderContent(w http.ResponseWriter, r *http.Request, route string) string {
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

	if configCopy["@type"].(string) == composite.HxApiConfigGetName() {
		// Extract hxdata
		hxdata, extractError := shared.ExtractHxData(r)
		if extractError != nil {
			logging.GetLogger().Error("Failed to extract hxdata:", extractError)
		}

		// adding to mapstructure
		configCopy["hx_form_data"] = hxdata
		configCopy["hx_response"] = w

	}

	if configCopy["@type"].(string) == composite.FragmentConfigGetName() {
		configCopy["hx_response"] = w
	}

	htmlContent, renderErrors := rm.Render(configCopy["@type"].(string), configCopy)

	htmlContent = HandleRenderErrors(renderErrors) + htmlContent

	if hbConfig.Server.Beautify {
		htmlContent = gohtml.Format(htmlContent)
	}

	cacheTime := time.Now().Format(time.RFC3339)
	htmlContent += fmt.Sprintf("\n<!-- Cached at: %s -->", cacheTime)
	return htmlContent
}

func HandleRenderErrors(renderErrors []error) string {
	errors := ""
	for e := range renderErrors {

		componentError, ok := renderErrors[e].(shared.ComponentError)
		if ok {
			errors += "<!-- Error @" + fmt.Sprintf(`path:%s %v`, componentError.Path, componentError.Err) + " -->"

		} else {
			e := error(renderErrors[e])
			errors += "<!-- Error @" + fmt.Sprintf("%v", e) + " -->"
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

	htmlContent := ""
	if hbConfig.Mode == shared.LIVE_MODE {
		htmlContent = handleLiveMode(w, r, route)
	} else {
		htmlContent = handleDeveloperMode(w, r, route)
	}

	//w.Header().Set("HX-Trigger", "Deleted")
	w.Header().Set("Content-Type", "text/html")
	if _, err := fmt.Fprint(w, htmlContent); err != nil {
		logging.GetLogger().Errorw("Error writing response", "route", route, "error", err)
	} else {
		logging.GetLogger().Debugw("Served request", "route", route)
	}
}

// RENDER WITHOUT CACHE
func handleDeveloperMode(w http.ResponseWriter, r *http.Request, route string) string {
	logging.GetLogger().Debugw("Developer mode active. Rendering fresh content.", "route", route)
	return renderContent(w, r, route)
}

// RENDER WITH CACHE
func handleLiveMode(w http.ResponseWriter, r *http.Request, route string) string {

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
		logging.GetLogger().Infof("Cache expired for route. Re-rendering content. %v", "route", route)
	} else {
		logging.GetLogger().Debugw("Cache miss for route. Rendering content. %v", "route", route)
	}

	htmlContent := renderContent(w, r, route)
	if htmlContent != "" {
		htmlCacheMutex.Lock()
		htmlCache[route] = CacheEntry{
			Content:   htmlContent,
			Timestamp: time.Now(),
		}
		htmlCacheMutex.Unlock()
		logging.GetLogger().Debugw("Updated cache for route", "route", route)
	}

	return htmlContent
}
