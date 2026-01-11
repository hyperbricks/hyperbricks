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
    MetaDocDescription string `mapstructure:"@doc" description:"Link js or render script tags from a js file or use inline attribute for multiline js blocks." example:"{!{javascript-@doc.hyperbricks}}"`
    Inline             string `mapstructure:"inline" description:"Use inline to define JavaScript in a multiline block <<[ /* JavaScript goes here */ ]>>" example:"{!{javascript-inline.hyperbricks}}"`
    Link               string `mapstructure:"link" description:"Use link for a script tag with a src attribute" example:"{!{javascript-link.hyperbricks}}"`
    File               string `mapstructure:"file" description:"File overrides link and inline, it loads contents of a file and renders it in a script tag." example:"{!{javascript-file.hyperbricks}}"`
}

// Backward-compatible aliases
type JavaScriptConfig = JSConfig

func JSConfigGetName() string { return "<JS>" }
// Legacy name retained for compatibility in other parts of the codebase
func JavaScriptConfigGetName() string { return "<JAVASCRIPT>" }

type JSRenderer struct{}

// Backward-compatible alias
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
        JavaScriptConfigGetName(), // support legacy type name
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

