package component

import (
	"fmt"
	"log"
	"path/filepath"
	"plugin"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/renderer"
	"github.com/hyperbricks/hyperbricks/internal/shared"
)

// Define a specific implementation of a RenderComponent
type PluginRenderer struct {
	renderer.CompositeRenderer
}

// Ensure ComponentRenderer implements the interface ComponentRenderer, that implements the Render method
var _ shared.ComponentRenderer = (*PluginRenderer)(nil)

// Basic config for ComponentRenderers
type PluginConfig struct {
	shared.Component `mapstructure:",squash"`
	PluginName       string                 `mapstructure:"plugin"`
	Classes          []string               `mapstructure:"classes" description:"Optional CSS classes for the link" example:"{!{link-classes.hyperbricks}}"`
	Data             map[string]interface{} `mapstructure:"data"`
}

// LinkConfigGetName returns the HyperBricks type associated with the PluginConfig.
func PluginRenderGetName() string {
	return "<PLUGIN>"
}
func (r *PluginRenderer) Types() []string {
	return []string{
		PluginRenderGetName(),
	}
}
func (r *PluginRenderer) Render(instance interface{}) (string, []error) {
	var errors []error
	var builder strings.Builder

	config, ok := instance.(PluginConfig)
	if !ok {
		errors = append(errors, shared.ComponentError{
			Err: fmt.Errorf("invalid type for MenuRenderer").Error(),
		})
		return fmt.Errorf("<!-- invalid type for MenuRenderer -->").Error(), errors
	}

	pluginRenderer, pluginExists := r.RenderManager.Plugins[config.PluginName]
	if !pluginExists {
		errors = append(errors, shared.ComponentError{
			Err: fmt.Errorf("plugin is not preloaded or found").Error(),
		})

		// If not found it can be loaded
		renderedContent, renderErrs := r.LoadAndRender(instance)
		if renderErrs != nil {
			errors = append(errors, renderErrs...)
		}

		return renderedContent, errors

	}

	// Call the Render method
	renderedContent, renderErrs := pluginRenderer.Render(instance)
	if renderErrs != nil {
		//log.Fatalf("Failed to render 'Plugin' symbol: %v", errr)
		errors = append(errors, renderErrs...)
	}

	// Define allowed attributes for this component and render them into a string.
	allowedAttributes := []string{"id", "data-role", "data-action"}
	extraAttributes := shared.RenderAllowedAttributes(config.ExtraAttributes, allowedAttributes)

	classes := ""
	// Construct the div element.
	if len(config.Classes) > 0 {
		classes = strings.Join(config.Classes, " ")
		classes = fmt.Sprintf(` class="%s"`, classes)
	}

	html := fmt.Sprintf(
		`<div%s%s>%s</div>`,
		classes,
		extraAttributes,
		renderedContent,
	)

	// Apply wrapping if specified
	if config.Enclose != "" {
		html = shared.EncloseContent(config.Enclose, html)
	}

	builder.WriteString(html)
	return builder.String(), errors
}

// Implement the Render method PluginRenderer
func (r *PluginRenderer) LoadAndRender(instance interface{}) (string, []error) {

	var errors []error
	var builder strings.Builder

	config, ok := instance.(PluginConfig)
	if !ok {
		errors = append(errors, shared.ComponentError{
			Err: fmt.Errorf("invalid type for MenuRenderer").Error(),
		})
		return "", errors
	}

	// Load the plugin
	hbConfig := shared.GetHyperBricksConfiguration()
	pluginDir := "./bin/plugins"
	if tbplugindir, ok := hbConfig.Directories["plugins"]; ok {
		pluginDir = tbplugindir
	}

	pluginPath := filepath.Join(pluginDir, config.PluginName+".so")
	// log.Printf("pluginPath: %s", pluginPath)
	p, err := plugin.Open(pluginPath)
	if err != nil {
		builder.WriteString(fmt.Sprintf("<!-- Error loading plugin %v: %v -->\n", config.PluginName, err))
		errors = append(errors, shared.ComponentError{
			Err: fmt.Sprintf("Error loading plugin %v: %v\n", config.PluginName, err),
		})
		return builder.String(), errors
	}

	// Lookup "Plugin" as a function
	symbol, err := p.Lookup("Plugin")
	if err != nil {
		log.Fatalf("Failed to lookup 'Plugin' symbol: %v", err)
	}

	// Assert it is of the correct function type
	pluginFactory, ok := symbol.(func() (shared.PluginRenderer, error))
	if !ok {
		log.Fatalf("Plugin symbol is not of expected type 'func() (shared.Renderer, error)'")
	}

	// Create an instance of the plugin
	renderer, err := pluginFactory()
	if err != nil {
		log.Fatalf("Error initializing plugin: %v", err)
	}

	// Call the Render method
	renderedContent, renderErrs := renderer.Render(instance)
	if renderErrs != nil {
		//log.Fatalf("Failed to render 'Plugin' symbol: %v", errr)
		errors = append(errors, renderErrs...)
	}

	// Define allowed attributes for this component and render them into a string.
	allowedAttributes := []string{"id", "data-role", "data-action"}
	extraAttributes := shared.RenderAllowedAttributes(config.ExtraAttributes, allowedAttributes)

	classes := ""
	// Construct the div element.
	if len(config.Classes) > 0 {
		classes = strings.Join(config.Classes, " ")
		classes = fmt.Sprintf(` class="%s" `, classes)
	}

	html := fmt.Sprintf(
		`<div%s%s>%s</div>`,
		classes,
		extraAttributes,
		renderedContent,
	)

	// Apply wrapping if specified
	if config.Enclose != "" {
		html = shared.EncloseContent(config.Enclose, html)
	}

	builder.WriteString(html)
	return builder.String(), errors

}
