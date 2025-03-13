package component

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"strings"

	"github.com/hyperbricks/hyperbricks/internal/composite"
	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

type LocalJSONConfig struct {
	shared.Component `mapstructure:",squash"`
	FilePath         string                 `mapstructure:"file" validate:"required" description:"Path to the local JSON file" example:"{!{json-file.hyperbricks}}"`
	Template         string                 `mapstructure:"template" description:"Loads contents of a template file in the modules template directory" example:"{!{json-template.hyperbricks}}"`
	Inline           string                 `mapstructure:"inline" description:"Use inline to define the template in a multiline block <<[ /* Template code goes here */ ]>>" example:"{!{json-inline.hyperbricks}}"`
	Values           map[string]interface{} `mapstructure:"values" description:"Key-value pairs for template rendering" example:"{!{json-values.hyperbricks}}"`
	Debug            bool                   `mapstructure:"debug" description:"Debug the response data" example:"{!{json-debug.hyperbricks}}"`
}

func LocalJSONConfigGetName() string {
	return "<JSON_RENDER>"
}

type LocalJSONRenderer struct {
	TemplateProvider func(templateName string) (string, bool)
}

var _ shared.ComponentRenderer = (*LocalJSONRenderer)(nil)

func (r *LocalJSONRenderer) Types() []string {
	return []string{
		LocalJSONConfigGetName(),
	}
}

func (config *LocalJSONConfig) Validate() []error {
	errors := shared.Validate(config)

	return errors
}

func (renderer *LocalJSONRenderer) Render(instance interface{}, ctx context.Context) (string, []error) {
	var errors []error
	var builder strings.Builder

	config, ok := instance.(LocalJSONConfig)
	if !ok {
		errors = append(errors, fmt.Errorf("invalid type for LocalJSONRenderer"))
		return "", errors
	}

	errors = append(errors, config.Validate()...)

	jsonData, err := readLocalJSON(config.FilePath)
	if err != nil {
		errors = append(errors, fmt.Errorf("failed to read local JSON file: %w", err))
		return builder.String(), errors
	}

	if config.Debug {
		jsonBytes, err := json.MarshalIndent(jsonData, "", "  ")
		if err != nil {
			fmt.Println("Error marshaling struct to JSON:", err)

		}
		builder.WriteString(fmt.Sprintf("<!-- JSON_RENDER.debug = true -->\n<!--  <![CDATA[ \n%s\n ]]> -->", string(jsonBytes)))
	}

	var templateContent string

	if config.Inline != "" {
		templateContent = config.Inline
	} else {
		// Fetch the template content
		tc, found := renderer.TemplateProvider(config.Template)
		if found {
			templateContent = tc
		} else {
			logging.GetLogger().Errorf("precached template '%s' not found, use {{TEMPLATE:sometemplate.tmpl}} for precaching", config.Template)
			// MARKER_FOR_CODE:
			// Attempt to load the file from disk and cache it.
			fileContent, err := composite.GetTemplateFileContent(config.Template)
			if err != nil {
				errors = append(errors, shared.ComponentError{
					Hash: shared.GenerateHash(),
					Key:  config.Component.Meta.HyperBricksKey,
					Path: config.Component.Meta.HyperBricksPath,
					File: config.Component.Meta.HyperBricksFile,
					Type: LocalJSONConfigGetName(),
					Err:  fmt.Errorf("failed to load template file '%s': %v", config.Template, err).Error(),
				})
			} else {
				templateContent = fileContent
			}
		}
	}

	renderedOutput, tmplErrors := applyJsonTemplate(templateContent, jsonData, config)
	if tmplErrors != nil {
		errors = append(errors, tmplErrors...)
	}

	if config.Enclose != "" {
		renderedOutput = shared.EncloseContent(config.Enclose, renderedOutput)
	}

	builder.WriteString(renderedOutput)

	return builder.String(), errors
}

func readLocalJSON(filePath string) (map[string]interface{}, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var jsonData map[string]interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return jsonData, nil
}

func applyJsonTemplate(templateStr string, data map[string]interface{}, config LocalJSONConfig) (string, []error) {

	var errors []error
	var output strings.Builder

	context := map[string]interface{}{
		"Data": data, // Ensure Data is explicitly typed as interface{}
	}

	// Merge config.Values into the root
	for k, v := range config.Values {
		context[k] = v
	}

	tmpl, err := shared.GenericTemplate().Parse(templateStr)
	if err != nil {
		errors = append(errors, shared.ComponentError{
			Hash: shared.GenerateHash(),
			Key:  config.Component.Meta.HyperBricksKey,
			Path: config.Component.Meta.HyperBricksPath,
			File: config.Component.Meta.HyperBricksFile,
			Type: LocalJSONConfigGetName(),
			Err:  fmt.Sprintf("Error parsing template: %v", err),
		})
	}

	err = tmpl.Execute(&output, context)
	if err != nil {
		errors = append(errors, shared.ComponentError{
			Hash: shared.GenerateHash(),
			Key:  config.Component.Meta.HyperBricksKey,
			Path: config.Component.Meta.HyperBricksPath,
			File: config.Component.Meta.HyperBricksFile,
			Type: LocalJSONConfigGetName(),
			Err:  fmt.Sprintf("Error executing template: %v", err),
		})
	}

	return output.String(), errors
}
