package component

import (
	"fmt"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/shared"
)

// TextConfig represents configuration for paragraph text.
type TextConfig struct {
	shared.Component `mapstructure:",squash"`
	Value            string `mapstructure:"value" validate:"required" description:"The paragraph content" example:"{!{text-value.hyperbricks}}"`
}

// TextConfigGetName returns the HyperBricks type associated with the TextConfig.
func TextConfigGetName() string {
	return "<TEXT>"
}

// TextRenderer handles rendering of paragraph text content.
type TextRenderer struct{}

// Ensure TextRenderer implements shared.ComponentRenderer
var _ shared.ComponentRenderer = (*TextRenderer)(nil)

func (r *TextRenderer) Types() []string {
	return []string{
		TextConfigGetName(),
	}
}

// Validate ensures that the paragraph content is not empty.
func (tc *TextConfig) Validate() []error {
	var errors []error

	if tc.Value == "" {
		errors = append(errors, fmt.Errorf("missing value property or empty"))
	}

	return errors
}

// Render processes paragraph text and outputs it, applying enclosing if specified.
func (tr *TextRenderer) Render(instance interface{}) (string, []error) {
	var errors []error
	var builder strings.Builder

	config, ok := instance.(TextConfig)
	if !ok {
		errors = append(errors, fmt.Errorf("invalid type for TextRenderer"))
		return "", errors
	}
	// appending validation errors
	errors = append(errors, config.Validate()...)

	// Apply enclosing if specified
	textHTML := config.Value
	if config.Enclose != "" {
		textHTML = shared.EncloseContent(config.Enclose, textHTML)
	}

	builder.WriteString(textHTML)

	return builder.String(), errors
}
