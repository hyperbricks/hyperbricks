package component

import (
	"fmt"
	"os"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/shared"
)

// CssConfig represents the configuration for the style renderer.
type CssConfig struct {
	shared.Component `mapstructure:",squash"`
	Inline           string `mapstructure:"inline" description:"Use inline to define css in a multiline block <<[ /* css goes here */ ]>>" example:"{!{css-inline.hyperbricks}}"`
	Link             string `mapstructure:"link" description:"Use link for a link tag" example:"{!{css-link.hyperbricks}}"`
	File             string `mapstructure:"file" description:"file overrides link and inline, it loads contents of a file and renders it in a style tag." example:"{!{css-file.hyperbricks}}"`
}

// CssConfigGetName returns the HyperBricks type associated with the CssConfig.
func CssConfigGetName() string {
	return "<CSS>"
}

// CssRenderer handles rendering of CSS content from a file, inline multiline or link
type CssRenderer struct{}

// Ensure StyleRenderer implements shared.ComponentRenderer
var _ shared.ComponentRenderer = (*CssRenderer)(nil)

// Validate ensures the CSS file exists and is readable.
func (css *CssConfig) Validate() []error {
	var errors []error

	if css.File != "" {
		if _, err := os.Stat(css.File); os.IsNotExist(err) {
			errors = append(errors, fmt.Errorf("file %s does not exist", css.File))
		} else {
			// Read the CSS file content
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

// Render reads the CSS file content and outputs it encloseped in <style> tags with extra attributes.
func (sr *CssRenderer) Render(instance interface{}) (string, []error) {
	var errors []error
	var builder strings.Builder

	config, ok := instance.(CssConfig)
	if !ok {
		errors = append(errors, fmt.Errorf("invalid type for StyleRenderer"))
		return "", errors
	}

	// appending validation errors
	errors = append(errors, config.Validate()...)
	CssHtml := ""
	if config.Link != "" {
		// Wrap the content in <style> tags with extra attributes
		CssHtml = fmt.Sprintf(`<link rel="stylesheet" href="%s">`, config.Link)
	} else {
		// Define allowed attributes for the <style> tag
		allowedAttributes := []string{"media", "nonce", "type", "id", "class", "data-role", "data-action", "scoped"}

		// Render extra attributes
		extraAttributes := shared.RenderAllowedAttributes(config.ExtraAttributes, allowedAttributes)

		// Wrap the content in <style> tags with extra attributes
		CssHtml = fmt.Sprintf("<style%s>\n%s\n</style>", extraAttributes, string(config.Inline))

		// Apply enclosing if specified
		if config.Enclose != "" {
			// Wrap the content in <style> tags with extra attributes
			CssHtml = fmt.Sprintf("\n%s\n", string(config.Inline))
			CssHtml = shared.EncloseContent(config.Enclose, CssHtml)
		}
	}

	builder.WriteString(CssHtml)
	return builder.String(), errors
}
