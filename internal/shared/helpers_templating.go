package shared

import (
	"bytes"
	"fmt"
	"text/template"
)

// applyTemplate generates output based on the provided template and API data.
func ApplyTemplate(templateStr string, data map[string]interface{}) (string, []error) {
	var errors []error

	// Parse the template string
	tmpl, err := template.New("apiTemplate").Parse(templateStr)
	if err != nil {
		errors = append(errors, ComponentError{
			Err:      fmt.Errorf("error parsing template: %v", err).Error(),
			Rejected: false,
		})
		// Handle parsing error gracefully
		return fmt.Sprintf("Error parsing template: %v", err), errors
	}

	// Execute the template with the provided data
	var output bytes.Buffer
	err = tmpl.Execute(&output, data)
	if err != nil {
		// Handle execution error gracefully
		errors = append(errors, ComponentError{

			Err:      fmt.Errorf("error executing template: %v", err).Error(),
			Rejected: false,
		})
		return fmt.Sprintf("error executing template: %v", err), errors
	}

	// Return the rendered output
	return output.String(), errors
}
