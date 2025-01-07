package component

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"

	"strings"

	"github.com/hyperbricks/hyperbricks/internal/shared"
)

// LocalJSONConfig represents configuration for fetching and rendering data from a local JSON file.
type LocalJSONConfig struct {
	shared.Component `mapstructure:",squash"`
	FilePath         string `mapstructure:"file" validate:"required" description:"Path to the local JSON file" example:"{!{json-file.hyperbricks}}"`
	Template         string `mapstructure:"template" validate:"required" description:"Template for rendering output" example:"{!{json-template.hyperbricks}}"`
}

// LocalJSONConfigGetName returns the HyperBricks type associated with the LocalJSONConfig.
func LocalJSONConfigGetName() string {
	return "<JSON>"
}

// LocalJSONRenderer handles rendering of data from a local JSON file.
type LocalJSONRenderer struct {
	TemplateProvider func(templateName string) (string, bool) // Function to retrieve templates
}

// Ensure LocalJSONRenderer implements shared.ComponentRenderer
var _ shared.ComponentRenderer = (*LocalJSONRenderer)(nil)

func (r *LocalJSONRenderer) Types() []string {
	return []string{
		LocalJSONConfigGetName(),
	}
}

// Validate ensures the local JSON configuration is correct.
func (config *LocalJSONConfig) Validate() []error {
	errors := shared.Validate(config)

	return errors
}

// Render processes local JSON data and outputs it according to the specified template.
func (renderer *LocalJSONRenderer) Render(instance interface{}) (string, []error) {
	var errors []error
	var builder strings.Builder

	config, ok := instance.(LocalJSONConfig)
	if !ok {
		errors = append(errors, fmt.Errorf("invalid type for LocalJSONRenderer"))
		return "", errors
	}
	// appending validation errors
	errors = append(errors, config.Validate()...)

	// Read and parse the JSON file
	jsonData, err := readLocalJSON(config.FilePath)
	if err != nil {
		errors = append(errors, fmt.Errorf("failed to read local JSON file: %w", err))
		return builder.String(), errors
	}

	// Fetch the template content
	templateContent, found := renderer.TemplateProvider(config.Template)
	if !found {
		warnings := []error{fmt.Errorf("template '%s' not found", config.Template)}
		builder.WriteString(fmt.Sprintf("<!-- Template '%s' not found -->", config.Template))
		errors = append(errors, warnings...)
		return builder.String(), errors
	}

	// Apply the template
	renderedOutput, tmplErrors := applyJsonTemplate(templateContent, jsonData)
	if tmplErrors != nil {
		errors = append(errors, tmplErrors...)
	}

	// Apply wrapping if specified
	if config.Enclose != "" {
		renderedOutput = shared.EncloseContent(config.Enclose, renderedOutput)
	}

	builder.WriteString(renderedOutput)

	return builder.String(), errors
}

// readLocalJSON reads and parses a JSON file into a map[string]interface{}.
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

// applyTemplate generates output based on the provided template and JSON data.
func applyJsonTemplate(templateStr string, data map[string]interface{}) (string, []error) {
	var errors []error
	var output strings.Builder
	tmpl, err := template.New("localJSONTemplate").Parse(templateStr)
	if err != nil {
		errors = append(errors, shared.ComponentError{
			Err: fmt.Sprintf("Error parsing template: %v", err),
		})
	}

	err = tmpl.Execute(&output, data)
	if err != nil {
		errors = append(errors, shared.ComponentError{
			Err: fmt.Sprintf("Error executing template: %v", err),
		})
	}

	return output.String(), errors
}
