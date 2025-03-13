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

	for _, key := range sortedKeys {
		switch value := config.Values[key].(type) {
		case map[string]interface{}:
			// Check if "@type" exists and is a string.
			if componentType, ok := value["@type"].(string); ok {
				result, renderErrors := tr.RenderManager.Render(componentType, value, ctx)
				treeRenderOutPut[key] = template.HTML(result)
				errors = append(errors, renderErrors...)
			} else {
				treeRenderOutPut[key] = value
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

	htmlContent := templatebuilder.String()
	if config.Enclose != "" {
		htmlContent = shared.EncloseContent(config.Enclose, htmlContent)
	}

	return htmlContent, errors
}

// applyTemplate generates output based on the provided template and API data.
func applyTemplate(templateStr string, data map[string]interface{}, config TemplateConfig) (string, []error) {
	var errors []error

	// Parse the template string
	tmpl, err := shared.GenericTemplate.Parse(templateStr)
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

	// Return the rendered output
	return output.String(), errors
}

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
