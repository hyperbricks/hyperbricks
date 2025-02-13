package composite

import (
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/renderer"
	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/mitchellh/mapstructure"
)

type HxResponse struct {
	HxTemplateResult     string // just for output of the parsed template
	HxLocation           string `mapstructure:"hx_location" header:"HX-Location"  description:"allows you to do a client-side redirect that does not do a full page reload" `
	HxPushedUrl          string `mapstructure:"hx_push_url" header:"HX-Pushed-Url" description:"pushes a new url into the history stack"`
	HxRedirect           string `mapstructure:"hx_redirect" header:"HX-Redirect" description:"can be used to do a client-side redirect to a new location"`
	HxRefresh            string `mapstructure:"hx_refresh" header:"HX-Refresh" description:"if set to 'true' the client-side will do a full refresh of the page"`
	HxReplaceUrl         string `mapstructure:"hx_replace_url" header:"HX-Replace-Url" description:"replaces the current url in the location bar"`
	HxReswap             string `mapstructure:"hx_reswap" header:"HX-Reswap" description:"allows you to specify how the response will be swapped"`
	HxRetarget           string `mapstructure:"hx_retarget" header:"HX-Retarget" description:"a css selector that updates the target of the content update"`
	HxReselect           string `mapstructure:"hx_reselect" header:"HX-Reselect" description:"a css selector that allows you to choose which part of the response is used to be swapped in"`
	HxTrigger            string `mapstructure:"hx_trigger" header:"HX-Trigger" description:"allows you to trigger client-side events"`
	HxTriggerafterSettle string `mapstructure:"hx_trigger_after_settle"  header:"HX-Trigger-After-Settle" description:"allows you to trigger client-side events after the settle step"`
	HxTriggerafterSwap   string `mapstructure:"hx_trigger_after_swap"  header:"HX-Trigger-After-Swap" description:"allows you to trigger client-side events after the swap step"`
}

// FragmentConfig represents configuration for a single fragment.
type FragmentConfig struct {
	shared.Composite   `mapstructure:",squash"`
	HxResponse         `mapstructure:"response" description:"HTMX response header configuration." example:"{!{fragment-response.hyperbricks}}"`
	MetaDocDescription string                 `mapstructure:"@doc" description:"A <FRAGMENT> dynamically renders a part of an HTML page, allowing updates without a full page reload and improving performance and user experience." example:"{!{fragment-@doc.hyperbricks}}"`
	HxResponseWriter   http.ResponseWriter    `mapstructure:"hx_response" exclude:"true"`
	Title              string                 `mapstructure:"title" description:"The title of the fragment" example:"{!{fragment-title.hyperbricks}}"`
	Route              string                 `mapstructure:"route" description:"The route (URL-friendly identifier) for the fragment" example:"{!{fragment-route.hyperbricks}}"`
	Section            string                 `mapstructure:"section" description:"The section the fragment belongs to" example:"{!{fragment-section.hyperbricks}}"`
	Items              map[string]interface{} `mapstructure:",remain"`
	Enclose            string                 `mapstructure:"enclose" description:"Wrapping property for the fragment rendered output" example:"{!{fragment-enclose.hyperbricks}}"`
	Template           map[string]interface{} `mapstructure:"template" description:"Template configurations for rendering the fragment" example:"{!{fragment-template.hyperbricks}}"`
	//File               string                 `mapstructure:"@file" exclude:"true"`
	IsStatic bool   `mapstructure:"isstatic" exclude:"true"`
	Static   string `mapstructure:"static" description:"Static file path associated with the fragment" example:"{!{fragment-static.hyperbricks}}"`
	Index    int    `mapstructure:"index" description:"Index number is a sort order option for the fragment menu section. See MENU and MENU_TEMPLATE for further explanation" example:"{!{fragment-index.hyperbricks}}"`
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

	var templatebuilder strings.Builder
	var treebuilder strings.Builder

	err := mapstructure.Decode(instance, &config)
	if err != nil {
		return "", append(errors, shared.ComponentError{
			Err: fmt.Errorf("failed to decode instance into HeadConfig: %w", err).Error(),
		})
	}

	if config.ConfigType != "<FRAGMENT>" {
		errors = append(errors, shared.ComponentError{
			File:     config.Composite.Meta.File,
			Key:      config.Key,
			Path:     config.Path,
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
		// TO-DO: INSERT HEAD to TEMPLATE VALUES....
		config.Template["file"] = config.Composite.Meta.File
		config.Template["path"] = config.Composite.Meta.Path + config.Composite.Meta.Key + ".template"

		result, errr := pr.RenderManager.Render("<TEMPLATE>", config.Template)
		errors = append(errors, errr...)
		templatebuilder.WriteString(result)
		outputHtml = shared.EncloseContent(config.Enclose, templatebuilder.String())
	} else {

		// TREE
		config.Composite.Items["file"] = config.Composite.Meta.File
		config.Composite.Items["path"] = config.Composite.Meta.Path + config.Composite.Meta.Key

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

func SetHeadersFromHxRequest(config *HxResponse, writer http.ResponseWriter) {
	// Use reflection to access struct fields
	v := reflect.ValueOf(*config)
	t := reflect.TypeOf(*config)

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Use the "header" tag to get the HTTP header name
		headerName := fieldType.Tag.Get("header")
		if headerName == "" || !field.IsValid() || (field.Kind() == reflect.String && field.String() == "") {
			// Skip fields without a header tag or empty string fields
			continue
		}

		// Convert the field value to a string
		headerValue := ""
		switch field.Kind() {
		case reflect.String:
			headerValue = field.String()
		case reflect.Int, reflect.Int64, reflect.Float64, reflect.Bool:
			headerValue = fmt.Sprintf("%v", field.Interface())
		default:
			// Skip unsupported types
			continue
		}

		// Set the header using Go's default canonicalization
		writer.Header().Set(headerName, headerValue)
		log.Println(writer.Header())
	}
}
