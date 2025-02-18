package composite

import (
	"context"
	"fmt"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/renderer"
	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/mitchellh/mapstructure"
)

// HyperMediaConfig represents configuration hypermedia.
type HyperMediaConfig struct {
	shared.Composite   `mapstructure:",squash"`
	MetaDocDescription string                 `mapstructure:"@doc" description:"HYPERMEDIA description" example:"{!{hypermedia-@doc.hyperbricks}}"`
	Title              string                 `mapstructure:"title" description:"The title of the hypermedia site" example:"{!{hypermedia-title.hyperbricks}}"`
	Route              string                 `mapstructure:"route" description:"The route (URL-friendly identifier) for the hypermedia" example:"{!{hypermedia-route.hyperbricks}}"`
	Section            string                 `mapstructure:"section" description:"The section the hypermedia belongs to. This can be used with the component <MENU> for example." example:"{!{hypermedia-section.hyperbricks}}"`
	Items              map[string]interface{} `mapstructure:",remain"`
	BodyTag            string                 `mapstructure:"bodytag" description:"Special body enclose with use of |. Please note that this will not work when a <HYPERMEDIA>.template is configured. In that case, you have to add the bodytag in the template." example:"{!{hypermedia-bodytag.hyperbricks}}"`
	Enclose            string                 `mapstructure:"enclose" description:"Enclosure of the property for the hypermedia" example:"{!{hypermedia-enclose.hyperbricks}}"`
	Favicon            string                 `mapstructure:"favicon" description:"Path to the favicon for the hypermedia" example:"{!{hypermedia-favicon.hyperbricks}}"`
	Template           map[string]interface{} `mapstructure:"template" description:"Template configurations for rendering the hypermedia. See <TEMPLATE> for field descriptions." example:"{!{hypermedia-template.hyperbricks}}"`
	IsStatic           bool                   `mapstructure:"isstatic" exclude:"true"`
	Cache              string                 `mapstructure:"cache" description:"Cache expire string" example:"{!{hypermedia-cache.hyperbricks}}"`
	NoCache            bool                   `mapstructure:"nocache" description:"Explicitly deisable cache" example:"{!{hypermedia-nocache.hyperbricks}}"`
	Static             string                 `mapstructure:"static" description:"Static file path associated with the hypermedia, for rendering out the hypermedia to static files." example:"{!{hypermedia-static.hyperbricks}}"`
	Index              int                    `mapstructure:"index" description:"Index number is a sort order option for the hypermedia defined in the section field. See <MENU> for further explanation and field options" example:"{!{hypermedia-index.hyperbricks}}"`
	Doctype            string                 `mapstructure:"doctype" description:"Alternative Doctype for the HTML document" example:"{!{hypermedia-doctype.hyperbricks}}"`
	HtmlTag            string                 `mapstructure:"htmltag" description:"The opening HTML tag with attributes" example:"{!{hypermedia-htmltag.hyperbricks}}"`
	Head               map[string]interface{} `mapstructure:"head" description:"Configurations for the head section of the hypermedia" example:"{!{hypermedia-head.hyperbricks}}"`
}

// HyperMediaConfigGetName returns the HyperBricks type associated with the HyperMediaConfig.
func HyperMediaConfigGetName() string {
	return "<HYPERMEDIA>"
}

// Validate ensures that the page has valid data.
func (hm *HyperMediaConfig) Validate() []error {
	if hm.Doctype == "" {
		hm.Doctype = "<!DOCTYPE html>"
	}

	if hm.HtmlTag == "" {
		hm.HtmlTag = "<html>"
	}

	var warnings []error
	return warnings
}

// HyperMediaRenderer handles rendering of PAGE content.
type HyperMediaRenderer struct {
	renderer.CompositeRenderer
}

// Ensure HyperMediaRenderer implements renderer.RenderComponent interface.
var _ shared.CompositeRenderer = (*HyperMediaRenderer)(nil)

func (r *HyperMediaRenderer) Types() []string {
	return []string{
		HyperMediaConfigGetName(),
	}
}

// Render implements the RenderComponent interface.
func (pr *HyperMediaRenderer) Render(instance interface{}, ctx context.Context) (string, []error) {

	var errors []error
	var config HyperMediaConfig

	err := mapstructure.Decode(instance, &config)
	if err != nil {
		return "", append(errors, shared.ComponentError{
			Key:  config.Composite.Meta.HyperBricksKey,
			Path: config.Composite.Meta.HyperBricksPath,
			File: config.Composite.Meta.HyperBricksFile,
			Type: "<HYPERMEDIA>",
			Err:  fmt.Errorf("failed to decode instance into HeadConfig: %w", err).Error(),
		})
	}

	if config.ConfigType != "<HYPERMEDIA>" {
		errors = append(errors, shared.ComponentError{
			Key:      config.Composite.Meta.HyperBricksKey,
			Path:     config.Composite.Meta.HyperBricksPath,
			File:     config.Composite.Meta.HyperBricksFile,
			Type:     "<HYPERMEDIA>",
			Err:      fmt.Errorf("invalid type").Error(),
			Rejected: true,
		})
	}

	// appending validation errors
	errors = append(errors, config.Validate()...)

	// HEAD?
	var headbuilder strings.Builder
	var templatebuilder strings.Builder
	var treebuilder strings.Builder

	if config.BodyTag == "" {
		// emty bodyenclose fallback
		config.BodyTag = "<body>|</body>"
	}

	// Not sure how to handle this situation....
	// if no <HEAD> is defined create it
	//if config.Head == nil {
	//	config.Head = make(map[string]interface{})
	//}

	// If a main header config is present, render add it to the string builder
	if config.Head != nil || config.Title != "" || config.Favicon != "" {

		if config.Head == nil {
			config.Head = make(map[string]interface{})
		}

		//head := shared.StructToMap(config.Head)
		config.Head["@type"] = HeadConfigGetName()
		config.Head["hyperbricksfile"] = config.Composite.Meta.HyperBricksFile
		config.Head["hyperbrickspath"] = config.Composite.Meta.HyperBricksPath + config.Composite.Meta.HyperBricksKey

		if config.Title != "" {
			config.Head["title"] = config.Title
		}

		if config.Favicon != "" {
			config.Head["favicon"] = config.Favicon
		}

		result, errr := pr.RenderManager.Render(HeadConfigGetName(), config.Head, ctx)
		errors = append(errors, errr...)
		headbuilder.WriteString(result)
	}
	outputHtml := ""
	// TEMPLATE?
	if config.Template != nil {
		config.Template["hyperbricksfile"] = config.Composite.Meta.HyperBricksFile
		config.Template["hyperbrickspath"] = config.Composite.Meta.HyperBricksKey + ".template"

		// INSERT HEAD to TEMPLATE VALUES....
		// Ensure 'values' exists inside Template
		if _, exists := config.Template["values"]; !exists {
			config.Template["values"] = make(map[string]interface{})
		}

		// Set 'head' inside 'values'
		if config.Head != nil {
			config.Template["values"].(map[string]interface{})["head"] = config.Head
		}

		result, errr := pr.RenderManager.Render("<TEMPLATE>", config.Template, ctx)
		errors = append(errors, errr...)
		templatebuilder.WriteString(result)
		outputHtml = shared.EncloseContent(config.Enclose, templatebuilder.String())
	} else {

		// TREE
		if config.Composite.Items != nil {
			config.Composite.Items["hyperbricksfile"] = config.Composite.Meta.HyperBricksFile
			config.Composite.Items["hyperbrickspath"] = config.Composite.Meta.HyperBricksPath + config.Composite.Meta.HyperBricksKey
		}

		result, errr := pr.RenderManager.Render(TreeRendererConfigGetName(), config.Composite.Items, ctx)
		errors = append(errors, errr...)
		treebuilder.WriteString(result)
		outputHtml = shared.EncloseContent(config.Enclose, treebuilder.String())
	}
	finalHTML := ""
	if config.Template != nil {
		finalHTML = outputHtml
	} else {
		headHtml := headbuilder.String()
		// Wrap the content with the HTML structure
		finalHTML = fmt.Sprintf("%s%s%s%s</html>", config.Doctype, config.HtmlTag, headHtml, shared.EncloseContent(config.BodyTag, outputHtml))

	}

	return finalHTML, errors
}
