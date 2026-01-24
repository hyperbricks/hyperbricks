package component

import (
	"context"
	"fmt"
	"path/filepath"
	"plugin"
	"strings"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/renderer"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
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

	config, ok := instance.(PluginConfig)
	if !ok {
		errors = append(errors, shared.ComponentError{
			Hash: shared.GenerateHash(),
			Key:  config.HyperBricksKey,
			Path: config.HyperBricksPath,
			File: config.HyperBricksFile,
			Type: PluginRenderGetName(),
			Err:  fmt.Errorf("invalid type for PluginRenderer").Error(),
		})
		return fmt.Errorf("<!-- invalid type for PluginRenderer -->").Error(), errors
	}

	pluginRenderer, pluginExists := r.RenderManager.GetPlugin(config.PluginName)
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

	return r.renderAndWrap(pluginRenderer, config, instance, ctx, errors)
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
		builder.WriteString(fmt.Sprintf("<!-- Failed to lookup plugin %v: %v -->\n", config.PluginName, err))
		errors = append(errors, shared.ComponentError{
			Hash: shared.GenerateHash(),
			Key:  config.Component.Meta.HyperBricksKey,
			Path: config.Component.Meta.HyperBricksPath,
			File: config.Component.Meta.HyperBricksFile,
			Type: PluginRenderGetName(),
			Err:  fmt.Sprintf("Failed to lookup plugin %v: %v\n", config.PluginName, err),
		})
		return builder.String(), errors
	}

	pluginFactory, ok := symbol.(func() (shared.PluginRenderer, error))
	if !ok {
		builder.WriteString(fmt.Sprintf(
			"<!-- Plugin symbol is not of expected type 'func() (shared.Renderer, error)' %v -->\n",
			config.PluginName,
		))
		errors = append(errors, shared.ComponentError{
			Hash: shared.GenerateHash(),
			Key:  config.Component.Meta.HyperBricksKey,
			Path: config.Component.Meta.HyperBricksPath,
			File: config.Component.Meta.HyperBricksFile,
			Type: PluginRenderGetName(),
			Err:  fmt.Sprintf("Plugin symbol is not of expected type 'func() (shared.Renderer, error)' %v", config.PluginName),
		})
		return builder.String(), errors
	}

	renderer, err := pluginFactory()
	if err != nil {
		builder.WriteString(fmt.Sprintf("<!--Error initializing plugin: %v: %v -->\n", config.PluginName, err))
		errors = append(errors, shared.ComponentError{
			Hash: shared.GenerateHash(),
			Key:  config.Component.Meta.HyperBricksKey,
			Path: config.Component.Meta.HyperBricksPath,
			File: config.Component.Meta.HyperBricksFile,
			Type: PluginRenderGetName(),
			Err:  fmt.Sprintf("Error initializing plugin: %v: %v\n", config.PluginName, err),
		})
		return builder.String(), errors
	}

	// Store hot-loaded plugin so it's available next time
	r.RenderManager.SetPlugin(config.PluginName, renderer)

	return r.renderAndWrap(renderer, config, instance, ctx, errors)
}

func (r *PluginRenderer) renderAndWrap(pluginRenderer shared.PluginRenderer, config PluginConfig, instance interface{}, ctx context.Context, errs []error) (string, []error) {
	var builder strings.Builder

	if ctx == nil && commands.RenderStatic {
		ctx = context.Background()
	}
	renderedValue, renderErrs := pluginRenderer.Render(instance, ctx)
	if renderErrs != nil {
		errs = append(errs, renderErrs...)
	}

	renderedContent := ""
	switch value := renderedValue.(type) {
	case string:
		renderedContent = value
	case map[string]interface{}:
		componentType, ok := value["@type"].(string)
		if !ok || componentType == "" {
			errs = append(errs, shared.ComponentError{
				Hash:     shared.GenerateHash(),
				Key:      config.HyperBricksKey,
				Path:     config.HyperBricksPath,
				File:     config.HyperBricksFile,
				Type:     PluginRenderGetName(),
				Rejected: true,
				Err:      "plugin returned map without @type",
			})
			renderedContent = "<!-- plugin returned map without @type -->"
			break
		}

		if r.RenderManager == nil {
			errs = append(errs, shared.ComponentError{
				Hash:     shared.GenerateHash(),
				Key:      config.HyperBricksKey,
				Path:     config.HyperBricksPath,
				File:     config.HyperBricksFile,
				Type:     PluginRenderGetName(),
				Rejected: true,
				Err:      "plugin render manager is nil",
			})
			renderedContent = "<!-- plugin render manager is nil -->"
			break
		}

		renderedHTML, nestedErrs := r.RenderManager.Render(componentType, value, ctx)
		if nestedErrs != nil {
			errs = append(errs, nestedErrs...)
		}
		renderedContent = renderedHTML
	case typefactory.TypeRequest:
		if r.RenderManager == nil {
			errs = append(errs, shared.ComponentError{
				Hash:     shared.GenerateHash(),
				Key:      config.HyperBricksKey,
				Path:     config.HyperBricksPath,
				File:     config.HyperBricksFile,
				Type:     PluginRenderGetName(),
				Rejected: true,
				Err:      "plugin render manager is nil",
			})
			renderedContent = "<!-- plugin render manager is nil -->"
			break
		}

		renderedHTML, nestedErrs := r.RenderManager.Render(value.TypeName, value.Data, ctx)
		if nestedErrs != nil {
			errs = append(errs, nestedErrs...)
		}
		renderedContent = renderedHTML
	case *typefactory.TypeRequest:
		if value == nil {
			renderedContent = ""
			break
		}

		if r.RenderManager == nil {
			errs = append(errs, shared.ComponentError{
				Hash:     shared.GenerateHash(),
				Key:      config.HyperBricksKey,
				Path:     config.HyperBricksPath,
				File:     config.HyperBricksFile,
				Type:     PluginRenderGetName(),
				Rejected: true,
				Err:      "plugin render manager is nil",
			})
			renderedContent = "<!-- plugin render manager is nil -->"
			break
		}

		renderedHTML, nestedErrs := r.RenderManager.Render(value.TypeName, value.Data, ctx)
		if nestedErrs != nil {
			errs = append(errs, nestedErrs...)
		}
		renderedContent = renderedHTML
	case nil:
		renderedContent = ""
	default:
		errs = append(errs, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Key:      config.HyperBricksKey,
			Path:     config.HyperBricksPath,
			File:     config.HyperBricksFile,
			Type:     PluginRenderGetName(),
			Rejected: true,
			Err:      fmt.Sprintf("plugin returned unsupported type: %T", renderedValue),
		})
		renderedContent = fmt.Sprintf("<!-- plugin returned unsupported type: %T -->", renderedValue)
	}

	builder.WriteString(wrapPluginHTML(config, renderedContent))
	return builder.String(), errs
}

func wrapPluginHTML(config PluginConfig, renderedContent string) string {
	allowedAttributes := []string{"id", "data-role", "data-action"}
	extraAttributes := shared.RenderAllowedAttributes(config.ExtraAttributes, allowedAttributes)

	var html string
	if len(config.Classes) > 0 || extraAttributes != "" {
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
	return html
}
