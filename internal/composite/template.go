package composite

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"os"
	"strings"
	"sync"

	"github.com/mitchellh/mapstructure"

	"github.com/hyperbricks/hyperbricks/internal/renderer"
	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

// TemplateConfig represents the configuration for a TEMPLATE type.
type TemplateConfig struct {
	shared.Composite   `mapstructure:",squash"`
	MetaDocDescription string `mapstructure:"@doc" description:"TEMPLATE description" example:"{!{template-@doc.hyperbricks}}"`
	Template           string `mapstructure:"template" description:"Loads contents of a template file in the modules template directory" example:"{!{template-template.hyperbricks}}"`
	Inline             string `mapstructure:"inline" description:"Use inline to define the template in a multiline block <<[ /* TEmplate goes here */ ]>>" example:"{!{template-inline.hyperbricks}}"`

	Values  map[string]interface{} `mapstructure:"values" description:"Key-value pairs for template rendering" example:"{!{template-values.hyperbricks}}"`
	Enclose string                 `mapstructure:"enclose" description:"Enclosing property for the template rendered output" example:"{!{template-enclose.hyperbricks}}"`
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

func (tr *TemplateRenderer) Render(instance interface{}, ctx context.Context) (string, []error) {
	var templatebuilder strings.Builder
	var errors []error

	// Decode the instance into TemplateConfig without type assertion
	var config TemplateConfig
	err := mapstructure.Decode(instance, &config)
	if err != nil {
		return "", append(errors, shared.ComponentError{
			Hash: shared.GenerateHash(),
			File: config.Composite.Meta.HyperBricksFile,
			Path: config.Composite.Meta.HyperBricksPath,
			Key:  config.Composite.Meta.HyperBricksKey,
			Type: "<TEMPLATE>",
			Err:  fmt.Errorf("failed to decode instance into HeadConfig: %w", err).Error(),
		})
	}
	// appending validation errors
	errors = append(errors, config.Validate()...)

	var templateContent string

	if config.Inline != "" {
		templateContent = config.Inline
	} else {
		// Fetch the template content
		tc, found := tr.TemplateProvider(config.Template)
		if found {
			templateContent = tc
		} else {
			logging.GetLogger().Errorf("precached template '%s' not found, use {{TEMPLATE:sometemplate.tmpl}} for precaching", config.Template)
			// MARKER_FOR_CODE:
			// Attempt to load the file from disk and cache it.
			fileContent, err := GetTemplateFileContent(config.Template)
			if err != nil {
				errors = append(errors, shared.ComponentError{
					Hash: shared.GenerateHash(),
					File: config.Composite.Meta.HyperBricksFile,
					Path: config.Composite.Meta.HyperBricksPath,
					Key:  config.Composite.Meta.HyperBricksKey,
					Type: "<TEMPLATE>",
					Err:  fmt.Errorf("failed to load template file '%s'|%v", config.Template, err).Error(),
				})
			} else {
				templateContent = fileContent
			}
		}
	}

	// Retrieve sorted keys using the utility function
	sortedKeys := shared.SortedUniqueKeys(config.Values)
	var treeRenderOutPut = make(map[string]interface{})
	// for _, key := range sortedKeys {
	// 	if value, ok := config.Values[key].(map[string]interface{}); ok {
	// 		if componentType, ok := value["@type"].(string); ok {
	// 			result, render_errors := tr.RenderManager.Render(componentType, value, ctx)
	// 			treeRenderOutPut[key] = template.HTML(result)
	// 			errors = append(errors, render_errors...)
	// 		} else {
	// 			errors = append(errors, shared.ComponentError{
	// 				Hash:     shared.GenerateHash(),
	// 				File:     config.Composite.Meta.HyperBricksFile,
	// 				Path:     config.Composite.Meta.HyperBricksPath + ".values",
	// 				Key:      key,
	// 				Type:     "<TEMPLATE>",
	// 				Err:      "no type defined at replacement marker '" + key + "' in template values",
	// 				Rejected: true,
	// 			})
	// 			treeRenderOutPut[key] = template.HTML("<!-- no type defined: " + fmt.Sprintf("%s", value) + "-->")
	// 		}
	// 	} else {
	// 		if value, ok := config.Values[key].(string); ok {
	// 			treeRenderOutPut[key] = value
	// 		}
	// 	}
	// }

	for _, key := range sortedKeys {
		switch value := config.Values[key].(type) {
		case map[string]interface{}:
			// Check if "@type" exists and is a string.
			if componentType, ok := value["@type"].(string); ok {
				result, renderErrors := tr.RenderManager.Render(componentType, value, ctx)
				treeRenderOutPut[key] = template.HTML(result)
				errors = append(errors, renderErrors...)
			} else {
				errors = append(errors, shared.ComponentError{
					Hash:     shared.GenerateHash(),
					File:     config.Composite.Meta.HyperBricksFile,
					Path:     config.Composite.Meta.HyperBricksPath + ".values",
					Key:      key,
					Type:     "<TEMPLATE>",
					Err:      "no type defined at replacement marker '" + key + "' in template values",
					Rejected: true,
				})
				treeRenderOutPut[key] = template.HTML("<!-- no type defined: " + fmt.Sprintf("%s", value) + "-->")
			}
		case string:
			treeRenderOutPut[key] = value
		case []interface{}:
			// Convert to a slice of strings
			strSlice := make([]string, 0, len(value))
			for _, elem := range value {
				if s, ok := elem.(string); ok {
					strSlice = append(strSlice, s)
				}
			}
			treeRenderOutPut[key] = strSlice // Make sure it stays a slice!
		default:
			// Optionally handle unexpected types
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

	// removed this, it is dangerous because it breaks other template functionality
	// templateStr = preprocessTemplate(templateStr)

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

// preprocessTemplate converts {{a}} to {{.a}} for variable references
// while preserving reserved template keywords.
// func preprocessTemplate(templateStr string) string {
// 	// Define reserved keywords that should not be prefixed with a dot.
// 	reserved := map[string]bool{
// 		"range": true,
// 		"end":   true,
// 		"if":    true,
// 		"else":  true,
// 		"with":  true,
// 		// Add more reserved keywords as needed.
// 	}

// 	// This regex matches {{ key }} where 'key' is one or more alphanumeric or underscore characters.
// 	varRefRegex := regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*\}\}`)

// 	// Replace using a function so we can decide on each match.
// 	return varRefRegex.ReplaceAllStringFunc(templateStr, func(match string) string {
// 		submatches := varRefRegex.FindStringSubmatch(match)
// 		if len(submatches) > 1 {
// 			key := submatches[1]
// 			// If the key is reserved, return the match as-is.
// 			if reserved[key] {
// 				return match
// 			}
// 			// Otherwise, prepend a dot.
// 			return "{{." + key + "}}"
// 		}
// 		return match
// 	})
// }

// Global concurrent cache variables.
// Use sync.RWMutex for safe concurrent access.
var (
	templateCache = make(map[string]string)
	cacheMutex    sync.RWMutex
)

// getTemplateFileContent attempts to retrieve the template content from the cache.
// If not found, it reads the file from disk, caches it, and returns the content.
func GetTemplateFileContent(templatePath string) (string, error) {
	// First, check if the template content is already in the cache.
	cacheMutex.RLock()
	if content, exists := templateCache[templatePath]; exists {
		cacheMutex.RUnlock()
		return content, nil
	}
	cacheMutex.RUnlock()

	// Not in cache: attempt to read the file from disk.
	data, err := os.ReadFile(templatePath) // Uses os.ReadFile (Go 1.16+)
	if err != nil {
		return "", err
	}
	content := string(data)

	// Cache the content using a write lock.
	cacheMutex.Lock()
	templateCache[templatePath] = content
	cacheMutex.Unlock()

	return content, nil
}

// // flexibleDataWrapper is a encloseper to resolve both {{a}} and {{.a}}.
// type flexibleDataWrapper struct {
// 	data map[string]interface{}
// }

// // Implement template's "Field by Name" resolution
// func (fdw *flexibleDataWrapper) Lookup(field string) (interface{}, bool) {
// 	if val, found := fdw.data[field]; found {
// 		return val, true
// 	}
// 	return nil, false
// }

// // Implement the template execution interface
// func (fdw *flexibleDataWrapper) Get(name string) interface{} {
// 	if val, found := fdw.data[name]; found {
// 		return val
// 	}
// 	return "" // Return empty string if not found
// }

// // replaceRemainingPlaceholders replaces any unreplaced placeholders with empty strings.
// func replaceRemainingPlaceholders(template string) string {
// 	// This is a simple implementation. For more complex templates, consider using regex.
// 	start := strings.Index(template, "{{")
// 	for start != -1 {
// 		end := strings.Index(template[start:], "}}")
// 		if end == -1 {
// 			break
// 		}
// 		end += start
// 		placeholder := template[start : end+2]
// 		template = strings.Replace(template, placeholder, "", 1)
// 		start = strings.Index(template, "{{")
// 	}
// 	return template
// }

// // checkString checks if the input contains "{{" and "}}" but does not contain ".html" or ".tmpl"
//  func checkString(s string) bool {
// 	return strings.Contains(s, "{{") &&
// 		strings.Contains(s, "}}") &&
// 		!strings.Contains(s, ".html") &&
// 		!strings.Contains(s, ".tmpl")
// }
