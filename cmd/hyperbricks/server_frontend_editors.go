package main

import (
	"context"
	"net/http"
	"sort"
	"strings"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/spaces"
)

var spacesEditor spaces.Handler

func spacesEditorAvailable() bool {
	return !commands.RenderStatic && shared.SpacesAvailable(getHyperBricksConfiguration(), shared.GetRuntimeOptions())
}

func validEditorRoute(route string) bool {
	return shared.ValidFrontendEditorRoute(route)
}

func handleFrontendEditor(w http.ResponseWriter, r *http.Request) bool {
	cfg := getHyperBricksConfiguration()
	editing := cfg.Development.FrontendEditing
	if commands.RenderStatic || shared.GetRuntimeOptions().Production || !editing.Enabled || cfg.ValidateFrontendEditing() != nil {
		return false
	}
	if r.URL.Path == editing.Spaces.Route || strings.HasPrefix(r.URL.Path, editing.Spaces.Route+"/") {
		if !spacesEditorAvailable() {
			return false
		}
		if !shared.RequireSpacesAuth(w, r, cfg.Development.Dashboard.Credentials) {
			return true
		}
		spacesEditor.ServeHTTP(w, r)
		return true
	}
	if cfg.Mode != shared.DEVELOPMENT_MODE {
		return false
	}
	keys := make([]string, 0, len(editing.Editors))
	for name := range editing.Editors {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		editor := editing.Editors[name]
		if !validEditorRoute(editor.Route) || (r.URL.Path != editor.Route && !strings.HasPrefix(r.URL.Path, editor.Route+"/")) {
			continue
		}
		if !requireDeveloperInterfaceAuth(w, r) {
			return true
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		enabled := false
		for _, plugin := range cfg.Plugins.Enabled {
			if plugin == editor.Plugin {
				enabled = true
			}
		}
		if !enabled || rm == nil {
			http.Error(w, "Frontend editor plugin is not enabled", http.StatusServiceUnavailable)
			return true
		}
		plugin, ok := rm.GetPlugin(editor.Plugin)
		if !ok {
			http.Error(w, "Frontend editor plugin is not loaded", http.StatusServiceUnavailable)
			return true
		}
		data := shared.CloneMapDeep(editor.Data)
		if data == nil {
			data = make(map[string]interface{})
		}
		data["route"] = editor.Route
		ctx := context.WithValue(r.Context(), shared.Request, r)
		value, errs := plugin.Render(component.PluginConfig{PluginName: editor.Plugin, Data: data}, ctx)
		if len(errs) > 0 {
			logging.GetLogger().Errorw("Frontend editor failed", "editor", name, "errors", errs)
			http.Error(w, "Frontend editor failed; see server diagnostics", http.StatusInternalServerError)
			return true
		}
		var response *shared.HandledResponse
		switch v := value.(type) {
		case shared.HandledResponse:
			response = &v
		case *shared.HandledResponse:
			response = v
		}
		if response == nil {
			http.Error(w, "Frontend editor must return a handled response", http.StatusInternalServerError)
			return true
		}
		content := renderHandledContent(nextRenderRequestID(), 200, "text/html; charset=utf-8", nil, nil, true, 0, response)
		if response.Stream != nil {
			if err := writeStreamResponse(w, r, content); err != nil {
				logging.GetLogger().Errorw("Frontend editor stream failed", "error", err)
			}
			return true
		}
		if r.Method == http.MethodHead {
			copy := *response
			copy.Body = nil
			response = &copy
		}
		writeRenderResponse(w, r.URL.Path, content.RequestID, "", "", response, content.Headers, content.Cookies, content.ContentType, content.Status, 0)
		return true
	}
	return false
}
