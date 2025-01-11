package composite

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/renderer"
	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/mitchellh/mapstructure"
)

// FragmentConfig represents configuration for a single fragment.
type FragmentConfig struct {
	shared.Composite `mapstructure:",squash"`
	HxResponse       `mapstructure:"response"`
	HxResponseWriter http.ResponseWriter    `mapstructure:"hx_response"`
	Title            string                 `mapstructure:"title" description:"The title of the fragment" example:"{!{fragment-title.hyperbricks}}"`
	Route            string                 `mapstructure:"route" description:"The route (URL-friendly identifier) for the fragment" example:"{!{fragment-route.hyperbricks}}"`
	Section          string                 `mapstructure:"section" description:"The section the fragment belongs to" example:"{!{fragment-section.hyperbricks}}"`
	Items            map[string]interface{} `mapstructure:",remain"`
	BodyTag          string                 `mapstructure:"bodytag" description:"Special body wrap with use of |. Please note that this will not work when a fragment.template is configured. In that case, you have to add the bodytag in the template." example:"{!{fragment-bodywrap.hyperbricks}}"`
	Enclose          string                 `mapstructure:"enclose" description:"Wrapping property for the fragment" example:"{!{fragment-enclose.hyperbricks}}"`
	Favicon          string                 `mapstructure:"favicon" description:"Path to the favicon for the fragment" example:"{!{fragment-favicon.hyperbricks}}"`
	Template         map[string]interface{} `mapstructure:"template" description:"Template configurations for rendering the fragment" example:"{!{fragment-template.hyperbricks}}"`
	File             string                 `mapstructure:"@file"`
	IsStatic         bool                   `mapstructure:"isstatic"`
	Static           string                 `mapstructure:"static" description:"Static file path associated with the fragment" example:"{!{fragment-static.hyperbricks}}"`
	Index            int                    `mapstructure:"index" description:"Index number is a sort order option for the fragment menu section. See MENU and MENU_TEMPLATE for further explanation" example:"{!{fragment-index.hyperbricks}}"`
	Meta             map[string]string      `mapstructure:"meta" description:"Metadata for the fragment, such as descriptions and keywords" example:"{!{fragment-meta.hyperbricks}}"`
	Doctype          string                 `mapstructure:"doctype" description:"Doctype for the HTML document" example:"{!{fragment-doctype.hyperbricks}}"`
	HtmlTag          string                 `mapstructure:"htmltag" description:"The opening HTML tag with attributes" example:"{!{fragment-htmltag.hyperbricks}}"`
	Head             map[string]interface{} `mapstructure:"head" description:"Configurations for the head section of the fragment" example:"{!{fragment-head.hyperbricks}}"`
	Css              []string               `mapstructure:"css" description:"CSS files associated with the fragment" example:"{!{fragment-css.hyperbricks}}"`
	Js               []string               `mapstructure:"js" description:"JavaScript files associated with the fragment" example:"{!{fragment-js.hyperbricks}}"`
}

// FragmentConfigGetName returns the HyperBricks type associated with the FragmentConfig.
func FragmentConfigGetName() string {
	return "<FRAGMENT>"
}

// Validate ensures that the fragment has valid data.
func (fragment *FragmentConfig) Validate() []error {
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
		outputHtml = shared.EncloseContent(config.Enclose, templatebuilder.String())
	} else {
		// TREE
		result, errr := pr.RenderManager.Render(TreeRendererConfigGetName(), config.Composite.Items)
		errors = append(errors, errr...)
		treebuilder.WriteString(result)
		outputHtml = shared.EncloseContent(config.Enclose, treebuilder.String())
	}

	// Wrap the content with the HTML structure
	finalHTML := outputHtml
	if config.HxResponseWriter != nil {
		SetHeadersFromHxRequest(&config.HxResponse, config.HxResponseWriter)
	}

	return finalHTML, errors
}
