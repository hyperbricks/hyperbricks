package component

import (
	"fmt"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/shared"
)

// HTMLConfig represents configuration for raw HTML.
type HTMLConfig struct {
	shared.Component `mapstructure:",squash"`
	Value            string `mapstructure:"value" validate:"required" description:"The raw HTML content" example:"{!{html-value.hyperbricks}}"`
	TrimSpace        bool   `mapstructure:"trimspace"  description:"The raw HTML content" example:"{!{html-value.hyperbricks}}"`
}

// HTMLConfigGetName returns the HyperBricks type associated with the HTMLConfig.
func HTMLConfigGetName() string {
	return "<HTML>"
}

// HTMLRenderer handles rendering of raw HTML content.
type HTMLRenderer struct{}

// Ensure HTMLRenderer implements shared.ComponentRenderer
var _ shared.ComponentRenderer = (*HTMLRenderer)(nil)

func (r *HTMLRenderer) Types() []string {
	return []string{
		HTMLConfigGetName(),
	}
}

// Validate ensures that the HTML content is not empty.
func (hc *HTMLConfig) Validate() []error {
	// standard validation on struct metadata of APIConfig
	errors := shared.Validate(hc)

	return errors
}

// Render processes raw HTML data and outputs it, applying wrapping if specified.
func (hr *HTMLRenderer) Render(instance interface{}) (string, []error) {
	var errors []error
	var builder strings.Builder

	config, ok := instance.(HTMLConfig)
	if !ok {
		errors = append(errors, fmt.Errorf("invalid type for HTMLRenderer"))
		return "", errors
	}

	// appending validation errors
	errors = append(errors, config.Validate()...)

	// Apply wrapping if specified
	htmlContent := config.Value
	if config.Enclose != "" {
		htmlContent = shared.EncloseContent(config.Enclose, htmlContent)
	}

	if config.TrimSpace {
		htmlContent = strings.TrimSpace(htmlContent)
	}

	builder.WriteString(htmlContent)

	return builder.String(), errors
}
