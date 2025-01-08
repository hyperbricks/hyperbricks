package composite

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/renderer"
	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/mitchellh/mapstructure"
)

// FragmentConfig represents configuration for a single endpoint.
type FragmentConfig struct {
	shared.Composite `mapstructure:",squash"`
	HxResponse       `mapstructure:"response"`
	HxResponseWriter http.ResponseWriter    `mapstructure:"hx_response"`
	Title            string                 `mapstructure:"title" description:"The title of the endpoint" example:"{!{endpoint-title.hyperbricks}}"`
	Route            string                 `mapstructure:"route" description:"The route (URL-friendly identifier) for the endpoint" example:"{!{endpoint-route.hyperbricks}}"`
	Section          string                 `mapstructure:"section" description:"The section the endpoint belongs to" example:"{!{endpoint-section.hyperbricks}}"`
	Items            map[string]interface{} `mapstructure:",remain"`
	BodyTag          string                 `mapstructure:"bodytag" description:"Special body wrap with use of |. Please note that this will not work when a endpoint.template is configured. In that case, you have to add the bodytag in the template." example:"{!{endpoint-bodywrap.hyperbricks}}"`
	Enclose          string                 `mapstructure:"enclose" description:"Wrapping property for the endpoint" example:"{!{endpoint-wrap.hyperbricks}}"`
	Favicon          string                 `mapstructure:"favicon" description:"Path to the favicon for the endpoint" example:"{!{endpoint-favicon.hyperbricks}}"`
	Template         map[string]interface{} `mapstructure:"template" description:"Template configurations for rendering the endpoint" example:"{!{endpoint-template.hyperbricks}}"`
	File             string                 `mapstructure:"@file"`
	IsStatic         bool                   `mapstructure:"isstatic"`
	Static           string                 `mapstructure:"static" description:"Static file path associated with the endpoint" example:"{!{endpoint-static.hyperbricks}}"`
	Index            int                    `mapstructure:"index" description:"Index number is a sort order option for the endpoint menu section. See MENU and MENU_TEMPLATE for further explanation" example:"{!{endpoint-index.hyperbricks}}"`
	Meta             map[string]string      `mapstructure:"meta" description:"Metadata for the endpoint, such as descriptions and keywords" example:"{!{endpoint-meta.hyperbricks}}"`
	Doctype          string                 `mapstructure:"doctype" description:"Doctype for the HTML document" example:"{!{endpoint-doctype.hyperbricks}}"`
	HtmlTag          string                 `mapstructure:"htmltag" description:"The opening HTML tag with attributes" example:"{!{endpoint-htmltag.hyperbricks}}"`
	Head             map[string]interface{} `mapstructure:"head" description:"Configurations for the head section of the endpoint" example:"{!{endpoint-head.hyperbricks}}"`
	Css              []string               `mapstructure:"css" description:"CSS files associated with the endpoint" example:"{!{endpoint-css.hyperbricks}}"`
	Js               []string               `mapstructure:"js" description:"JavaScript files associated with the endpoint" example:"{!{endpoint-js.hyperbricks}}"`
}

// FragmentConfigGetName returns the HyperBricks type associated with the FragmentConfig.
func FragmentConfigGetName() string {
	return "<FRAGMENT>"
}

// Validate ensures that the endpoint has valid data.
func (endpoint *FragmentConfig) Validate() []error {
	var warnings []error
	return warnings
}

// FragmentRenderer handles rendering of PAGE content.
type FragmentRenderer struct {
	renderer.CompositeRenderer
}

// Ensure FragmentRenderer implements renderer.RenderComponent interface.
var _ shared.CompositeRenderer = (*FragmentRenderer)(nil)

func (r *FragmentRenderer) Types() []string {
	return []string{
		FragmentConfigGetName(),
	}
}

// Render implements the RenderComponent interface.
func (pr *FragmentRenderer) Render(instance interface{}) (string, []error) {

	var errors []error
	var config FragmentConfig

	err := mapstructure.Decode(instance, &config)
	if err != nil {
		return "", append(errors, shared.ComponentError{
			Err: fmt.Errorf("failed to decode instance into HeadConfig: %w", err).Error(),
		})
	}

	if config.ConfigType != "<FRAGMENT>" {
		errors = append(errors, shared.ComponentError{
			Key:      config.Key,
			Path:     config.Path,
			Err:      fmt.Errorf("invalid type for Fragment").Error(),
			Rejected: true,
		})
	}

	// appending validation errors
	errors = append(errors, config.Validate()...)

	// HEAD?

	var templatebuilder strings.Builder
	var treebuilder strings.Builder

	outputHtml := ""
	// TEMPLATE?
	if config.Template != nil {
		// TO-DO: INSERT HEAD to TEMPLATE VALUES....

		result, errr := pr.RenderManager.Render("<TEMPLATE>", config.Template)
		errors = append(errors, errr...)
		templatebuilder.WriteString(result)
		outputHtml = shared.EncloseContent(config.BodyTag, templatebuilder.String())
	} else {
		// TREE
		result, errr := pr.RenderManager.Render(TreeRendererConfigGetName(), config.Composite.Items)
		errors = append(errors, errr...)
		treebuilder.WriteString(result)
		outputHtml = shared.EncloseContent(config.BodyTag, treebuilder.String())
	}

	// Wrap the content with the HTML structure
	finalHTML := outputHtml

	SetHeadersFromHxRequest(&config.HxResponse, config.HxResponseWriter)

	return finalHTML, errors
}
