package component

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type JSConfig struct {
	shared.Component   `mapstructure:",squash"`
	MetaDocDescription string `mapstructure:"@doc" description:"Link a JavaScript asset or render a script tag from file or inline source." example:"{!{javascript-@doc.hyperbricks.yaml}}"`
	Inline             string `mapstructure:"inline" description:"Inline JavaScript source. Use a normal YAML string, or a YAML block scalar when the source spans multiple lines." example:"{!{javascript-inline.hyperbricks.yaml}}"`
	Link               string `mapstructure:"link" description:"Use link for a script tag with a src attribute" example:"{!{javascript-link.hyperbricks.yaml}}"`
	File               string `mapstructure:"file" description:"File overrides link and inline, it loads contents of a file and renders it in a script tag." example:"{!{javascript-file.hyperbricks.yaml}}"`
}

// Compatibility alias.
type JavaScriptConfig = JSConfig

func JSConfigGetName() string { return "<JS>" }

// JavaScriptConfigGetName returns the explicit JavaScript component alias.
func JavaScriptConfigGetName() string { return "<JAVASCRIPT>" }

type JSRenderer struct{}

// Compatibility alias.
type JavaScriptRenderer = JSRenderer

var _ shared.ComponentRenderer = (*JSRenderer)(nil)

func (js *JSConfig) Validate() []error {
	var errors []error

	if js.File != "" {
		content, err := os.ReadFile(js.File)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to read file %s: %w", js.File, err))
		} else {
			js.Inline = string(content)
		}
	}

	return errors
}

func (r *JSRenderer) Types() []string {
	return []string{
		JSConfigGetName(),
		JavaScriptConfigGetName(),
	}
}

func (jsr *JSRenderer) Render(instance interface{}, ctx context.Context) (string, []error) {
	var errors []error
	var builder strings.Builder

	config, ok := instance.(JSConfig)
	if !ok {
		errors = append(errors, fmt.Errorf("invalid type for JSRenderer"))
		return "", errors
	}

	errors = append(errors, config.Validate()...)

	var scriptHTML string

	if config.Inline != "" { // inline or file-loaded content
		allowedAttributes := []string{"async", "defer", "type", "id", "class", "data-role", "data-action", "nonce", "integrity", "crossorigin"}
		extraAttributes := shared.RenderAllowedAttributes(config.ExtraAttributes, allowedAttributes)
		scriptHTML = fmt.Sprintf("<script%s>\n%s\n</script>", extraAttributes, config.Inline)

		if config.Enclose != "" {
			// Enclose raw inline JS to allow custom tag replacement
			raw := fmt.Sprintf("\n%s\n", config.Inline)
			scriptHTML = shared.EncloseContent(config.Enclose, raw)
		}
	} else if config.Link != "" { // link path
		allowedLinkAttrs := []string{"async", "defer", "type", "id", "class", "data-role", "data-action", "nonce", "integrity", "crossorigin", "referrerpolicy"}
		extraAttributes := shared.RenderAllowedAttributes(config.ExtraAttributes, allowedLinkAttrs)
		scriptHTML = fmt.Sprintf(`<script src="%s"%s></script>`, config.Link, extraAttributes)
	}

	builder.WriteString(scriptHTML)
	return builder.String(), errors
}
