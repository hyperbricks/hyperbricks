package main

import (
	"context"
	"fmt"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/russross/blackfriday/v2"
)

// HOW TO USE THIS PLUGIN:
// markdown = <PLUGIN>
// markdown.plugin = MarkdownPlugin
// markdown.data.content = # Welcome\n\nThis is **Markdown** content.

// Fields defines the configuration fields for the Markdown plugin.
type Fields struct {
	Content string `mapstructure:"content"`
}

// MarkdownConfig holds the complete configuration for the Markdown plugin.
type MarkdownConfig struct {
	shared.Component `mapstructure:",squash"`
	PluginName       string `mapstructure:"plugin"`
	Fields           `mapstructure:"data"`
}

// MarkdownPlugin implements the shared.PluginRenderer interface.
type MarkdownPlugin struct{}

// Ensure MarkdownPlugin implements shared.PluginRenderer.
var _ shared.PluginRenderer = (*MarkdownPlugin)(nil)

// Render converts the Markdown content to HTML.
func (p *MarkdownPlugin) Render(instance interface{}, ctx context.Context) (string, []error) {
	var errs []error

	var config MarkdownConfig
	err := shared.DecodeWithBasicHooks(instance, &config)
	if err != nil {
		errs = append(errs, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Path:     config.HyperBricksPath,
			Key:      config.HyperBricksKey,
			Rejected: true,
			Err:      fmt.Sprintf("Failed to decode plugin instance: %v", err),
		})
		return "<!-- Failed to render markdown_plugin -->", errs
	}

	// Convert the Markdown content to HTML.
	htmlBytes := blackfriday.Run([]byte(config.Fields.Content))
	htmlContent := string(htmlBytes)

	// Wrap the HTML content in a container div.
	return fmt.Sprintf("<div class=\"markdown_plugin-content\">\n%s\n</div>\n", htmlContent), errs
}

// Plugin is the exported function that returns an instance of MarkdownPlugin.
func Plugin() (shared.PluginRenderer, error) {
	return &MarkdownPlugin{}, nil
}
