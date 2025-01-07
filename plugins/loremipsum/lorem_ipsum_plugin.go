package main

import (
	"fmt"

	lorem "github.com/drhodes/golorem"
	"github.com/hyperbricks/hyperbricks/internal/shared"
)

// The plugin field definition
type Fields struct {
	Paragraphs int `mapstructure:"paragraphs"`
}

// Basic config for ComponentRenderers
type LoremIpsumConfig struct {
	shared.Component `mapstructure:",squash"`
	PluginName       string `mapstructure:"plugin"`
	Fields           `mapstructure:"data"`
}

// MyPlugin implements the Renderer interface.
type LoremIpsumPlugin struct{}

// Ensure MyPlugin implements shared.ComponentRenderer
var _ shared.PluginRenderer = (*LoremIpsumPlugin)(nil)

// Render is the function that will be called by the renderer.
func (p *LoremIpsumPlugin) Render(instance interface{}) (string, []error) {

	var errors []error

	var config LoremIpsumConfig

	err := shared.DecodeWithBasicHooks(instance, &config)
	if err != nil {
		errors = append(errors, shared.ComponentError{
			Path:     config.Path,
			Key:      config.Key,
			Rejected: true,
			Err:      fmt.Sprintf("Failed to decode plugin instance: %v", err),
		})
		return "<!--Failed to render lorem_ipsum_v2_plugin  -->", errors
	}

	paragraphs := int(config.Fields.Paragraphs)

	return fmt.Sprintf("<div class=\"lorem_ipsum_v2_plugin-content\">%s</div>\n", lorem.Paragraph(paragraphs, paragraphs)), errors
}

// var Plugin shared.PluginRenderer = &MyPlugin{}
// This function is exposed for the main application.
func Plugin() (shared.PluginRenderer, error) {
	return &LoremIpsumPlugin{}, nil
}
