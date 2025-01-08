package component

import (
	"fmt"
	"os"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/shared"
)

type StyleConfig struct {
	shared.Component `mapstructure:",squash"`
	File             string `mapstructure:"file" validate:"required" description:"Path to the CSS file" example:"{!{styles-file.hyperbricks}}"`
}

func StyleConfigGetName() string {
	return "<STYLES>"
}

type StyleRenderer struct{}

var _ shared.ComponentRenderer = (*StyleRenderer)(nil)

func (r *StyleRenderer) Types() []string {
	return []string{
		StyleConfigGetName(),
	}
}

func (style *StyleConfig) Validate() []error {
	var errors []error

	if style.File == "" {
		errors = append(errors, fmt.Errorf("missing file property or empty"))
		return errors
	}

	if _, err := os.Stat(style.File); os.IsNotExist(err) {
		errors = append(errors, fmt.Errorf("file %s does not exist", style.File))
	}

	return errors
}

func (sr *StyleRenderer) Render(instance interface{}) (string, []error) {
	var errors []error
	var builder strings.Builder

	config, ok := instance.(StyleConfig)
	if !ok {
		errors = append(errors, fmt.Errorf("invalid type for StyleRenderer"))
		return "", errors
	}

	errors = append(errors, config.Validate()...)

	content, err := os.ReadFile(config.File)
	if err != nil {
		errors = append(errors, fmt.Errorf("failed to read file %s: %w", config.File, err))
		return builder.String(), errors
	}

	allowedAttributes := []string{"media", "nonce", "type", "id", "class", "data-role", "data-action", "scoped"}

	extraAttributes := shared.RenderAllowedAttributes(config.ExtraAttributes, allowedAttributes)

	styleHTML := fmt.Sprintf("<style%s>\n%s\n</style>", extraAttributes, string(content))

	if config.Enclose != "" {
		styleHTML = shared.EncloseContent(config.Enclose, styleHTML)
	}

	builder.WriteString(styleHTML)

	return builder.String(), errors
}
