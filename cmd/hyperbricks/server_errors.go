package main

import (
	"html/template"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/hyperbricks/hyperbricks/assets"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

const errorsViewPath = "/__hyperbricks/errors"

var errorsTemplate = template.Must(template.New("errors").Parse(assets.ErrorsPage))

func errorsViewEnabled() bool {
	return developerDashboardEnabled()
}

func handleErrorsView(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != errorsViewPath && !strings.HasPrefix(r.URL.Path, errorsViewPath+"/") {
		return false
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	if !errorsViewEnabled() {
		http.NotFound(w, r)
		return true
	}
	if !requireDeveloperInterfaceAuth(w, r) {
		return true
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return true
	}
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
	var content []byte
	switch strings.TrimPrefix(r.URL.Path, errorsViewPath) {
	case "", "/":
		cfg := getHyperBricksConfiguration()
		data := SysData{Module: filepath.Base(shared.GetRuntimeOptions().ModuleRoot), Mode: cfg.Mode}
		if cfg.Mode == shared.DEVELOPMENT_MODE && cfg.Development.FrontendEditing.Enabled &&
			cfg.Development.FrontendEditing.Spaces.Enabled && cfg.ValidateFrontendEditing() == nil {
			data.SpacesRoute = cfg.Development.FrontendEditing.Spaces.Route
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.Method != http.MethodHead {
			if err := errorsTemplate.Execute(w, data); err != nil {
				logging.GetLogger().Errorw("Errors view could not be rendered", "error", err)
			}
		}
		return true
	case "/web/errors.css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		content = assets.ErrorsCSS
	case "/web/errors.js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		content = assets.ErrorsScript
	case "/web/errors-model.mjs":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		content = assets.ErrorsModel
	default:
		http.NotFound(w, r)
		return true
	}
	if r.Method != http.MethodHead {
		_, _ = w.Write(content)
	}
	return true
}
