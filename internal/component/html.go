package component

import (
	"context"
	"fmt"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/shared"
)

type HTMLConfig struct {
	shared.Component `mapstructure:",squash"`
	Value            string `mapstructure:"value" validate:"required" description:"The raw HTML content" example:"{!{html-value.hyperbricks}}"`
	TrimSpace        bool   `mapstructure:"trimspace"  description:"TrimSpace filters all leading and trailing white space removed, as defined by Unicode." example:"{!{html-trimspace.hyperbricks}}"`
}

func HTMLConfigGetName() string {
	return "<HTML>"
}

type HTMLRenderer struct{}

var _ shared.ComponentRenderer = (*HTMLRenderer)(nil)

func (r *HTMLRenderer) Types() []string {
	return []string{
		HTMLConfigGetName(),
	}
}

func (hc *HTMLConfig) Validate() []error {

	errors := shared.Validate(hc)

	return errors
}

func (hr *HTMLRenderer) Render(instance interface{}, ctx context.Context) (string, []error) {
	var errors []error
	var builder strings.Builder

	config, ok := instance.(HTMLConfig)
	if !ok {
		errors = append(errors, fmt.Errorf("invalid type for HTMLRenderer"))
		return "", errors
	}

	errors = append(errors, config.Validate()...)

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
