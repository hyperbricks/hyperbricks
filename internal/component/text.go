package component

import (
	"fmt"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/shared"
)

type TextConfig struct {
	shared.Component `mapstructure:",squash"`
	Value            string `mapstructure:"value" validate:"required" description:"The paragraph content" example:"{!{text-value.hyperbricks}}"`
}

func TextConfigGetName() string {
	return "<TEXT>"
}

type TextRenderer struct{}

var _ shared.ComponentRenderer = (*TextRenderer)(nil)

func (r *TextRenderer) Types() []string {
	return []string{
		TextConfigGetName(),
	}
}

func (tc *TextConfig) Validate() []error {
	var errors []error

	if tc.Value == "" {
		errors = append(errors, fmt.Errorf("missing value property or empty"))
	}

	return errors
}

func (tr *TextRenderer) Render(instance interface{}) (string, []error) {
	var errors []error
	var builder strings.Builder

	config, ok := instance.(TextConfig)
	if !ok {
		errors = append(errors, fmt.Errorf("invalid type for TextRenderer"))
		return "", errors
	}

	errors = append(errors, config.Validate()...)

	textHTML := config.Value
	if config.Enclose != "" {
		textHTML = shared.EncloseContent(config.Enclose, textHTML)
	}

	builder.WriteString(textHTML)

	return builder.String(), errors
}
