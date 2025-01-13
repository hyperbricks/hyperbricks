package composite

import (
	"fmt"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/renderer"
	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/mitchellh/mapstructure"
)

// HyperMediaConfig represents configuration hypermedia.
type HyperMediaConfig struct {
	shared.Composite   `mapstructure:",squash"`
	MetaDocDescription string                 `mapstructure:"@doc" description:"HYPERMEDIA description" example:"{!{hypermedia.hyperbricks}}"`
	Title              string                 `mapstructure:"title" description:"The title of the hypermedia site" example:"{!{hypermedia-title.hyperbricks}}"`
	Route              string                 `mapstructure:"route" description:"The route (URL-friendly identifier) for the hypermedia" example:"{!{hypermedia-route.hyperbricks}}"`
	Section            string                 `mapstructure:"section" description:"The section the hypermedia belongs to. This can be used with the component <MENU> for example." example:"{!{hypermedia-section.hyperbricks}}"`
	Items              map[string]interface{} `mapstructure:",remain"`
	BodyTag            string                 `mapstructure:"bodytag" description:"Special body wrap with use of |. Please note that this will not work when a <HYPERMEDIA>.template is configured. In that case, you have to add the bodytag in the template." example:"{!{hypermedia-bodywrap.hyperbricks}}"`
	Enclose            string                 `mapstructure:"enclose" description:"Enclosure of the property for the hypermedia" example:"{!{hypermedia-wrap.hyperbricks}}"`
	Favicon            string                 `mapstructure:"favicon" description:"Path to the favicon for the hypermedia" example:"{!{hypermedia-favicon.hyperbricks}}"`
	Template           map[string]interface{} `mapstructure:"template" description:"Template configurations for rendering the hypermedia. See <TEMPLATE> for field descriptions." example:"{!{hypermedia-template.hyperbricks}}"`
	IsStatic           bool                   `mapstructure:"isstatic"`
	Static             string                 `mapstructure:"static" description:"Static file path associated with the hypermedia, for rendering out the hypermedia to static files." example:"{!{hypermedia-static.hyperbricks}}"`
	Index              int                    `mapstructure:"index" description:"Index number is a sort order option for the hypermedia defined in the section field. See <MENU> for further explanation and field options" example:"{!{hypermedia-index.hyperbricks}}"`
	Meta               map[string]string      `mapstructure:"meta" description:"Metadata for the hypermedia, such as descriptions and keywords" example:"{!{hypermedia-meta.hyperbricks}}"`
	Doctype            string                 `mapstructure:"doctype" description:"Alternative Doctype for the HTML document" example:"{!{hypermedia-doctype.hyperbricks}}"`
	HtmlTag            string                 `mapstructure:"htmltag" description:"The opening HTML tag with attributes" example:"{!{hypermedia-htmltag.hyperbricks}}"`
	Head               map[string]interface{} `mapstructure:"head" description:"Configurations for the head section of the hypermedia" example:"{!{hypermedia-head.hyperbricks}}"`
	Css                []string               `mapstructure:"css" description:"CSS files associated with the hypermedia" example:"{!{hypermedia-css.hyperbricks}}"`
	Js                 []string               `mapstructure:"js" description:"JavaScript files associated with the hypermedia" example:"{!{hypermedia-js.hyperbricks}}"`
}

// HyperMediaConfigGetName returns the HyperBricks type associated with the HyperMediaConfig.
func HyperMediaConfigGetName() string {
	return "<HYPERMEDIA>"
}

// Validate ensures that the page has valid data.
func (page *HyperMediaConfig) Validate() []error {
	if page.Doctype == "" {
		page.Doctype = "<!DOCTYPE html>"
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
func (pr *HyperMediaRenderer) Render(instance interface{}) (string, []error) {

	var errors []error
	var config HyperMediaConfig

	err := mapstructure.Decode(instance, &config)
	if err != nil {
		return "", append(errors, shared.ComponentError{
			Err: fmt.Errorf("failed to decode instance into HeadConfig: %w", err).Error(),
		})
	}

	if config.ConfigType != "<HYPERMEDIA>" {
		errors = append(errors, shared.ComponentError{
			Key:      config.Key,
			Path:     config.Path,
			Err:      fmt.Errorf("invalid type for ConcurentRenderConfig").Error(),
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
		// emty bodywrap fallback
		config.BodyTag = "<body>|</body>"
	}

	// If a main header config is present, render add it to the string builder
	if config.Head != nil {
		config.Head["@type"] = HeadConfigGetName()
		config.Head["file"] = config.File
		config.Head["path"] = config.File + ":" + config.Path
		result, errr := pr.RenderManager.Render(HeadConfigGetName(), config.Head)
		errors = append(errors, errr...)
		headbuilder.WriteString(result)
	}
	outputHtml := ""
	// TEMPLATE?
	if config.Template != nil {
		config.Template["file"] = config.File
		config.Template["path"] = config.File + ":" + config.Path
		// TO-DO: INSERT HEAD to TEMPLATE VALUES....
		result, errr := pr.RenderManager.Render("<TEMPLATE>", config.Template)
		errors = append(errors, errr...)
		templatebuilder.WriteString(result)
		outputHtml = shared.EncloseContent(config.Enclose, templatebuilder.String())
	} else {

		// TREE
		if config.Composite.Items != nil {
			config.Composite.Items["file"] = config.File
			config.Composite.Items["path"] = config.File + ":" + config.Path
		}

		result, errr := pr.RenderManager.Render(TreeRendererConfigGetName(), config.Composite.Items)
		errors = append(errors, errr...)
		treebuilder.WriteString(result)
		outputHtml = shared.EncloseContent(config.Enclose, treebuilder.String())
	}

	headHtml := headbuilder.String()

	// Wrap the content with the HTML structure
	finalHTML := fmt.Sprintf("%s<html>%s%s</html>", config.Doctype, headHtml, shared.EncloseContent(config.BodyTag, outputHtml))

	return finalHTML, errors
}
