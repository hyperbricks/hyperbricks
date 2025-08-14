package component

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type CssConfig struct {
    shared.Component   `mapstructure:",squash"`
    MetaDocDescription string `mapstructure:"@doc" description:"Link css or render style tags from a css file or use inline attribute for multiline css blocks." example:"{!{css-@doc.hyperbricks}}"`
    Inline             string `mapstructure:"inline" description:"Use inline to define css in a multiline block <<[ /* css goes here */ ]>>" example:"{!{css-inline.hyperbricks}}"`
    Link               string `mapstructure:"link" description:"Use link for a link tag" example:"{!{css-link.hyperbricks}}"`
    File               string `mapstructure:"file" description:"file overrides link and inline, it loads contents of a file and renders it in a style tag." example:"{!{css-file.hyperbricks}}"`
}

func CssConfigGetName() string {
	return "<CSS>"
}

type CssRenderer struct{}

var _ shared.ComponentRenderer = (*CssRenderer)(nil)

func (css *CssConfig) Validate() []error {
    var errors []error

    if css.File != "" {
        // Prefer a single read; this covers non-existence and read errors
        content, err := os.ReadFile(css.File)
        if err != nil {
            errors = append(errors, fmt.Errorf("failed to read file %s: %w", css.File, err))
        } else {
            css.Inline = string(content)
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
        errors = append(errors, fmt.Errorf("invalid type for CssRenderer"))
        return "", errors
    }

    errors = append(errors, config.Validate()...)
    cssHTML := ""

    // Determine precedence: file (inline loaded) > inline > link
    if config.Inline != "" || (config.File != "" && config.Inline != "") {
        // Render a <style> tag using inline content
        allowedAttributes := []string{"media", "nonce", "type", "id", "class", "data-role", "data-action", "scoped"}
        extraAttributes := shared.RenderAllowedAttributes(config.ExtraAttributes, allowedAttributes)
        cssHTML = fmt.Sprintf("<style%s>\n%s\n</style>", extraAttributes, config.Inline)
    } else if config.Link != "" {
        // Render a <link> tag with allowed attributes
        allowedLinkAttrs := []string{"media", "nonce", "type", "id", "class", "data-role", "data-action", "integrity", "crossorigin", "referrerpolicy", "title", "disabled"}
        extraAttributes := shared.RenderAllowedAttributes(config.ExtraAttributes, allowedLinkAttrs)
        cssHTML = fmt.Sprintf(`<link rel="stylesheet" href="%s"%s>`, config.Link, extraAttributes)
    }

    if config.Enclose != "" && cssHTML != "" {
        // For CSS, enclose is used to define a custom wrapping tag for the raw CSS content,
        // effectively replacing the default <style> tag when provided.
        // Therefore, extract the raw inline CSS and enclose that.
        // Only applies when rendering inline/file (i.e., when cssHTML is a style block).
        if config.Inline != "" {
            raw := fmt.Sprintf("\n%s\n", config.Inline)
            cssHTML = shared.EncloseContent(config.Enclose, raw)
        }
    }

    builder.WriteString(cssHTML)
    return builder.String(), errors
}
