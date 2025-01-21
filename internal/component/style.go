package component

import (
	"fmt"
	"os"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/shared"
)

// StyleConfig represents the configuration for the style renderer.
type StyleConfig struct {
	shared.Component `mapstructure:",squash"`
	File             string `mapstructure:"file" validate:"required" description:"Path to the CSS file" example:"{!{styles-file.hyperbricks}}"`
}

// StyleConfigGetName returns the HyperBricks type associated with the StyleConfig.
func StyleConfigGetName() string {
	return "<STYLES>"
}

// StyleRenderer handles rendering of CSS content from a file.
type StyleRenderer struct{}

// Ensure StyleRenderer implements shared.ComponentRenderer
var _ shared.ComponentRenderer = (*StyleRenderer)(nil)

func (r *StyleRenderer) Types() []string {
	return []string{
		StyleConfigGetName(),
	}
}

// Validate ensures the CSS file exists and is readable.
func (style *StyleConfig) Validate() []error {
	var errors []error

	// Check if the file is specified
	if style.File == "" {
		errors = append(errors, fmt.Errorf("missing file property or empty"))
		return errors
	}

	// Check if the file exists
	if _, err := os.Stat(style.File); os.IsNotExist(err) {
		errors = append(errors, fmt.Errorf("file %s does not exist", style.File))
	}

	return errors
}

// Render reads the CSS file content and outputs it wrapped in <style> tags with extra attributes.
func (sr *StyleRenderer) Render(instance interface{}) (string, []error) {
	var errors []error
	var builder strings.Builder

	config, ok := instance.(StyleConfig)
	if !ok {
		errors = append(errors, fmt.Errorf("invalid type for StyleRenderer"))
		return "", errors
	}

	// appending validation errors
	errors = append(errors, config.Validate()...)

	// Read the CSS file content
	content, err := os.ReadFile(config.File)
	if err != nil {
		errors = append(errors, fmt.Errorf("failed to read file %s: %w", config.File, err))
		return builder.String(), errors
	}

	// Define allowed attributes for the <style> tag
	allowedAttributes := []string{"media", "nonce", "type", "id", "class", "data-role", "data-action", "scoped"}

	// Render extra attributes
	extraAttributes := shared.RenderAllowedAttributes(config.ExtraAttributes, allowedAttributes)

	// Wrap the content in <style> tags with extra attributes
	styleHTML := fmt.Sprintf("<style%s>\n%s\n</style>", extraAttributes, string(content))

	// Apply enclosing if specified
	if config.Enclose != "" {
		styleHTML = shared.EncloseContent(config.Enclose, styleHTML)
	}

	builder.WriteString(styleHTML)

	return builder.String(), errors
}
