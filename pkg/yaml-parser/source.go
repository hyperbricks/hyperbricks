package yamlparser

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v4"
)

type SourceError struct {
	File         string
	Line, Column int
	Err          error
}

func (err *SourceError) Error() string { return err.Err.Error() }
func (err *SourceError) Unwrap() error { return err.Err }

func sourceError(file string, err error) error {
	var existing *SourceError
	if errors.As(err, &existing) {
		copy := *existing
		copy.Err = err
		if copy.File == "" {
			copy.File = file
		}
		return &copy
	}
	result := &SourceError{File: file, Err: err}
	// The YAML library's positional error types are internal, not exported.
	if match := yamlErrorPosition.FindStringSubmatch(err.Error()); match != nil {
		result.Line, _ = strconv.Atoi(match[1])
		result.Column, _ = strconv.Atoi(match[2])
	}
	return result
}

var yamlErrorPosition = regexp.MustCompile(`line ([0-9]+)(?:(?::|, column )([0-9]+))?`)

func nodeSourceError(line, column int, message string) error {
	return &SourceError{Line: line, Column: column, Err: fmt.Errorf("line %d:%d: %s", line, column, message)}
}

func collectSourcePositions(target map[string]sourcePosition, path string, node *yaml.Node) {
	target[path] = sourcePosition{line: node.Line, column: node.Column}
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			collectSourcePositions(target, joinPath(path, node.Content[i].Value), node.Content[i+1])
		}
	}
}

func setNodeSource(node *Node, file string) {
	if node == nil {
		return
	}
	node.Source = file
	for key, position := range node.positions {
		position.file = file
		node.positions[key] = position
	}
	for _, child := range node.Children {
		setNodeSource(child, file)
	}
	var visit func(interface{})
	visit = func(value interface{}) {
		switch value := value.(type) {
		case *Node:
			setNodeSource(value, file)
		case map[string]interface{}:
			for _, child := range value {
				visit(child)
			}
		case []interface{}:
			for _, child := range value {
				visit(child)
			}
		}
	}
	visit(node.Props)
}

func sourceFilename(file, module string) string {
	if file == "" {
		return ""
	}
	if module != "" {
		absolute, err := filepath.Abs(file)
		base, baseErr := filepath.Abs(module)
		if err == nil && baseErr == nil {
			if relative, err := filepath.Rel(base, absolute); err == nil {
				return filepath.ToSlash(relative)
			}
		}
	}
	return filepath.ToSlash(filepath.Clean(file))
}

func (ctx *valueResolverContext) sourceMetadata(node *Node, path, prefix string) map[string]interface{} {
	file, line, column := node.Source, node.Line, node.Column
	if position, ok := node.positions[prefix]; ok {
		file, line, column = position.file, position.line, position.column
	}
	fields := make(map[string]interface{})
	for key, position := range node.positions {
		if prefix != "" {
			if !strings.HasPrefix(key, prefix+".") {
				continue
			}
			key = strings.TrimPrefix(key, prefix+".")
		}
		fields[key] = map[string]interface{}{"file": sourceFilename(position.file, ctx.opts.Paths.Module), "line": position.line, "column": position.column}
	}
	resources := make(map[string]interface{})
	for _, key := range []string{"template", "script", "file", "entry"} {
		if file := ctx.resources[joinPath(path, key)]; file != "" {
			resources[key] = sourceFilename(file, ctx.opts.Paths.Module)
		}
	}
	return map[string]interface{}{
		"file": sourceFilename(file, ctx.opts.Paths.Module), "line": line, "column": column,
		"path": path, "key": path[strings.LastIndex(path, ".")+1:],
		"fields": fields, "resources": resources,
	}
}

func (ctx *valueResolverContext) attachSourceMetadata(node *Node, out map[string]interface{}, path string) {
	if !ctx.opts.IncludeSourceMetadata {
		return
	}
	if formatType(node.Type) == "<MARKDOWN>" {
		if file, ok := out["file"].(string); ok && file != "" {
			if !filepath.IsAbs(file) {
				file = filepath.Join(ctx.opts.Paths.Resources, file)
			}
			ctx.resources[joinPath(path, "file")] = file
		}
	}
	out["@source"] = ctx.sourceMetadata(node, path, "")
	out["hyperbricksfile"] = sourceFilename(node.Source, ctx.opts.Paths.Module)
	out["hyperbrickspath"] = path
	out["hyperbrickskey"] = node.Name
	if options, ok := out["template"].(map[string]interface{}); ok && options["@type"] == nil {
		options["@source"] = ctx.sourceMetadata(node, joinPath(path, "template"), "template")
	}
}
