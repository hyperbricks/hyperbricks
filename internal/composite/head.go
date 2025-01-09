package composite

import (
	"fmt"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/renderer"
	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/mitchellh/mapstructure"
)

// HeadConfig represents the configuration for the head section.
type HeadConfig struct {
	shared.Composite `mapstructure:",squash"`
	MetaData         map[string]string `mapstructure:"meta" description:"Metadata for the head section" example:"{!{head-metadata.hyperbricks}}"`
	Css              []string          `mapstructure:"css" description:"CSS files to include" example:"{!{head-css.hyperbricks}}"`
	Js               []string          `mapstructure:"js" description:"JavaScript files to include" example:"{!{head-js.hyperbricks}}"`
}

// HeadConfigGetName returns the HyperBricks type associated with the HeadConfig.
func HeadConfigGetName() string {
	return "<HEAD>"
}

// Validate ensures that the RENDER has valid data.
func (config *HeadConfig) Validate() []error {

	// standard validation on struct metadata of APIConfig
	warnings := shared.Validate(config)

	if config.ConfigType != "HEAD" {
		warnings = append(warnings, shared.ComponentError{
			Key:      config.Meta.Key,
			Path:     config.Meta.Path,
			Err:      fmt.Errorf("invalid type for ConcurentRenderConfig").Error(),
			Rejected: true,
		})
	}
	if len(config.Items) == 0 {
		warnings = append(warnings, shared.ComponentError{
			Key:  config.Meta.Key,
			Path: config.Meta.Path,
			Err:  fmt.Errorf("type RENDER items are empty").Error(),
		})
	}

	return warnings
}

// HeadRenderer handles rendering of COA content.
type HeadRenderer struct {
	renderer.CompositeRenderer
}

// Ensure HeadRenderer implements renderer.RenderComponent interface.
var _ shared.CompositeRenderer = (*HeadRenderer)(nil)

func (r *HeadRenderer) Types() []string {
	return []string{
		HeadConfigGetName(),
	}
}

// Render implements the RenderComponent interface for COA.
func (cr *HeadRenderer) Render(instance interface{}) (string, []error) {
	var headbuilder strings.Builder
	var errors []error
	var config HeadConfig

	err := mapstructure.Decode(instance, &config)
	if err != nil {
		return "", append(errors, shared.ComponentError{
			Err: fmt.Errorf("failed to decode instance into HeadConfig: %w", err).Error(),
		})
	}

	// appending page validation errors
	errors = append(errors, config.Validate()...)

	// Generate link tags for CSS files
	for _, cssFile := range config.Css {
		headbuilder.WriteString(fmt.Sprintf(`<link rel="stylesheet" href="%s">`, cssFile))
		headbuilder.WriteString("\n")
	}

	// Generate script tags for JS files
	for _, jsFile := range config.Js {
		headbuilder.WriteString(fmt.Sprintf(`<script src="%s"></script>`, jsFile))
		headbuilder.WriteString("\n")
	}

	if config.Items == nil {
		// js and css always shows up at 100 so user can choose to add before or after
		config.Items = make(map[string]interface{})
	}

	renderedHeadContent := headbuilder.String()

	config.Items["999"] = map[string]interface{}{
		"@type": "<HTML>",
		"value": `<meta name="generator" content="hyperbricks cms">`,
	}

	// check if css and js is not empty
	if renderedHeadContent != "" {
		config.Items["1000"] = map[string]interface{}{
			"@type": "<HTML>",
			"value": headbuilder.String(),
		}
	}
	config.Items["enclose"] = "<head>|</head>"

	result, errr := cr.RenderManager.Render(TreeRendererConfigGetName(), config.Items)
	errors = append(errors, errr...)

	return result, errors
}
