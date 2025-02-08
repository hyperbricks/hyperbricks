package composite

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"strings"

	"github.com/mitchellh/mapstructure"

	"github.com/hyperbricks/hyperbricks/internal/renderer"
	"github.com/hyperbricks/hyperbricks/internal/shared"
)

// TemplateConfig represents the configuration for a TEMPLATE type.
type TemplateConfig struct {
	shared.Composite   `mapstructure:",squash"`
	MetaDocDescription string                 `mapstructure:"@doc" description:"TEMPLATE description" example:"{!{template-@doc.hyperbricks}}"`
	Template           string                 `mapstructure:"template" description:"The template used for rendering" example:"{!{template-template.hyperbricks}}"`
	IsTemplate         bool                   `mapstructure:"istemplate" description:"Determines if the field is a template or reference" example:"{!{template-istemplate.hyperbricks}}"`
	Values             map[string]interface{} `mapstructure:"values" description:"Key-value pairs for template rendering" example:"{!{template-values.hyperbricks}}"`
	Enclose            string                 `mapstructure:"enclose" description:"Enclosing property for the template rendered output" example:"{!{template-enclose.hyperbricks}}"`
}

type TemplateRenderer struct {
	renderer.CompositeRenderer
}

// Ensure CompositeRenderer implements RenderComponent with the concrete type `shared.CompositeRenderer`.
var _ shared.CompositeRenderer = (*TemplateRenderer)(nil)

func TemplateConfigGetName() string {
	return "<TEMPLATE>"
}
func (r *TemplateRenderer) Types() []string {
	return []string{
		TemplateConfigGetName(),
	}
}

func (head *TemplateConfig) Validate() []error {
	var warnings []error
	return warnings
}

func (tr *TemplateRenderer) Render(instance interface{}) (string, []error) {
	var templatebuilder strings.Builder
	var errors []error

	// Decode the instance into TemplateConfig without type assertion
	var config TemplateConfig
	err := mapstructure.Decode(instance, &config)
	if err != nil {
		return "", append(errors, shared.ComponentError{
			Err: fmt.Errorf("failed to decode instance into HeadConfig: %w", err).Error(),
		})
	}
	// appending validation errors
	errors = append(errors, config.Validate()...)

	var templateContent string

	if !config.IsTemplate {
		config.IsTemplate = checkString(config.Template)
	}

	if config.IsTemplate {
		templateContent = config.Template
	} else {
		// Fetch the template content
		tc, found := tr.TemplateProvider(config.Template)
		if !found {
			return fmt.Sprintf("<!-- Template '%s' not found -->", config.Template), nil
		} else {
			templateContent = tc
		}
	}

	// Retrieve sorted keys using the utility function
	sortedKeys := shared.SortedUniqueKeys(config.Values)
	var treeRenderOutPut = make(map[string]interface{})
	for _, key := range sortedKeys {
		//value := config.Values[key].(map[string]interface{})
		if value, ok := config.Values[key].(map[string]interface{}); ok {

			if componentType, ok := value["@type"].(string); ok {
				// Now tree is a map[string]interface{}
				result, render_errors := tr.RenderManager.Render(componentType, value) // als dit een platte value is ?

				// if componentType == "<HTML>" {
				// 	treeRenderOutPut[key] = template.HTML(result)
				// } else {
				// 	treeRenderOutPut[key] = result
				// }

				treeRenderOutPut[key] = template.HTML(result)
				errors = append(errors, render_errors...)
			} else {
				treeRenderOutPut[key] = value
			}
		} else {
			if value, ok := config.Values[key].(string); ok {
				treeRenderOutPut[key] = value
			}
		}

	}

	renderedOutput, _errors := applyTemplate(templateContent, treeRenderOutPut, config)
	if _errors != nil {
		errors = append(errors, _errors...)
	}

	templatebuilder.WriteString(renderedOutput)

	return templatebuilder.String(), errors
}

// applyTemplate generates output based on the provided template and API data.
func applyTemplate(templateStr string, data map[string]interface{}, config TemplateConfig) (string, []error) {
	var errors []error

	// Preprocess the template string to ensure variables can be referenced without a leading dot
	templateStr = preprocessTemplate(templateStr)

	// Debug: Print the preprocessed template string
	//fmt.Printf("Debug: Preprocessed Template string: %s\n", templateStr)

	// Create a FuncMap with a custom function
	funcMap := template.FuncMap{
		"valueOrEmpty": func(value interface{}) string {
			if value == nil {
				return ""
			}
			return fmt.Sprintf("%v", value)
		},
	}

	// Parse the template string
	tmpl, err := template.New("template").Funcs(funcMap).Parse(templateStr)
	if err != nil {
		errors = append(errors, fmt.Errorf("error parsing template: %v", err))
		return "", errors
	}

	// Execute the template with the provided data
	var output bytes.Buffer
	err = tmpl.Execute(&output, data)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing template: %v", err))
		return "", errors
	}

	htmlContent := output.String()
	if config.Enclose != "" {
		htmlContent = shared.EncloseContent(config.Enclose, htmlContent)
	}

	// Return the rendered output
	return htmlContent, errors
}

// preprocessTemplate converts {{a}} to {{.a}} for all variable references without a leading dot
func preprocessTemplate(templateStr string) string {
	// This regex matches {{key}} where 'key' is one or more word characters and does not already have a leading dot
	var varRefRegex = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*\}\}`)
	return varRefRegex.ReplaceAllString(templateStr, `{{.$1}}`)
}

// flexibleDataWrapper is a encloseper to resolve both {{a}} and {{.a}}.
type flexibleDataWrapper struct {
	data map[string]interface{}
}

// Implement template's "Field by Name" resolution
func (fdw *flexibleDataWrapper) Lookup(field string) (interface{}, bool) {
	if val, found := fdw.data[field]; found {
		return val, true
	}
	return nil, false
}

// Implement the template execution interface
func (fdw *flexibleDataWrapper) Get(name string) interface{} {
	if val, found := fdw.data[name]; found {
		return val
	}
	return "" // Return empty string if not found
}

// replaceRemainingPlaceholders replaces any unreplaced placeholders with empty strings.
func replaceRemainingPlaceholders(template string) string {
	// This is a simple implementation. For more complex templates, consider using regex.
	start := strings.Index(template, "{{")
	for start != -1 {
		end := strings.Index(template[start:], "}}")
		if end == -1 {
			break
		}
		end += start
		placeholder := template[start : end+2]
		template = strings.Replace(template, placeholder, "", 1)
		start = strings.Index(template, "{{")
	}
	return template
}

// checkString checks if the input contains "{{" and "}}" but does not contain ".html" or ".tmpl"
func checkString(s string) bool {
	return strings.Contains(s, "{{") &&
		strings.Contains(s, "}}") &&
		!strings.Contains(s, ".html") &&
		!strings.Contains(s, ".tmpl")
}
