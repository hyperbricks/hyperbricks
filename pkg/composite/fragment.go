package composite

import (
	"context"
	"fmt"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/renderer"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/mitchellh/mapstructure"
)

// FragmentConfig represents configuration for a single fragment.
type FragmentConfig struct {
	shared.Composite   `mapstructure:",squash"`
	Response           HTTPResponseConfig     `mapstructure:"response" description:"HTTP status and response headers for this fragment route"`
	MetaDocDescription string                 `mapstructure:"@doc" description:"A <FRAGMENT> dynamically renders a part of an HTML page, allowing updates without a full page reload and improving performance and user experience." example:"{!{fragment-@doc.hyperbricks.yaml}}"`
	Beautify           *bool                  `mapstructure:"beautify" json:"Beautify,omitempty" description:"Override server.beautify for this object when rendered directly"`
	Title              string                 `mapstructure:"title" description:"The title of the fragment" example:"{!{fragment-title.hyperbricks.yaml}}"`
	Route              string                 `mapstructure:"route" description:"The route (URL-friendly identifier) for the fragment" example:"{!{fragment-route.hyperbricks.yaml}}"`
	Section            string                 `mapstructure:"section" description:"The section the fragment belongs to" example:"{!{fragment-section.hyperbricks.yaml}}"`
	Items              map[string]interface{} `mapstructure:",remain"`
	Enclose            string                 `mapstructure:"enclose" description:"Wrapping property for the fragment rendered output" example:"{!{fragment-enclose.hyperbricks.yaml}}"`
	Template           *TemplateOptions       `mapstructure:"template" description:"Template configurations for rendering the fragment" example:"{!{fragment-template.hyperbricks.yaml}}"`
	Static             string                 `mapstructure:"static" description:"Static file path associated with the fragment" example:"{!{fragment-static.hyperbricks.yaml}}"`
	Cache              string                 `mapstructure:"cache" description:"Cache expire string" example:"{!{fragment-cache.hyperbricks.yaml}}"`
	NoCache            bool                   `mapstructure:"nocache" description:"Explicitly disable cache" example:"{!{fragment-nocache.hyperbricks.yaml}}"`
	Index              int                    `mapstructure:"index" description:"Index number is a sort order option for the fragment menu section. See MENU and MENU_TEMPLATE for further explanation" example:"{!{fragment-index.hyperbricks.yaml}}"`
	ContentType        string                 `mapstructure:"content_type" description:"content type header definition"`
	Guard              *RouteGuardConfig      `mapstructure:"guard" json:",omitempty" description:"Optional pre-render route guard. When omitted or disabled, current FRAGMENT behavior remains unchanged"`
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
func (pr *FragmentRenderer) Render(instance interface{}, ctx context.Context) (string, []error) {

	var errors []error
	var config FragmentConfig

	var templatebuilder strings.Builder
	var treebuilder strings.Builder

	switch typed := instance.(type) {
	case FragmentConfig:
		config = typed
	case *FragmentConfig:
		if typed != nil {
			config = *typed
		}
	default:
		err := mapstructure.Decode(instance, &config)
		if err != nil {
			return "", append(errors, shared.ComponentError{
				Hash: shared.GenerateHash(),
				Err:  fmt.Errorf("failed to decode instance into HeadConfig: %w", err).Error(),
			})
		}
	}

	if config.ConfigType != "<FRAGMENT>" {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			File:     config.Composite.Meta.HyperBricksFile,
			Key:      config.HyperBricksKey,
			Path:     config.HyperBricksPath,
			Err:      fmt.Errorf("invalid type for Fragment").Error(),
			Rejected: true,
		})
	}

	// appending validation errors
	errors = append(errors, config.Validate()...)

	// HEAD?

	outputHtml := ""
	// TEMPLATE?
	if config.Template != nil {
		templateConfig := config.Template.ToRenderMap()
		// TO-DO: INSERT HEAD to TEMPLATE VALUES....
		templateConfig["hyperbricksfile"] = config.Composite.Meta.HyperBricksFile
		templateConfig["hyperbrickspath"] = config.Composite.Meta.HyperBricksPath + config.Composite.Meta.HyperBricksKey + ".template"

		result, errr := pr.RenderManager.Render("<TEMPLATE>", templateConfig, ctx)
		errors = append(errors, errr...)
		templatebuilder.WriteString(result)
		outputHtml = shared.EncloseContent(config.Enclose, templatebuilder.String())
	} else {

		if config.Composite.Items == nil {
			config.Composite.Items = make(map[string]interface{})
		} else {
			config.Composite.Items = shared.CloneMapDeep(config.Composite.Items)
		}
		// TREE
		config.Composite.Items["hyperbricksfile"] = config.Composite.Meta.HyperBricksFile
		config.Composite.Items["hyperbrickspath"] = config.Composite.Meta.HyperBricksPath + config.Composite.Meta.HyperBricksKey

		result, errr := pr.RenderManager.Render(TreeRendererConfigGetName(), config.Composite.Items, ctx)
		errors = append(errors, errr...)
		treebuilder.WriteString(result)
		outputHtml = shared.EncloseContent(config.Enclose, treebuilder.String())
	}

	// Wrap the content with the HTML structure
	finalHTML := outputHtml

	return finalHTML, errors
}
