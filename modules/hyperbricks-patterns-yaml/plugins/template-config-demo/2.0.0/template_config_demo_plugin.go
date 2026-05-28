package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/russross/blackfriday/v2"
)

type Fields struct {
	Content  string `mapstructure:"content"`
	Class    string `mapstructure:"class"`
	Template string `mapstructure:"template"` // template.file resolves to this string
}

type TemplateConfigDemoConfig struct {
	shared.Component `mapstructure:",squash"`
	PluginName       string `mapstructure:"plugin"`
	Fields           `mapstructure:"data"`
}

type TemplateConfigDemoPlugin struct{}

var _ shared.PluginRenderer = (*TemplateConfigDemoPlugin)(nil)

func normalizeContent(value string) string {
	value = strings.TrimSpace(value)

	if unquoted, err := strconv.Unquote(value); err == nil {
		return unquoted
	}

	value = strings.ReplaceAll(value, `\n`, "\n")
	value = strings.Trim(value, `"`)

	return value
}

func (p *TemplateConfigDemoPlugin) Render(instance interface{}, ctx context.Context) (any, []error) {
	var errs []error
	var config TemplateConfigDemoConfig
	err := shared.DecodeWithBasicHooks(instance, &config)
	if err != nil {
		errs = append(errs, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Path:     config.HyperBricksPath,
			Key:      config.HyperBricksKey,
			Rejected: true,
			Err:      fmt.Sprintf("failed to decode plugin instance: %v", err),
		})
		return "<!-- Failed to render template_config_demo_plugin -->", errs
	}

	if config.Fields.Class == "" {
		config.Fields.Class = "template_config_demo-content"
	}

	// 1. Clean the input Markdown (strips extra quotes/literal \n from the raw string)
	cleanMarkdown := normalizeContent(config.Fields.Content)

	// 2. Render to HTML bytes
	htmlBytes := blackfriday.Run([]byte(cleanMarkdown))

	// 3. Convert to plain string so it survives the <TREE> serialization
	htmlContent := string(htmlBytes)

	values := map[string]interface{}{
		"class": config.Fields.Class,
		"html":  htmlContent,
	}

	if strings.TrimSpace(config.Fields.Template) == "" {
		errs = append(errs, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Path:     config.HyperBricksPath,
			Key:      config.HyperBricksKey,
			Rejected: true,
			Err:      "template_config_demo_plugin requires data.template",
		})
		return "<!-- template_config_demo_plugin requires data.template -->", errs
	}

	// Return a plain <TEMPLATE> config for the normal demo flow.
	// If you later need more complex HyperBricks composition, this same template
	// node can be wrapped in a synthetic <TREE> and returned as a larger shape.
	return map[string]interface{}{
		"@type":    "<TEMPLATE>",
		"template": config.Fields.Template,
		"values":   values,
	}, errs

	// Example optional wrapper for more complex compositions:
	//
	// return map[string]interface{}{
	// 	"@type": "<TREE>",
	// 	"10": map[string]interface{}{
	// 		"@type":    "<TEMPLATE>",
	// 		"template": config.Fields.Template,
	// 		"values":   values,
	// 	},
	// }, errs
}

func Plugin() (shared.PluginRenderer, error) {
	return &TemplateConfigDemoPlugin{}, nil
}
