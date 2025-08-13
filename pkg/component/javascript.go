package component

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type JavaScriptConfig struct {
	shared.Component   `mapstructure:",squash"`
	MetaDocDescription string `mapstructure:"@doc" description:"Link js or render script tags from a js file or use inline attribute for multiline js blocks." example:"{!{javascript-@doc.hyperbricks}}"`
	Inline             string `mapstructure:"inline" description:"Use inline to define JavaScript in a multiline block <<[ /* JavaScript goes here */ ]>>" example:"{!{javascript-inline.hyperbricks}}"`
	Link               string `mapstructure:"link" description:"Use link for a script tag with a src attribute" example:"{!{javascript-link.hyperbricks}}"`
	File               string `mapstructure:"file" description:"File overrides link and inline, it loads contents of a file and renders it in a script tag." example:"{!{javascript-file.hyperbricks}}"`
}

func JavaScriptConfigGetName() string {
	return "<JAVASCRIPT>"
}

type JavaScriptRenderer struct{}

var _ shared.ComponentRenderer = (*JavaScriptRenderer)(nil)

func (js *JavaScriptConfig) Validate() []error {
	var errors []error

	if js.File != "" {
		if _, err := os.Stat(js.File); os.IsNotExist(err) {
			errors = append(errors, fmt.Errorf("file %s does not exist", js.File))
		} else {

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

func (jsr *JavaScriptRenderer) Render(instance interface{}, ctx context.Context) (string, []error) {
	var errors []error
	var builder strings.Builder

	config, ok := instance.(JavaScriptConfig)
	if !ok {
		errors = append(errors, fmt.Errorf("invalid type for JavaScriptRenderer"))
		return "", errors
	}

	errors = append(errors, config.Validate()...)

	var scriptHTML string
	if config.Link != "" {

		scriptHTML = fmt.Sprintf(`<script src="%s"></script>`, config.Link)
	} else {

		allowedAttributes := []string{"async", "defer", "type", "id", "class", "data-role", "data-action", "nonce", "integrity", "crossorigin"}

		extraAttributes := shared.RenderAllowedAttributes(config.ExtraAttributes, allowedAttributes)

		scriptHTML = fmt.Sprintf("<script%s>\n%s\n</script>", extraAttributes, config.Inline)

		if config.Enclose != "" {
			scriptHTML = fmt.Sprintf("\n%s\n", string(config.Inline))
			scriptHTML = shared.EncloseContent(config.Enclose, scriptHTML)
		}

	}

	builder.WriteString(scriptHTML)
	return builder.String(), errors
}
