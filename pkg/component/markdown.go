package component

import (
	"context"
	"fmt"
	"html"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/hyperbricks/hyperbricks/pkg/markdown"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

const DefaultMarkdownMaxBytes = markdown.DefaultMaxBytes
const MaxMarkdownBytes = markdown.MaxBytes

type MarkdownConfig struct {
	shared.Component   `mapstructure:",squash"`
	MetaDocDescription string      `mapstructure:"@doc" description:"Render declared Markdown content or a bounded resources-relative file as sanitized HTML. Routes and page layout belong to fragments, hypermedia, and templates."`
	Content            *string     `mapstructure:"content" description:"Inline Markdown text, including an explicitly empty string. Mutually exclusive with file. The file resolver can also supply content at load time."`
	File               string      `mapstructure:"file" description:"Clean resources-relative .md or .markdown reference, read at render time. No absolute paths, remote URLs, or query parameter selection."`
	Class              string      `mapstructure:"class" description:"Optional escaped CSS class for a wrapping div. Empty emits only Markdown HTML."`
	MaxBytes           int64       `mapstructure:"max_bytes" description:"Maximum input bytes. Default 1048576 (1 MiB); must be between 1 and 20971520 when specified."`
	Editable           interface{} `mapstructure:"editable" json:"-" description:"Source-owned Spaces metadata for this component's content or file field. Not rendered."`
}

type MarkdownRenderer struct{ ResourcesRoot string }

var _ shared.ComponentRenderer = (*MarkdownRenderer)(nil)

func MarkdownConfigGetName() string         { return "<MARKDOWN>" }
func (r *MarkdownRenderer) Types() []string { return []string{MarkdownConfigGetName()} }
func NewMarkdownRenderer(resources string) *MarkdownRenderer {
	return &MarkdownRenderer{ResourcesRoot: resources}
}

func (config MarkdownConfig) ValidateRawConfig(raw map[string]interface{}) error {
	_, hasContent := raw["content"]
	_, hasFile := raw["file"]
	if hasContent == hasFile {
		return fmt.Errorf("markdown requires exactly one of content or file")
	}
	for _, key := range []string{"content", "file"} {
		if value, exists := raw[key]; exists {
			if _, ok := value.(string); !ok {
				return fmt.Errorf("markdown.%s must be a string", key)
			}
		}
	}
	if value, exists := raw["max_bytes"]; exists {
		// Validate before weak decoding can truncate fractional values or treat null as default.
		var limit int64
		switch v := value.(type) {
		case string:
			var err error
			if limit, err = strconv.ParseInt(v, 10, 64); err != nil {
				return fmt.Errorf("markdown.max_bytes must be a positive integer")
			}
		case int:
			limit = int64(v)
		case int64:
			limit = v
		default:
			return fmt.Errorf("markdown.max_bytes must be a positive integer")
		}
		if limit < 1 || limit > MaxMarkdownBytes {
			return fmt.Errorf("markdown.max_bytes must be between 1 and %d", MaxMarkdownBytes)
		}
	}
	return nil
}

func (r *MarkdownRenderer) Render(instance interface{}, ctx context.Context) (string, []error) {
	config, ok := instance.(MarkdownConfig)
	if !ok {
		return "", []error{fmt.Errorf("invalid type for MarkdownRenderer: %T", instance)}
	}
	fail := func(err error) (string, []error) {
		diagnostic := shared.ResourceDiagnostic(err, config.Meta, "render", "file")
		diagnostic.Hash, diagnostic.Type, diagnostic.Rejected = shared.GenerateHash(), MarkdownConfigGetName(), true
		return "", []error{diagnostic}
	}
	if (config.Content != nil) == (config.File != "") {
		return fail(fmt.Errorf("markdown requires exactly one of content or file"))
	}
	limit := config.MaxBytes
	if limit == 0 {
		limit = DefaultMarkdownMaxBytes
	}
	if limit < 1 || limit > MaxMarkdownBytes {
		return fail(fmt.Errorf("markdown.max_bytes must be between 1 and %d", MaxMarkdownBytes))
	}
	if ctx != nil && ctx.Err() != nil {
		return fail(ctx.Err())
	}
	var content []byte
	if config.Content != nil {
		if int64(len(*config.Content)) > limit {
			return fail(fmt.Errorf("Markdown exceeds %d bytes", limit))
		}
		content = []byte(*config.Content)
	} else {
		var err error
		content, err = readMarkdownFile(r.ResourcesRoot, config.File, limit)
		if err != nil {
			return fail(err)
		}
	}
	if !utf8.Valid(content) || strings.ContainsRune(string(content), 0) {
		return fail(fmt.Errorf("Markdown must be UTF-8 without NUL characters"))
	}
	output := markdown.Render(content)
	if config.Class != "" {
		output = `<div class="` + html.EscapeString(config.Class) + `">` + output + "</div>\n"
	}
	return shared.EncloseContent(config.Enclose, output), nil
}

func readMarkdownFile(resources, ref string, limit int64) ([]byte, error) {
	if ref == "" || strings.ContainsAny(ref, "\\:\x00") || path.Clean(ref) != ref || !filepath.IsLocal(ref) || (path.Ext(ref) != ".md" && path.Ext(ref) != ".markdown") {
		return nil, fmt.Errorf("markdown.file must be a clean resources-relative .md or .markdown reference")
	}
	if resources == "" {
		return nil, fmt.Errorf("markdown.file requires a configured resources directory")
	}
	root, err := os.OpenRoot(resources)
	if err != nil {
		return nil, fmt.Errorf("cannot open Markdown resources directory")
	}
	defer root.Close()
	// Stat through the same root first to reject special files without blocking on a FIFO.
	info, err := root.Stat(filepath.FromSlash(ref))
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("Markdown file is unavailable or not a regular file")
	}
	f, err := root.Open(filepath.FromSlash(ref))
	if err != nil {
		return nil, fmt.Errorf("cannot open Markdown file")
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("Markdown file is not a regular file")
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("Markdown exceeds %d bytes", limit)
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, fmt.Errorf("cannot read Markdown file")
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("Markdown exceeds %d bytes", limit)
	}
	return b, nil
}
