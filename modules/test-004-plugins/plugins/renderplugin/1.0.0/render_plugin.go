package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
)

// The plugin field definition
type Fields struct {
	Mode    string `mapstructure:"mode"`
	Message string `mapstructure:"message"`
	Enclose string `mapstructure:"enclose"`
}

// Basic config for ComponentRenderers
type RenderPluginConfig struct {
	shared.Component `mapstructure:",squash"`
	PluginName       string `mapstructure:"plugin"`
	Fields           `mapstructure:"data"`
}

// RenderPlugin implements the Renderer interface.
type RenderPlugin struct{}

// Ensure RenderPlugin implements shared.ComponentRenderer
var _ shared.PluginRenderer = (*RenderPlugin)(nil)

// Render is the function that will be called by the renderer.
func (p *RenderPlugin) Render(instance interface{}, ctx context.Context) (any, []error) {

	var errors []error

	var config RenderPluginConfig
	err := shared.DecodeWithBasicHooks(instance, &config)
	if err != nil {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Path:     config.HyperBricksPath,
			Key:      config.HyperBricksKey,
			Rejected: true,
			Err:      fmt.Sprintf("Failed to decode plugin instance: %v", err),
		})
		return "<!--Failed to render RenderMapPlugin -->", errors
	}

	mode := strings.ToLower(strings.TrimSpace(config.Fields.Mode))
	switch mode {
	case "map":
		return map[string]interface{}{
			"@type":   "<TEXT>",
			"value":   config.Fields.Message,
			"enclose": config.Fields.Enclose,
		}, errors
	case "request":
		return typefactory.TypeRequest{
			TypeName: "<TEXT>",
			Data: map[string]interface{}{
				"@type":   "<TEXT>",
				"value":   config.Fields.Message,
				"enclose": config.Fields.Enclose,
			},
		}, errors
	default:
		return fmt.Sprintf("<div class=\"render-plugin-html\">%s</div>\n", config.Fields.Message), errors
	}
}

// var Plugin shared.PluginRenderer = &RenderPlugin{}
// This function is exposed for the main application.
func Plugin() (shared.PluginRenderer, error) {
	return &RenderPlugin{}, nil
}
