package main

import (
	"context"
	"fmt"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

// The plugin field definition
type Fields struct {
	Message string `mapstructure:"message"`
}

// Basic config for ComponentRenderers
type MyPluginConfig struct {
	shared.Component `mapstructure:",squash"`
	PluginName       string `mapstructure:"plugin"`
	Fields           `mapstructure:"data"`
}

// MyPlugin implements the Renderer interface.
type MyPlugin struct{}

// Ensure MyPlugin implements shared.ComponentRenderer
var _ shared.PluginRenderer = (*MyPlugin)(nil)

// Render is the function that will be called by the renderer.
func (p *MyPlugin) Render(instance interface{}, ctx context.Context) (any, []error) {

	var errors []error

	var config MyPluginConfig
	err := shared.DecodeWithBasicHooks(instance, &config)
	if err != nil {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Path:     config.HyperBricksPath,
			Key:      config.HyperBricksKey,
			Rejected: true,
			Err:      fmt.Sprintf("Failed to decode plugin instance: %v", err),
		})
		return "<!--Failed to render MyPlugin -->", errors
	}

	return fmt.Sprintf("<div class=\"my_plugin-content\">%s</div>\n", config.Fields.Message), errors
}

// var Plugin shared.PluginRenderer = &MyPlugin{}
// This function is exposed for the main application.
func Plugin() (shared.PluginRenderer, error) {
	return &MyPlugin{}, nil
}
