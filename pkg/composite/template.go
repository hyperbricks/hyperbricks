package composite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/mitchellh/mapstructure"

	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/renderer"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/shared/apiutil"
)

// TemplateOptions is the reusable template field set used by <TEMPLATE> and
// template-bearing composites.
type TemplateOptions struct {
	Template         string                 `mapstructure:"template" json:"template,omitempty" description:"Loads contents of a template file in the modules template directory" example:"{!{template-template.hyperbricks.yaml}}"`
	Inline           string                 `mapstructure:"inline" json:"inline,omitempty" description:"Inline Go template source. Use a normal YAML string, or a YAML block scalar when the source spans multiple lines." example:"{!{template-inline.hyperbricks.yaml}}"`
	AllowedQueryKeys []string               `mapstructure:"querykeys" json:"querykeys,omitempty" description:"Set allowed proxy query keys" example:"{!{template-querykeys.hyperbricks.yaml}}"`
	QueryParams      map[string]string      `mapstructure:"queryparams" json:"queryparams,omitempty" description:"Set proxy query keys in the configuration" example:"{!{template-queryparams.hyperbricks.yaml}}"`
	Values           map[string]interface{} `mapstructure:"values" json:"values,omitempty" description:"Key-value pairs for template rendering" example:"{!{template-values.hyperbricks.yaml}}"`
	Enclose          string                 `mapstructure:"enclose" json:"enclose,omitempty" description:"Enclosing property for the template rendered output" example:"{!{template-enclose.hyperbricks.yaml}}"`
}

// ToRenderMap converts typed template options back into the map shape expected
// by the existing <TEMPLATE> renderer path.
func (opts *TemplateOptions) ToRenderMap() map[string]interface{} {
	if opts == nil {
		return nil
	}

	out := make(map[string]interface{})
	if opts.Template != "" {
		out["template"] = opts.Template
	}
	if opts.Inline != "" {
		out["inline"] = opts.Inline
	}
	if opts.AllowedQueryKeys != nil {
		out["querykeys"] = append([]string(nil), opts.AllowedQueryKeys...)
	}
	if opts.QueryParams != nil {
		queryParams := make(map[string]string, len(opts.QueryParams))
		for key, value := range opts.QueryParams {
			queryParams[key] = value
		}
		out["queryparams"] = queryParams
	}
	if opts.Values != nil {
		out["values"] = shared.CloneMapDeep(opts.Values)
	}
	if opts.Enclose != "" {
		out["enclose"] = opts.Enclose
	}
	return out
}

// TemplateConfig represents the configuration for a TEMPLATE type.
type TemplateConfig struct {
	shared.Composite   `mapstructure:",squash"`
	MetaDocDescription string `mapstructure:"@doc" description:"Template-backed component that binds scalar values and value-mounted bricks into generated HTML." example:"{!{template-@doc.hyperbricks.yaml}}"`
	TemplateOptions    `mapstructure:",squash"`
}

// MarshalJSON preserves the historical top-level <TEMPLATE> JSON shape while
// allowing TemplateOptions to marshal as lowercase nested template maps.
func (config TemplateConfig) MarshalJSON() ([]byte, error) {
	type templateConfigJSON struct {
		shared.Composite
		MetaDocDescription string
		Template           string
		Inline             string
		AllowedQueryKeys   []string
		QueryParams        map[string]string
		Values             map[string]interface{}
		Enclose            string
	}

	return json.Marshal(templateConfigJSON{
		Composite:          config.Composite,
		MetaDocDescription: config.MetaDocDescription,
		Template:           config.TemplateOptions.Template,
		Inline:             config.TemplateOptions.Inline,
		AllowedQueryKeys:   config.TemplateOptions.AllowedQueryKeys,
		QueryParams:        config.TemplateOptions.QueryParams,
		Values:             config.TemplateOptions.Values,
		Enclose:            config.TemplateOptions.Enclose,
	})
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

	var config TemplateConfig
	switch typed := instance.(type) {
	case TemplateConfig:
		config = typed
	case *TemplateConfig:
		if typed != nil {
			config = *typed
		}
	default:
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
	}
	// appending validation errors
	errors = append(errors, config.Validate()...)

	var templateContent string

	if config.Inline != "" {
		templateContent = config.Inline
	} else if config.Template != "" {
		// Fetch the template content
		tc, found := tr.TemplateProvider(config.Template)
		if found {
			templateContent = tc
		} else {
			logging.GetLogger().Warnf("precached template '%s' not found, use {{TEMPLATE:sometemplate.tmpl}} for precaching", config.Template)
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
	} else {
		errors = append(errors, shared.ComponentError{
			Hash: shared.GenerateHash(),
			File: config.Composite.Meta.HyperBricksFile,
			Path: config.Composite.Meta.HyperBricksPath,
			Key:  config.Composite.Meta.HyperBricksKey,
			Type: "<TEMPLATE>",
			Err:  fmt.Errorf(".template or a .inline is required").Error(),
		})
	}

	// Attempt to get the params of current request from the context and add it to the template values...
	if ctx != nil {
		req, ok := ctx.Value(shared.Request).(*http.Request)
		if ok && req != nil && req.URL != nil {
			allowed := apiutil.DefaultQueryKeys
			if config.AllowedQueryKeys != nil {
				allowed = config.AllowedQueryKeys
			}
			filtered := FilterAllowedQueryParams(req, allowed)
			config.Values = shared.CloneMapDeep(config.Values)
			if config.Values == nil {
				config.Values = make(map[string]interface{}, 1)
			}
			params := make(map[string]interface{}, len(filtered))
			for key, values := range filtered {
				if len(values) == 1 {
					params[key] = values[0] // Store as a string if only one value
				} else {
					params[key] = append([]string(nil), values...) // Store as a []string otherwise
				}
			}
			config.Values["Params"] = params
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
	tmpl, err := shared.ParsedGenericTemplate(templateStr)
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
