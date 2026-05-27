package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type routeSplitDemoFields struct {
	Action   string `mapstructure:"action"`
	Template string `mapstructure:"template"`
}

type routeSplitDemoConfig struct {
	shared.Component `mapstructure:",squash"`
	PluginName       string               `mapstructure:"plugin"`
	Fields           routeSplitDemoFields `mapstructure:"data"`
}

type routeSplitDemoPlugin struct{}

var _ shared.PluginRenderer = (*routeSplitDemoPlugin)(nil)

func (p *routeSplitDemoPlugin) Render(instance interface{}, ctx context.Context) (any, []error) {
	var errs []error

	var cfg routeSplitDemoConfig
	if err := shared.DecodeWithBasicHooks(instance, &cfg); err != nil {
		errs = append(errs, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Path:     cfg.HyperBricksPath,
			Key:      cfg.HyperBricksKey,
			Rejected: true,
			Err:      fmt.Sprintf("failed to decode route split demo config: %v", err),
		})
		return "<!-- failed to decode route split demo config -->", errs
	}

	if strings.TrimSpace(cfg.Fields.Template) == "" {
		errs = append(errs, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Path:     cfg.HyperBricksPath,
			Key:      cfg.HyperBricksKey,
			Rejected: true,
			Err:      "route split demo plugin requires data.template",
		})
		return "<!-- route split demo plugin requires data.template -->", errs
	}

	req, _ := ctx.Value(shared.Request).(*http.Request)
	if req == nil {
		errs = append(errs, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Path:     cfg.HyperBricksPath,
			Key:      cfg.HyperBricksKey,
			Rejected: true,
			Err:      "route split demo plugin requires request context",
		})
		return "<!-- route split demo plugin requires request context -->", errs
	}

	switch strings.TrimSpace(cfg.Fields.Action) {
	case "import":
		return handleImport(cfg, req), errs
	default:
		errs = append(errs, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Path:     cfg.HyperBricksPath,
			Key:      cfg.HyperBricksKey,
			Rejected: true,
			Err:      "unknown route split demo action",
		})
		return "<!-- unknown route split demo action -->", errs
	}
}

func handleImport(cfg routeSplitDemoConfig, req *http.Request) map[string]interface{} {
	if err := req.ParseForm(); err != nil {
		return renderStage(cfg, map[string]interface{}{
			"error_message": "Could not parse the import request.",
		})
	}

	manifestURL := strings.TrimSpace(req.FormValue("manifest_url"))
	importMode := strings.TrimSpace(req.FormValue("import_mode"))
	notes := strings.TrimSpace(req.FormValue("notes"))

	if manifestURL == "" {
		return renderStage(cfg, map[string]interface{}{
			"error_message": "Enter a manifest URL or module-local manifest path first.",
		})
	}

	if !strings.HasPrefix(manifestURL, "https://") && !strings.HasPrefix(manifestURL, "/") {
		return renderStage(cfg, map[string]interface{}{
			"error_message": "Manifest URL must start with https:// or / for this demo.",
		})
	}

	if importMode == "" {
		importMode = "preview"
	}

	normalizedTags := normalizeTags(req.FormValue("tags"))
	steps := []string{
		"Validate the manifest location and normalize the request shape.",
		"Inspect import metadata before touching project state.",
	}
	if importMode == "apply" {
		steps = append(steps,
			"Transform the manifest into project-local bricks and assets.",
			"Queue post-import cleanup and editor refresh hooks.",
		)
	} else {
		steps = append(steps,
			"Build a dry-run preview so the operator can inspect the import plan.",
			"Return a safe summary without mutating project state.",
		)
	}

	return renderStage(cfg, map[string]interface{}{
		"success_title":   "Plugin orchestration complete",
		"success_message": "The plugin validated the request and built a normalized import plan before rendering the result template.",
		"manifest_url":    manifestURL,
		"import_mode":     importMode,
		"notes":           notes,
		"tags":            toInterfaces(normalizedTags),
		"steps":           toInterfaces(steps),
	})
}

func normalizeTags(raw string) []string {
	var tags []string
	seen := map[string]bool{}

	for _, part := range strings.Split(raw, ",") {
		tag := strings.ToLower(strings.TrimSpace(part))
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		tags = append(tags, tag)
	}

	return tags
}

func toInterfaces(values []string) []interface{} {
	items := make([]interface{}, 0, len(values))
	for _, value := range values {
		items = append(items, value)
	}
	return items
}

func renderStage(cfg routeSplitDemoConfig, values map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"@type":    "<TEMPLATE>",
		"template": cfg.Fields.Template,
		"values":   values,
	}
}

func Plugin() (shared.PluginRenderer, error) {
	return &routeSplitDemoPlugin{}, nil
}
