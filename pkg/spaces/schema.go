package spaces

import (
	"fmt"
	"net/mail"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/hyperbricks/hyperbricks/pkg/markdown"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/mitchellh/mapstructure"
)

type Directory = shared.SpacesDirectory
type UploadPolicy = shared.SpacesUploadPolicy
type DocumentPolicy struct {
	Type     string `json:"type" mapstructure:"type"`
	MaxBytes int64  `json:"max_bytes" mapstructure:"max_bytes"`
}
type Field struct {
	ID               string          `json:"id"`
	Key              string          `json:"key"`
	Path             []string        `json:"path"`
	Type             string          `json:"type" mapstructure:"type"`
	Label            string          `json:"label" mapstructure:"label"`
	Help             string          `json:"help,omitempty" mapstructure:"help"`
	Placeholder      string          `json:"placeholder,omitempty" mapstructure:"placeholder"`
	Required         bool            `json:"required" mapstructure:"required"`
	Rows             int             `json:"rows,omitempty" mapstructure:"rows"`
	Max              int             `json:"max,omitempty" mapstructure:"max"`
	Upload           *UploadPolicy   `json:"upload,omitempty" mapstructure:"upload"`
	Directory        *Directory      `json:"directory,omitempty" mapstructure:"directory"`
	Edit             *DocumentPolicy `json:"edit,omitempty" mapstructure:"edit"`
	Order            int             `json:"order" mapstructure:"order"`
	Group            string          `json:"group,omitempty" mapstructure:"group"`
	Value            string          `json:"value"`
	Default          string          `json:"default"`
	markdownFile     bool
	markdownMaxBytes int64
}

// SourceFields derives and validates the editing contract of an effective source.
// It does not read or mutate source files or enable the editor.
func SourceFields(root map[string]interface{}) ([]Field, error) {
	return schemaFields(root)
}

func schemaFields(root map[string]interface{}) ([]Field, error) {
	fields := []Field{}
	var visit func(map[string]interface{}, []string) error
	visit = func(m map[string]interface{}, parts []string) error {
		if schema, ok := m["editable"]; ok {
			if m["@type"] != "<TEMPLATE>" && m["@type"] != "<MARKDOWN>" {
				return fmt.Errorf("editable belongs to a template or markdown component at %s", strings.Join(parts, "."))
			}
			definitions := map[string]interface{}{}
			switch v := schema.(type) {
			case []interface{}:
				for _, item := range v {
					key, ok := item.(string)
					if !ok || definitions[key] != nil {
						return fmt.Errorf("editable list needs unique field names")
					}
					definitions[key] = "text"
				}
			case map[string]interface{}:
				definitions = v
			default:
				return fmt.Errorf("editable must be a list or mapping")
			}
			keys := make([]string, 0, len(definitions))
			for k := range definitions {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if k == "" {
					return fmt.Errorf("editable key cannot be empty")
				}
				f := Field{Key: k, Label: k, Type: "text", Rows: 3}
				switch v := definitions[k].(type) {
				case string:
					f.Type = v
				case map[string]interface{}:
					allowed := map[string]bool{"type": true, "label": true, "help": true, "placeholder": true, "required": true, "rows": true, "max": true, "upload": true, "directory": true, "edit": true, "order": true, "group": true}
					for key := range v {
						if !allowed[key] {
							return fmt.Errorf("unknown editable metadata %s.%s", k, key)
						}
					}
					if err := mapstructure.WeakDecode(v, &f); err != nil {
						return err
					}
					if edit, ok := v["edit"]; ok {
						options, ok := edit.(map[string]interface{})
						if !ok {
							return fmt.Errorf("edit must be a mapping")
						}
						for key := range options {
							if key != "type" && key != "max_bytes" {
								return fmt.Errorf("unknown document edit option %s", key)
							}
						}
					}
				default:
					return fmt.Errorf("invalid schema for %s", k)
				}
				if f.Type != "text" && f.Type != "textarea" && f.Type != "email" && f.Type != "asset" {
					return fmt.Errorf("unsupported field type %q", f.Type)
				}
				if f.Rows < 1 || f.Rows > 40 || f.Max < 0 || f.Max > 100000 {
					return fmt.Errorf("invalid rows or max for %s", k)
				}
				if f.Order < -100000 || f.Order > 100000 || len(f.Group) > 100 || strings.ContainsAny(f.Group, "\r\n\x00") {
					return fmt.Errorf("invalid order or group for %s", k)
				}
				if f.Edit != nil && (f.Type != "asset" || f.Edit.Type != "markdown" || f.Edit.MaxBytes <= 0 || f.Edit.MaxBytes > 1<<20 || (f.Directory == nil && f.Upload == nil)) {
					return fmt.Errorf("edit requires an asset with an explicit directory, type markdown, and max_bytes between 1 and 1048576")
				}
				if f.Type != "asset" && (f.Upload != nil || f.Directory != nil) {
					return fmt.Errorf("upload/directory requires an asset field")
				}
				if f.Upload != nil {
					if err := validateUploadPolicy(f.Upload); err != nil {
						return fmt.Errorf("%s: %w", k, err)
					}
				}
				if f.Directory != nil {
					if err := validateDirectory(*f.Directory); err != nil {
						return err
					}
					if f.Upload != nil && *f.Directory != f.Upload.Directory {
						return fmt.Errorf("asset and upload directories must match")
					}
				}
				f.Path = append(append([]string{}, parts...), "values", k)
				if m["@type"] == "<MARKDOWN>" {
					if err := validateMarkdownField(m, k, f); err != nil {
						return fmt.Errorf("markdown editable.%s: %w", k, err)
					}
					f.Path = append(append([]string{}, parts...), k)
					f.markdownFile = k == "file"
					f.markdownMaxBytes = markdown.DefaultMaxBytes
					if raw, exists := m["max_bytes"]; exists {
						f.markdownMaxBytes, _ = strconv.ParseInt(str(raw), 10, 64)
					}
				}
				for _, p := range f.Path {
					f.ID += "/" + strings.ReplaceAll(strings.ReplaceAll(p, "~", "~0"), "/", "~1")
				}
				v := getMap(root, f.Path)
				if _, ok := v.(map[string]interface{}); ok {
					return fmt.Errorf("editable field %s must contain a scalar reference or text", k)
				}
				if _, ok := v.([]interface{}); ok {
					return fmt.Errorf("editable field %s must be scalar", k)
				}
				f.Value = str(v)
				f.Default = f.Value
				fields = append(fields, f)
			}
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			if k != "editable" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			if v, ok := m[k].(map[string]interface{}); ok {
				if err := visit(v, append(append([]string{}, parts...), k)); err != nil {
					return err
				}
			}
		}
		return nil
	}
	err := visit(root, nil)
	sort.SliceStable(fields, func(i, j int) bool {
		if fields[i].Order != fields[j].Order {
			return fields[i].Order < fields[j].Order
		}
		return fields[i].ID < fields[j].ID
	})
	return fields, err
}

func validateMarkdownField(node map[string]interface{}, key string, field Field) error {
	if key != "content" && key != "file" {
		return fmt.Errorf("only content and file can be edited")
	}
	if _, ok := node[key].(string); !ok {
		return fmt.Errorf("the source must declare this field as a string")
	}
	_, hasContent := node["content"]
	_, hasFile := node["file"]
	if hasContent == hasFile || (hasFile && str(node["file"]) == "") {
		return fmt.Errorf("the source must declare exactly one of content or file")
	}
	limit := markdown.DefaultMaxBytes
	if raw, exists := node["max_bytes"]; exists {
		var err error
		limit, err = strconv.ParseInt(str(raw), 10, 64)
		if err != nil || limit < 1 || limit > markdown.MaxBytes {
			return fmt.Errorf("invalid renderer max_bytes")
		}
	}
	if key == "content" {
		if field.Type != "text" && field.Type != "textarea" {
			return fmt.Errorf("content must use text or textarea metadata")
		}
		return nil
	}
	if field.Type != "asset" || (field.Upload == nil && field.Directory == nil) || fieldDirectory(field).Base != "resources" {
		return fmt.Errorf("file requires an asset with an explicit resources directory")
	}
	if field.Edit != nil && field.Edit.MaxBytes > limit {
		return fmt.Errorf("edit.max_bytes exceeds the renderer max_bytes")
	}
	if field.Upload != nil {
		if field.Upload.MaxBytes > limit {
			return fmt.Errorf("upload.max_bytes exceeds the renderer max_bytes")
		}
		for _, ext := range field.Upload.Accept {
			if ext != ".md" && ext != ".markdown" {
				return fmt.Errorf("file uploads accept only .md and .markdown")
			}
		}
	}
	return nil
}

func validateDirectory(d Directory) error {
	if d.Base != "static" && d.Base != "resources" {
		return fmt.Errorf("asset directory base must be static or resources")
	}
	if d.Path != "" && (path.Clean(d.Path) != d.Path || strings.HasPrefix(d.Path, "/") || d.Path == ".." || strings.HasPrefix(d.Path, "../") || strings.Contains(d.Path, "\\")) {
		return fmt.Errorf("asset directory must be a clean relative path")
	}
	return nil
}
func validateUploadPolicy(p *UploadPolicy) error {
	if err := validateDirectory(p.Directory); err != nil {
		return err
	}
	if p.MaxBytes <= 0 || p.MaxBytes > 20<<20 {
		return fmt.Errorf("upload max_bytes must be between 1 and 20971520")
	}
	if len(p.Accept) == 0 {
		return fmt.Errorf("upload accept must specify file extensions")
	}
	for _, ext := range p.Accept {
		if !supportedExtension(ext) {
			return fmt.Errorf("unsupported upload extension %q", ext)
		}
	}
	return nil
}
func supportedExtension(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".md", ".markdown":
		return true
	}
	return false
}
func fieldDirectory(f Field) Directory {
	if f.Upload != nil {
		return f.Upload.Directory
	}
	if f.Directory != nil {
		return *f.Directory
	}
	return Directory{Base: "static"}
}
func validateText(f Field, v string) error {
	if f.markdownFile && v == "" {
		return fmt.Errorf("%s requires a Markdown file", f.Label)
	}
	if !f.markdownFile && f.markdownMaxBytes > 0 && int64(len(v)) > f.markdownMaxBytes {
		return fmt.Errorf("%s exceeds the Markdown renderer byte limit", f.Label)
	}
	if !utf8.ValidString(v) || strings.ContainsRune(v, 0) {
		return fmt.Errorf("%s must be UTF-8 text without NUL", f.Label)
	}
	if f.Required && strings.TrimSpace(v) == "" {
		return fmt.Errorf("%s is required", f.Label)
	}
	if f.Max > 0 && utf8.RuneCountInString(v) > f.Max {
		return fmt.Errorf("%s exceeds %d characters", f.Label, f.Max)
	}
	if len(v) > 512<<10 {
		return fmt.Errorf("%s is too large", f.Label)
	}
	if (f.Type == "text" || f.Type == "email") && strings.ContainsAny(v, "\r\n") {
		return fmt.Errorf("%s must be single-line", f.Label)
	}
	if f.Type == "email" && v != "" {
		address, err := mail.ParseAddress(v)
		if err != nil || address.Address != v {
			return fmt.Errorf("%s must be an email address", f.Label)
		}
	}
	return nil
}

var namePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,79}$`)
var routePattern = regexp.MustCompile(`^[A-Za-z0-9_~.-]+(?:/[A-Za-z0-9_~.-]+)*$`)
var metaPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.:-]{0,127}$`)

func validateRoute(route string) error {
	if !routePattern.MatchString(route) || path.Clean(route) != route || len(route) > 240 {
		return fmt.Errorf("route must be a relative path of letters, digits, -, _, ., or ~; use index for /")
	}
	for _, part := range strings.Split(route, "/") {
		if part == ".." || part == "." {
			return fmt.Errorf("route cannot contain dot segments")
		}
	}
	first := strings.Split(route, "/")[0]
	if first == "static" || first == "out" || first == "__hyperbricks" {
		return fmt.Errorf("route uses a reserved runtime prefix")
	}
	return nil
}
func normalizedRoute(v string) string {
	v = strings.Trim(v, "/")
	if v == "index" {
		return ""
	}
	return v
}
func checkIdentity(c *catalog, name, route, except string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("name must start with a letter and contain up to 80 letters, digits, underscores or hyphens")
	}
	if err := validateRoute(route); err != nil {
		return err
	}
	for other, d := range c.defs {
		if other == except {
			continue
		}
		if other == name {
			return conflict("component name already exists")
		}
		if r, ok := d.effective["route"]; ok && normalizedRoute(str(r)) == normalizedRoute(route) {
			return conflict(fmt.Sprintf("route is already used by %s", other))
		}
	}
	return nil
}
func validateMeta(key string, v *string) error {
	if !metaPattern.MatchString(key) {
		return fmt.Errorf("invalid metadata key %q", key)
	}
	if v == nil {
		return nil
	}
	if !utf8.ValidString(*v) || len(*v) > 8192 || strings.ContainsRune(*v, 0) {
		return fmt.Errorf("invalid metadata value for %s", key)
	}
	if (key == "og:url" || key == "og:image") && *v != "" {
		u, err := url.Parse(*v)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
			return fmt.Errorf("%s requires an absolute HTTP(S) public URL", key)
		}
	}
	return nil
}
