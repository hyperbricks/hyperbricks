// Package markdown provides the shared HTML rendering policy for Markdown content.
package markdown

import (
	"sync"

	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
)

const DefaultMaxBytes int64 = 1 << 20
const MaxBytes int64 = 20 << 20

// Build only when Markdown is used; the resulting policy is immutable and shared.
var policy = sync.OnceValue(bluemonday.UGCPolicy)

// Render skips raw HTML and sanitizes generated HTML, including link/image URLs.
// Page templates, CSS, HTTP responses, and file selection belong to callers.
func Render(content []byte) string {
	renderer := blackfriday.NewHTMLRenderer(blackfriday.HTMLRendererParameters{
		Flags: blackfriday.CommonHTMLFlags | blackfriday.SkipHTML | blackfriday.Safelink,
	})
	return string(policy().SanitizeBytes(blackfriday.Run(content, blackfriday.WithRenderer(renderer))))
}
