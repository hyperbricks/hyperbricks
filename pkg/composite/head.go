package composite

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/renderer"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/mitchellh/mapstructure"
)

const (
	headGeneratorItemKey = "generator"
	headPayloadItemKey   = "payload"
)

// HeadConfig represents the configuration for the head section.
type HeadConfig struct {
	shared.Composite `mapstructure:",squash"`
	Title            string            `mapstructure:"title" description:"The title of the hypermedia document" example:"{!{head-title.hyperbricks.yaml}}"`
	Favicon          string            `mapstructure:"favicon" description:"Path to the favicon for the hypermedia document" example:"{!{head-favicon.hyperbricks.yaml}}"`
	MetaData         map[string]string `mapstructure:"meta" description:"Metadata for the head section" example:"{!{head-meta.hyperbricks.yaml}}"`
	Css              []string          `mapstructure:"css" description:"CSS files to include" example:"{!{head-css.hyperbricks.yaml}}"`
	Js               []string          `mapstructure:"js" description:"JavaScript files to include" example:"{!{head-js.hyperbricks.yaml}}"`
}

// HeadConfigGetName returns the HyperBricks type associated with the HeadConfig.
func HeadConfigGetName() string {
	return "<HEAD>"
}

// Validate ensures that the RENDER has valid data.
func (config *HeadConfig) Validate() []error {

	// standard validation on struct metadata of APIConfig
	warnings := shared.Validate(config)

	if config.ConfigType != "<HEAD>" {
		warnings = append(warnings, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			File:     config.Composite.Meta.HyperBricksFile,
			Key:      config.Composite.Meta.HyperBricksKey,
			Path:     config.Composite.Meta.HyperBricksPath,
			Err:      fmt.Errorf("invalid type for HEAD").Error(),
			Rejected: true,
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
func (cr *HeadRenderer) Render(instance interface{}, ctx context.Context) (string, []error) {
	var headbuilder strings.Builder
	var errors []error
	var config HeadConfig

	switch typed := instance.(type) {
	case HeadConfig:
		config = typed
	case *HeadConfig:
		if typed != nil {
			config = *typed
		}
	default:
		err := mapstructure.Decode(instance, &config)
		if err != nil {
			return "", append(errors, shared.ComponentError{
				Hash: shared.GenerateHash(),
				File: config.Composite.Meta.HyperBricksFile,
				Key:  config.Composite.Meta.HyperBricksKey,
				Path: config.Composite.Meta.HyperBricksPath,
				Err:  fmt.Errorf("failed to decode instance into HeadConfig: %w", err).Error(),
			})
		}
	}

	// appending page validation errors
	errors = append(errors, config.Validate()...)

	// Generate favicon tag
	if config.Favicon != "" {
		headbuilder.WriteString(fmt.Sprintf(`<link rel="icon" type="image/x-icon" href="%s">`, config.Favicon))
		headbuilder.WriteString("\n")
	}

	// Generate title tag
	if config.Title != "" {
		headbuilder.WriteString(fmt.Sprintf(`<title>%s</title>`, config.Title))
		headbuilder.WriteString("\n")
	}

	// Generate meta tags

	headbuilder.WriteString(renderMeta(config.MetaData))

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
		config.Items = make(map[string]interface{})
	} else {
		config.Items = shared.CloneMapDeep(config.Items)
	}

	addGeneratedHeadItems(config.Items, headbuilder.String())
	config.Items["hyperbrickskey"] = config.Composite.Meta.HyperBricksKey
	config.Items["hyperbricksfile"] = config.Composite.Meta.HyperBricksFile
	config.Items["hyperbrickspath"] = config.Composite.Meta.HyperBricksPath + config.Composite.Meta.HyperBricksKey

	config.Items["enclose"] = "<head>|</head>"

	result, errr := cr.RenderManager.Render(TreeRendererConfigGetName(), config.Items, ctx)
	errors = append(errors, errr...)

	return result, errors
}

func addGeneratedHeadItems(items map[string]interface{}, renderedHeadContent string) {
	if items[headGeneratorItemKey] == nil {
		items[headGeneratorItemKey] = map[string]interface{}{
			"@type": "<HTML>",
			"value": `<meta name="generator" content="HyperBricks">`,
		}
	}
	if renderedHeadContent != "" && items[headPayloadItemKey] == nil {
		items[headPayloadItemKey] = map[string]interface{}{
			"@type": "<HTML>",
			"value": renderedHeadContent,
		}
	}
	appendGeneratedHeadItemsToOrder(items)
}

func appendGeneratedHeadItemsToOrder(items map[string]interface{}) {
	order := extractTreeOrder(items["@order"])
	if len(order) == 0 {
		return
	}

	seen := make(map[string]bool, len(order)+2)
	for _, key := range order {
		seen[key] = true
	}
	for _, key := range []string{headGeneratorItemKey, headPayloadItemKey} {
		if seen[key] || items[key] == nil {
			continue
		}
		order = append(order, key)
		seen[key] = true
	}
	items["@order"] = order
}

func renderMeta(meta map[string]string) string {
	// Extract keys
	keys := make([]string, 0, len(meta))
	for k := range meta {
		keys = append(keys, k)
	}

	// Sort keys alphabetically
	sort.Strings(keys)

	// Build the HTML
	var sb strings.Builder
	for _, k := range keys {
		v := meta[k]
		sb.WriteString(fmt.Sprintf(`<meta name="%s" content="%s">`, k, v))
		sb.WriteString("\n")
	}

	return sb.String()
}
