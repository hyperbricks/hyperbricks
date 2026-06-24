package component

import (
	"context"
	"fmt"
	"strings"

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
	PluginName       string                 `mapstructure:"plugin"  description:"Name of the plugin to render"`
	Classes          []string               `mapstructure:"classes" description:"Optional CSS classes for the plugin output wrapper" example:"{!{plugin-classes.hyperbricks.yaml}}"`
	Data             map[string]interface{} `mapstructure:"data" description:"Plugin-specific data passed to the renderer"`
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
	_, err := r.RenderManager.RegisterAndLoadPluginByName(ctx, pluginDir, config.PluginName)
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

	pluginRenderer, pluginExists := r.RenderManager.GetPlugin(config.PluginName)
	if !pluginExists {
		builder.WriteString(fmt.Sprintf("<!-- Plugin %v was loaded but not registered -->\n", config.PluginName))
		errors = append(errors, shared.ComponentError{
			Hash: shared.GenerateHash(),
			Key:  config.Component.Meta.HyperBricksKey,
			Path: config.Component.Meta.HyperBricksPath,
			File: config.Component.Meta.HyperBricksFile,
			Type: PluginRenderGetName(),
			Err:  fmt.Sprintf("Plugin %v was loaded but not registered", config.PluginName),
		})
		return builder.String(), errors
	}

	return r.renderAndWrap(pluginRenderer, config, instance, ctx, errors)
}

func (r *PluginRenderer) renderAndWrap(pluginRenderer shared.PluginRenderer, config PluginConfig, instance interface{}, ctx context.Context, errs []error) (string, []error) {
	var builder strings.Builder
	var handledResponse *shared.HandledResponse

	if ctx == nil {
		ctx = context.Background()
	}
	renderedValue, renderErrs := pluginRenderer.Render(instance, ctx)
	if renderErrs != nil {
		errs = append(errs, renderErrs...)
	}

	renderedContent := ""
	switch value := renderedValue.(type) {
	case shared.HandledResponse:
		handledResponse = cloneHandledResponse(&value)
	case *shared.HandledResponse:
		handledResponse = cloneHandledResponse(value)
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

	if handledResponse != nil {
		if capture, _ := ctx.Value(shared.HandledResponseCaptureKey).(*shared.HandledResponseCapture); capture != nil {
			capture.Response = handledResponse
			return "", errs
		}
		errs = append(errs, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Key:      config.HyperBricksKey,
			Path:     config.HyperBricksPath,
			File:     config.HyperBricksFile,
			Type:     PluginRenderGetName(),
			Rejected: true,
			Err:      "plugin returned handled response without an active capture context",
		})
		return "<!-- plugin returned handled response without capture context -->", errs
	}

	builder.WriteString(wrapPluginHTML(config, renderedContent))
	return builder.String(), errs
}

func cloneHandledResponse(response *shared.HandledResponse) *shared.HandledResponse {
	if response == nil {
		return nil
	}

	cloned := &shared.HandledResponse{
		Status:      response.Status,
		ContentType: response.ContentType,
		NoCache:     response.NoCache,
	}
	if len(response.Body) > 0 {
		cloned.Body = append([]byte(nil), response.Body...)
	}
	if len(response.Cookies) > 0 {
		cloned.Cookies = append([]string(nil), response.Cookies...)
	}
	if len(response.Headers) > 0 {
		cloned.Headers = make(map[string]string, len(response.Headers))
		for key, value := range response.Headers {
			cloned.Headers[key] = value
		}
	}
	return cloned
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
