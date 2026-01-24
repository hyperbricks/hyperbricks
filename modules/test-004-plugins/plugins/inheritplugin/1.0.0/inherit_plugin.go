package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

// The plugin field definition
type Fields struct {
	Template map[string]interface{} `mapstructure:"template"`
	Title    string                 `mapstructure:"title"`
	Content  string                 `mapstructure:"content"`
	Image    string                 `mapstructure:"image"`
}

// Basic config for ComponentRenderers
type InheritPluginConfig struct {
	shared.Component `mapstructure:",squash"`
	PluginName       string `mapstructure:"plugin"`
	Fields           `mapstructure:"data"`
}

// InheritPlugin implements the Renderer interface.
type InheritPlugin struct{}

// Ensure InheritPlugin implements shared.ComponentRenderer
var _ shared.PluginRenderer = (*InheritPlugin)(nil)

// Render is the function that will be called by the renderer.
func (p *InheritPlugin) Render(instance interface{}, ctx context.Context) (any, []error) {

	var errors []error

	var config InheritPluginConfig
	err := shared.DecodeWithBasicHooks(instance, &config)
	if err != nil {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Path:     config.HyperBricksPath,
			Key:      config.HyperBricksKey,
			Rejected: true,
			Err:      fmt.Sprintf("Failed to decode plugin instance: %v", err),
		})
		return "<!--Failed to render InheritMapPlugin -->", errors
	}

	if config.Fields.Template == nil {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Path:     config.HyperBricksPath,
			Key:      config.HyperBricksKey,
			Rejected: true,
			Err:      "missing data.template map for InheritMapPlugin",
		})
		return "<!-- InheritMapPlugin missing data.template -->", errors
	}

	templ := cloneMapDeep(config.Fields.Template)
	if _, ok := templ["@type"]; !ok {
		templ["@type"] = "<TREE>"
	}
	applyBindings(templ, config.Fields)

	return templ, errors
}

// var Plugin shared.PluginRenderer = &InheritPlugin{}
// This function is exposed for the main application.
func Plugin() (shared.PluginRenderer, error) {
	return &InheritPlugin{}, nil
}

func cloneMapDeep(source map[string]interface{}) map[string]interface{} {
	dest := make(map[string]interface{}, len(source))
	for k, v := range source {
		dest[k] = cloneValueDeep(v)
	}
	return dest
}

func cloneValueDeep(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return cloneMapDeep(typed)
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i, elem := range typed {
			out[i] = cloneValueDeep(elem)
		}
		return out
	default:
		return value
	}
}

func applyBindings(node map[string]interface{}, fields Fields) {
	if bindRaw, ok := node["@bind"]; ok {
		if bind, ok := bindRaw.(string); ok {
			switch strings.ToLower(strings.TrimSpace(bind)) {
			case "title":
				if fields.Title != "" {
					node["value"] = fields.Title
				}
			case "content":
				if fields.Content != "" {
					data := ensureMap(node, "data")
					data["content"] = fields.Content
				}
			case "image":
				if fields.Image != "" {
					node["src"] = fields.Image
				}
			}
		}
	}

	for _, value := range node {
		switch typed := value.(type) {
		case map[string]interface{}:
			applyBindings(typed, fields)
		case []interface{}:
			for _, elem := range typed {
				if nested, ok := elem.(map[string]interface{}); ok {
					applyBindings(nested, fields)
				}
			}
		}
	}
}

func ensureMap(node map[string]interface{}, key string) map[string]interface{} {
	if existing, ok := node[key].(map[string]interface{}); ok {
		return existing
	}
	created := make(map[string]interface{})
	node[key] = created
	return created
}
