package component

import (
	"fmt"
	"os"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/shared"
)

// JavaScriptConfig represents the configuration for the JavaScript renderer.
type JavaScriptConfig struct {
	shared.Component `mapstructure:",squash"`
	Inline           string `mapstructure:"inline" description:"Use inline to define JavaScript in a multiline block <<[ /* JavaScript goes here */ ]>>" example:"{!{javascript-inline.hyperbricks}}"`
	Link             string `mapstructure:"link" description:"Use link for a script tag with a src attribute" example:"{!{javascript-link.hyperbricks}}"`
	File             string `mapstructure:"file" description:"File overrides link and inline, it loads contents of a file and renders it in a script tag." example:"{!{javascript-file.hyperbricks}}"`
}

// JavaScriptConfigGetName returns the HyperBricks type associated with the JavaScriptConfig.
func JavaScriptConfigGetName() string {
	return "<JAVASCRIPT>"
}

// JavaScriptRenderer handles rendering of JavaScript content from a file, inline multiline, or link.
type JavaScriptRenderer struct{}

// Ensure JavaScriptRenderer implements shared.ComponentRenderer
var _ shared.ComponentRenderer = (*JavaScriptRenderer)(nil)

// Validate ensures the JavaScript file exists and is readable.
func (js *JavaScriptConfig) Validate() []error {
	var errors []error

	if js.File != "" {
		if _, err := os.Stat(js.File); os.IsNotExist(err) {
			errors = append(errors, fmt.Errorf("file %s does not exist", js.File))
		} else {
			// Read the JavaScript file content
			content, err := os.ReadFile(js.File)
			if err != nil {
				errors = append(errors, fmt.Errorf("failed to read file %s: %w", js.File, err))
			} else {
				js.Inline = string(content)
			}
		}
	}

	return errors
}

func (r *JavaScriptRenderer) Types() []string {
	return []string{
		JavaScriptConfigGetName(),
	}
}

// Render reads the JavaScript content and outputs it wrapped in <script> tags with appropriate attributes.
func (jsr *JavaScriptRenderer) Render(instance interface{}) (string, []error) {
	var errors []error
	var builder strings.Builder

	config, ok := instance.(JavaScriptConfig)
	if !ok {
		errors = append(errors, fmt.Errorf("invalid type for JavaScriptRenderer"))
		return "", errors
	}

	// Append validation errors
	errors = append(errors, config.Validate()...)

	var scriptHTML string
	if config.Link != "" {
		// Render a <script> tag with the src attribute
		scriptHTML = fmt.Sprintf(`<script src="%s"></script>`, config.Link)
	} else {
		// Define allowed attributes for the <script> tag
		allowedAttributes := []string{"async", "defer", "type", "id", "class", "data-role", "data-action", "nonce", "integrity", "crossorigin"}

		// Render extra attributes
		extraAttributes := shared.RenderAllowedAttributes(config.ExtraAttributes, allowedAttributes)

		// Wrap the content in <script> tags with extra attributes
		scriptHTML = fmt.Sprintf("<script%s>\n%s\n</script>", extraAttributes, config.Inline)

		// Apply wrapping if specified
		if config.Enclose != "" {
			scriptHTML = fmt.Sprintf("\n%s\n", string(config.Inline))
			scriptHTML = shared.EncloseContent(config.Enclose, scriptHTML)
		}

	}

	builder.WriteString(scriptHTML)
	return builder.String(), errors
}
