package component

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/shared"
)

type CssConfig struct {
	shared.Component `mapstructure:",squash"`
	Inline           string `mapstructure:"inline" description:"Use inline to define css in a multiline block <<[ /* css goes here */ ]>>" example:"{!{css-inline.hyperbricks}}"`
	Link             string `mapstructure:"link" description:"Use link for a link tag" example:"{!{css-link.hyperbricks}}"`
	File             string `mapstructure:"file" description:"file overrides link and inline, it loads contents of a file and renders it in a style tag." example:"{!{css-file.hyperbricks}}"`
}

func CssConfigGetName() string {
	return "<CSS>"
}

type CssRenderer struct{}

var _ shared.ComponentRenderer = (*CssRenderer)(nil)

func (css *CssConfig) Validate() []error {
	var errors []error

	if css.File != "" {
		if _, err := os.Stat(css.File); os.IsNotExist(err) {
			errors = append(errors, fmt.Errorf("file %s does not exist", css.File))
		} else {

			content, err := os.ReadFile(css.File)
			if err != nil {

			} else {
				css.Inline = string(content)
			}
		}
	}

	return errors
}

func (r *CssRenderer) Types() []string {
	return []string{
		CssConfigGetName(),
	}
}

func (sr *CssRenderer) Render(instance interface{}, ctx context.Context) (string, []error) {
	var errors []error
	var builder strings.Builder

	config, ok := instance.(CssConfig)
	if !ok {
		errors = append(errors, fmt.Errorf("invalid type for StyleRenderer"))
		return "", errors
	}

	errors = append(errors, config.Validate()...)
	CssHtml := ""
	if config.Link != "" {

		CssHtml = fmt.Sprintf(`<link rel="stylesheet" href="%s">`, config.Link)
	} else {

		allowedAttributes := []string{"media", "nonce", "type", "id", "class", "data-role", "data-action", "scoped"}

		extraAttributes := shared.RenderAllowedAttributes(config.ExtraAttributes, allowedAttributes)

		CssHtml = fmt.Sprintf("<style%s>\n%s\n</style>", extraAttributes, string(config.Inline))

		if config.Enclose != "" {

			CssHtml = fmt.Sprintf("\n%s\n", string(config.Inline))
			CssHtml = shared.EncloseContent(config.Enclose, CssHtml)
		}
	}

	builder.WriteString(CssHtml)
	return builder.String(), errors
}
