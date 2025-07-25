package component

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"plugin"
	"strings"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/renderer"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type PluginRenderer struct {
	renderer.CompositeRenderer
}

var _ shared.ComponentRenderer = (*PluginRenderer)(nil)

type PluginConfig struct {
	shared.Component `mapstructure:",squash"`
	PluginName       string                 `mapstructure:"plugin"  description:"Name of the plugin for lookup"`
	Classes          []string               `mapstructure:"classes" description:"Optional CSS classes for the link" example:"{!{plugin-classes.hyperbricks}}"`
	Data             map[string]interface{} `mapstructure:"data"`
}

func PluginRenderGetName() string {
	return "<PLUGIN>"
}
func (r *PluginRenderer) Types() []string {
	return []string{
		PluginRenderGetName(),
	}
}
func (r *PluginRenderer) Render(instance interface{}, ctx context.Context) (string, []error) {
	var errors []error
	var builder strings.Builder

	config, ok := instance.(PluginConfig)
	if !ok {
		errors = append(errors, shared.ComponentError{
			Hash: shared.GenerateHash(),
			Key:  config.HyperBricksKey,
			Path: config.HyperBricksPath,
			File: config.HyperBricksFile,
			Type: PluginRenderGetName(),
			Err:  fmt.Errorf("invalid type for MenuRenderer").Error(),
		})
		return fmt.Errorf("<!-- invalid type for MenuRenderer -->").Error(), errors
	}

	pluginRenderer, pluginExists := r.RenderManager.Plugins[config.PluginName]
	if !pluginExists {
		errors = append(errors, shared.ComponentError{
			Hash: shared.GenerateHash(),
			Key:  config.HyperBricksKey,
			Path: config.HyperBricksPath,
			File: config.HyperBricksFile,
			Type: PluginRenderGetName(),
			Err:  "plugin " + config.PluginName + " is not preloaded, make sure it is preloaded in production.",
		})

		renderedContent, renderErrs := r.LoadAndRender(instance, ctx)
		if renderErrs != nil {
			errors = append(errors, renderErrs...)
		}

		return renderedContent, errors

	}
	if ctx == nil && commands.RenderStatic {
		ctx = context.Background()
	}
	renderedContent, renderErrs := pluginRenderer.Render(instance, ctx)
	if renderErrs != nil {
		errors = append(errors, renderErrs...)
	}

	allowedAttributes := []string{"id", "data-role", "data-action"}
	extraAttributes := shared.RenderAllowedAttributes(config.ExtraAttributes, allowedAttributes)

	var html string

	if len(config.Classes) > 0 || extraAttributes != "" {
		// Build class attribute if needed
		classAttr := ""
		if len(config.Classes) > 0 {
			classAttr = fmt.Sprintf(` class="%s"`, strings.Join(config.Classes, " "))
		}
		html = fmt.Sprintf(`<div%s%s>%s</div>`, classAttr, extraAttributes, renderedContent)
	} else {
		html = renderedContent
	}

	if config.Enclose != "" {
		html = shared.EncloseContent(config.Enclose, html)
	}

	builder.WriteString(html)
	return builder.String(), errors
}

func (r *PluginRenderer) LoadAndRender(instance interface{}, ctx context.Context) (string, []error) {

	var errors []error
	var builder strings.Builder

	config, ok := instance.(PluginConfig)
	if !ok {
		errors = append(errors, shared.ComponentError{
			Hash: shared.GenerateHash(),
			Key:  config.Component.Meta.HyperBricksKey,
			Path: config.Component.Meta.HyperBricksPath,
			File: config.Component.Meta.HyperBricksFile,
			Type: PluginRenderGetName(),
			Err:  fmt.Errorf("invalid type").Error(),
		})
		return "", errors
	}

	hbConfig := shared.GetHyperBricksConfiguration()
	pluginDir := "./bin/plugins"
	if tbplugindir, ok := hbConfig.Directories["plugins"]; ok {
		pluginDir = tbplugindir
	}

	pluginPath := filepath.Join(pluginDir, config.PluginName+".so")

	p, err := plugin.Open(pluginPath)
	if err != nil {
		builder.WriteString(fmt.Sprintf("<!-- Error loading plugin %v: %v -->\n", config.PluginName, err))
		errors = append(errors, shared.ComponentError{
			Hash: shared.GenerateHash(),
			Key:  config.Component.Meta.HyperBricksKey,
			Path: config.Component.Meta.HyperBricksPath,
			File: config.Component.Meta.HyperBricksFile,
			Type: PluginRenderGetName(),
			Err:  fmt.Sprintf("Error loading plugin %v: %v\n", config.PluginName, err),
		})
		return builder.String(), errors
	}

	symbol, err := p.Lookup("Plugin")
	if err != nil {
		log.Fatalf("Failed to lookup 'Plugin' symbol: %v", err)
	}

	pluginFactory, ok := symbol.(func() (shared.PluginRenderer, error))
	if !ok {
		log.Fatalf("Plugin symbol is not of expected type 'func() (shared.Renderer, error)'")
	}

	renderer, err := pluginFactory()
	if err != nil {
		log.Fatalf("Error initializing plugin: %v", err)
	}
	if ctx == nil && commands.RenderStatic {
		ctx = context.Background()
	}
	renderedContent, renderErrs := renderer.Render(instance, ctx)
	if renderErrs != nil {

		errors = append(errors, renderErrs...)
	}

	allowedAttributes := []string{"id", "data-role", "data-action"}
	extraAttributes := shared.RenderAllowedAttributes(config.ExtraAttributes, allowedAttributes)

	var html string

	if len(config.Classes) > 0 || extraAttributes != "" {
		// Build class attribute if needed
		classAttr := ""
		if len(config.Classes) > 0 {
			classAttr = fmt.Sprintf(` class="%s"`, strings.Join(config.Classes, " "))
		}
		html = fmt.Sprintf(`<div%s%s>%s</div>`, classAttr, extraAttributes, renderedContent)
	} else {
		html = renderedContent
	}

	if config.Enclose != "" {
		html = shared.EncloseContent(config.Enclose, html)
	}

	builder.WriteString(html)
	return builder.String(), errors

}
