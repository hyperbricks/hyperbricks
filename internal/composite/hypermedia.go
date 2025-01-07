package composite

import (
	"fmt"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/renderer"
	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/mitchellh/mapstructure"
)

// HyperMediaConfig represents configuration for a single page.
type HyperMediaConfig struct {
	shared.Composite `mapstructure:",squash"`
	ContentType      string                 `mapstructure:"@type" category:"renderer" description:"HyperBricks type: PAGE" example:"{!{page.hyperbricks}}"`
	Title            string                 `mapstructure:"title" description:"The title of the page" example:"{!{page-title.hyperbricks}}"`
	Route            string                 `mapstructure:"route" description:"The route (URL-friendly identifier) for the page" example:"{!{page-route.hyperbricks}}"`
	Section          string                 `mapstructure:"section" description:"The section the page belongs to" example:"{!{page-section.hyperbricks}}"`
	Items            map[string]interface{} `mapstructure:",remain"`
	BodyTag          string                 `mapstructure:"bodytag" description:"Special body wrap with use of |. Please note that this will not work when a page.template is configured. In that case, you have to add the bodytag in the template." example:"{!{page-bodywrap.hyperbricks}}"`
	Enclose          string                 `mapstructure:"enclose" description:"Wrapping property for the page" example:"{!{page-wrap.hyperbricks}}"`
	Favicon          string                 `mapstructure:"favicon" description:"Path to the favicon for the page" example:"{!{page-favicon.hyperbricks}}"`
	Template         map[string]interface{} `mapstructure:"template" description:"Template configurations for rendering the page" example:"{!{page-template.hyperbricks}}"`
	IsStatic         bool                   `mapstructure:"isstatic"`
	Static           string                 `mapstructure:"static" description:"Static file path associated with the page" example:"{!{page-static.hyperbricks}}"`
	Index            int                    `mapstructure:"index" description:"Index number is a sort order option for the page menu section. See MENU and MENU_TEMPLATE for further explanation" example:"{!{page-index.hyperbricks}}"`
	Meta             map[string]string      `mapstructure:"meta" description:"Metadata for the page, such as descriptions and keywords" example:"{!{page-meta.hyperbricks}}"`
	Doctype          string                 `mapstructure:"doctype" description:"Doctype for the HTML document" example:"{!{page-doctype.hyperbricks}}"`
	HtmlTag          string                 `mapstructure:"htmltag" description:"The opening HTML tag with attributes" example:"{!{page-htmltag.hyperbricks}}"`
	Head             map[string]interface{} `mapstructure:"head" description:"Configurations for the head section of the page" example:"{!{page-head.hyperbricks}}"`
	Css              []string               `mapstructure:"css" description:"CSS files associated with the page" example:"{!{page-css.hyperbricks}}"`
	Js               []string               `mapstructure:"js" description:"JavaScript files associated with the page" example:"{!{page-js.hyperbricks}}"`
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
		result, errr := pr.RenderManager.Render("HEAD", config.Head)
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
		outputHtml = shared.EncloseContent(config.BodyTag, templatebuilder.String())
	} else {

		// TREE
		if config.Composite.Items != nil {
			config.Composite.Items["file"] = config.File
			config.Composite.Items["path"] = config.File + ":" + config.Path
		}

		result, errr := pr.RenderManager.Render(TreeRendererConfigGetName(), config.Composite.Items)
		errors = append(errors, errr...)
		treebuilder.WriteString(result)
		outputHtml = shared.EncloseContent(config.BodyTag, treebuilder.String())
	}

	// TO-DO: INSERT HEAD

	// PAGE COMPOSITION.....

	headHtml := headbuilder.String()
	// Wrap the content with the HTML structure
	finalHTML := fmt.Sprintf("%s<html>%s%s</html>", config.Doctype, headHtml, outputHtml)
	//shared.EncloseContent(fmt.Sprintf("%s<html>%s|</html>", config.Doctype, headHtml), outputHtml)

	return finalHTML, errors
}
