package spaces

import (
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/markdown"
	"go.yaml.in/yaml/v4"
)

// Paths are literal map keys, never expressions or array indexes.
func editablePath(raw interface{}) ([]string, error) {
	values, ok := raw.([]interface{})
	if !ok || len(values) == 0 {
		return nil, fmt.Errorf("path must be a nonempty list of string keys")
	}
	result := make([]string, len(values))
	for i, v := range values {
		key, ok := v.(string)
		if !ok || key == "" || strings.ContainsRune(key, 0) {
			return nil, fmt.Errorf("path must contain nonempty string keys")
		}
		result[i] = key
	}
	return result, nil
}

func fieldPointer(parts []string) string {
	var b strings.Builder
	for _, p := range parts {
		b.WriteByte('/')
		b.WriteString(strings.ReplaceAll(strings.ReplaceAll(p, "~", "~0"), "/", "~1"))
	}
	return b.String()
}

func structuralProperty(key string) bool {
	return strings.HasPrefix(key, "@") || key == "type" || key == "inherit" || key == "editable" || key == "imports" || key == "vars"
}

// Find the actual component owning a target, including when a template's path
// crosses into a mounted native component. Aliases cannot bypass native rules.
func configureFieldTarget(root map[string]interface{}, f *Field) error {
	current := root
	owner := root
	ownerDepth := 0
	for i, key := range f.Path {
		if structuralProperty(key) {
			return fmt.Errorf("cannot edit structural property %q", key)
		}
		if _, ok := current["@type"]; ok {
			owner, ownerDepth = current, i
		}
		value, exists := current[key]
		if i == len(f.Path)-1 {
			if !exists && f.strictTarget {
				return fmt.Errorf("target property is not declared")
			}
			break
		}
		next, ok := value.(map[string]interface{})
		// Legacy template fields may introduce a previously absent values map.
		if !ok && !exists && !f.strictTarget && owner["@type"] == "<TEMPLATE>" && key == "values" && i == len(f.Path)-2 {
			return nil
		}
		if !ok {
			return fmt.Errorf("path crosses missing or non-object property %q", key)
		}
		current = next
	}
	relative := f.Path[ownerDepth:]
	switch owner["@type"] {
	case "<TEMPLATE>":
		if len(relative) < 2 || relative[0] != "values" {
			return fmt.Errorf("template fields must target values")
		}
	case "<MARKDOWN>":
		if len(relative) != 1 {
			return fmt.Errorf("markdown fields must target content or file")
		}
		key := relative[0]
		if err := validateMarkdownField(owner, key, *f); err != nil {
			return err
		}
		f.markdownFile = key == "file"
		f.markdownMaxBytes = markdown.DefaultMaxBytes
		if raw, exists := owner["max_bytes"]; exists {
			f.markdownMaxBytes, _ = strconv.ParseInt(str(raw), 10, 64)
		}
	case "<IMAGE>":
		if len(relative) != 1 {
			return fmt.Errorf("image fields must target src, alt, or title")
		}
		if err := validateImageSettings(owner); err != nil {
			return err
		}
		f.strictTarget = true
		if _, exists := owner[relative[0]]; !exists {
			return fmt.Errorf("image property must be declared in the source")
		}
		switch relative[0] {
		case "src":
			if f.Type != "asset" || (f.Directory == nil && f.Upload == nil) || fieldDirectory(*f).Base != "resources" || f.Edit != nil {
				return fmt.Errorf("image src requires an asset with an explicit resources directory and no document edit policy")
			}
			if f.Upload != nil {
				for _, ext := range f.Upload.Accept {
					if !nativeImageExtension(ext) {
						return fmt.Errorf("image src uploads accept only .jpg, .jpeg, .png, and .gif")
					}
				}
			}
			f.imageSource = true
		case "alt", "title":
			if f.Type != "text" && f.Type != "textarea" {
				return fmt.Errorf("image alt and title require text or textarea")
			}
		default:
			return fmt.Errorf("unsupported image property %q; supported properties are src, alt, and title", relative[0])
		}
	default:
		return fmt.Errorf("editable target belongs to unsupported component %v", owner["@type"])
	}
	return nil
}

func validateImageSettings(node map[string]interface{}) error {
	for _, key := range []string{"width", "height", "quality"} {
		if raw, exists := node[key]; exists {
			n, err := strconv.ParseInt(str(raw), 10, 32)
			if err != nil || n < 0 || (key == "quality" && n > 100) {
				return fmt.Errorf("invalid image %s: dimensions must be non-negative integers and quality must be an integer from 0 to 100", key)
			}
		}
	}
	if raw, exists := node["is_static"]; exists && raw != false {
		return fmt.Errorf("editable image sources require normal native image processing")
	}
	return nil
}

func nativeImageExtension(ext string) bool {
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif"
}

func fieldValue(root map[string]interface{}, f Field, dirs map[string]string) (string, error) {
	v := getMap(root, f.Path)
	if f.imageSource {
		source, ok := v.(string)
		if !ok || source == "" || !filepath.IsAbs(source) {
			return "", fmt.Errorf("image src must resolve to an absolute path inside resources; use a resources path resolver")
		}
		base := dirs["resources"]
		if base == "" {
			return "", fmt.Errorf("image src editing requires the module resources directory")
		}
		absoluteBase, err := filepath.Abs(base)
		if err != nil {
			return "", err
		}
		ref, err := filepath.Rel(absoluteBase, source)
		if err != nil {
			return "", err
		}
		ref = filepath.ToSlash(ref)
		if err := validateImageReference(f, ref); err != nil {
			return "", err
		}
		if _, err := contained(absoluteBase, ref); err != nil {
			return "", err
		}
		return ref, nil
	}
	switch v.(type) {
	case map[string]interface{}, []interface{}:
		return "", fmt.Errorf("editable field must contain a scalar reference or text")
	}
	if f.strictTarget {
		if _, ok := v.(string); !ok {
			return "", fmt.Errorf("this control requires a declared string property; numeric and boolean editing are not supported")
		}
	}
	return str(v), nil
}

func validateImageReference(f Field, ref string) error {
	if ref == "" || path.Clean(ref) != ref || strings.HasPrefix(ref, "/") || ref == ".." || strings.HasPrefix(ref, "../") || strings.ContainsAny(ref, "\\\x00") {
		return fmt.Errorf("image src requires a clean resources-relative reference")
	}
	d := fieldDirectory(f)
	if d.Path != "" && !strings.HasPrefix(ref, d.Path+"/") {
		return fmt.Errorf("image src is outside the field's resources directory")
	}
	if !nativeImageExtension(strings.ToLower(path.Ext(ref))) {
		return fmt.Errorf("image src requires a JPEG, PNG, or GIF original")
	}
	return nil
}

// The editor holds a portable resource reference; YAML holds the parser's native
// resolver. Never write the materialized filesystem path or a working-dir path.
func fieldNode(f Field, value string) *yaml.Node {
	if !f.imageSource {
		return scalar(value)
	}
	resolver, location := mapping(), mapping()
	put(location, "base", scalar("resources"))
	put(location, "path", scalar(value))
	put(resolver, "path", location)
	return resolver
}
