package yamlparser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/parser"
)

type valueResolverContext struct {
	opts          Options
	vars          map[string]interface{}
	resolvedVars  map[string]interface{}
	resolvingVars map[string]bool
	diagnostics   []Diagnostic
}

func newValueResolverContext(doc *Document, opts Options) *valueResolverContext {
	vars := make(map[string]interface{})
	for key, value := range opts.Variables {
		vars[key] = value
	}
	if doc != nil {
		for key, value := range doc.Vars {
			vars[key] = cloneValue(value)
		}
	}
	return &valueResolverContext{
		opts:          opts,
		vars:          vars,
		resolvedVars:  make(map[string]interface{}),
		resolvingVars: make(map[string]bool),
	}
}

func materializeNodeToMap(node *Node, ctx *valueResolverContext, path string) map[string]interface{} {
	out := make(map[string]interface{}, len(node.Props)+len(node.Children)+2)
	if typ := formatType(node.Type); typ != "" {
		out["@type"] = typ
	}
	for key, value := range node.Props {
		propPath := joinPath(path, key)
		if key == "template" {
			if resolved, ok := resolveTemplateField(value, ctx, propPath); ok {
				out[key] = resolved
				continue
			}
		}
		out[key] = materializeValueWithResolver(value, ctx, propPath)
	}
	if len(node.Children) > 0 {
		order := make([]string, 0, len(node.Children))
		for _, child := range node.Children {
			if strings.TrimSpace(child.Name) == "" {
				continue
			}
			childPath := joinPath(path, child.Name)
			order = append(order, child.Name)
			out[child.Name] = materializeNodeToMap(child, ctx, childPath)
		}
		if len(order) > 0 {
			out[orderKey] = order
		}
	}
	return out
}

func materializeValueWithResolver(value interface{}, ctx *valueResolverContext, path string) interface{} {
	if ctx == nil {
		return materializeValue(value)
	}
	switch typed := value.(type) {
	case *Node:
		return materializeNodeToMap(typed, ctx, path)
	case map[string]interface{}:
		if resolved, ok := resolveValueMap(typed, ctx, path); ok {
			return resolved
		}
		out := make(map[string]interface{}, len(typed))
		for key, value := range typed {
			if key == "template" {
				if resolved, ok := resolveTemplateField(value, ctx, joinPath(path, key)); ok {
					out[key] = resolved
					continue
				}
			}
			out[key] = materializeValueWithResolver(value, ctx, joinPath(path, key))
		}
		return out
	case []interface{}:
		out := make([]interface{}, 0, len(typed))
		for index, value := range typed {
			out = append(out, materializeValueWithResolver(value, ctx, fmt.Sprintf("%s[%d]", path, index)))
		}
		return out
	default:
		return typed
	}
}

func resolveValueMap(value map[string]interface{}, ctx *valueResolverContext, path string) (interface{}, bool) {
	if ctx == nil {
		return nil, false
	}
	if isFormatResolver(value) {
		return ctx.resolveFormat(value, path), true
	}
	if len(value) != 1 {
		return nil, false
	}
	for key, raw := range value {
		switch key {
		case "var":
			return ctx.resolveVar(raw, path), true
		case "env":
			return ctx.resolveEnv(raw, path), true
		case "config":
			return ctx.resolveConfig(raw, path), true
		case "path":
			return ctx.resolvePath(raw, path), true
		case "file":
			return ctx.resolveFile(raw, path), true
		}
	}
	return nil, false
}

func isFormatResolver(value map[string]interface{}) bool {
	if _, ok := value["format"]; !ok {
		return false
	}
	for key := range value {
		if key != "format" && key != "args" {
			return false
		}
	}
	return true
}

func resolveTemplateField(value interface{}, ctx *valueResolverContext, path string) (interface{}, bool) {
	if ctx == nil {
		return nil, false
	}
	valueMap, ok := value.(map[string]interface{})
	if !ok || len(valueMap) != 1 {
		return nil, false
	}
	rawFile, ok := valueMap["file"]
	if !ok {
		return nil, false
	}
	templateName := strings.TrimSpace(ctx.resolveRelativePathSpec(rawFile, path))
	if templateName == "" {
		ctx.addDiagnostic("warning", "template_file_empty", path, "template.file resolved to an empty template name")
		return "", true
	}
	if ctx.opts.TemplateDir == "" {
		return templateName, true
	}
	content, err := os.ReadFile(filepath.Join(ctx.opts.TemplateDir, templateName))
	if err != nil {
		ctx.addDiagnostic("warning", "template_file_missing", path, fmt.Sprintf("template.file %q could not be read: %v", templateName, err))
		return templateName, true
	}
	parser.AddTemplate(templateName, string(content))
	return templateName, true
}

func (ctx *valueResolverContext) resolveVar(raw interface{}, path string) interface{} {
	name, defaultValue, hasDefault := resolverNameDefault(raw, "name")
	if name == "" {
		ctx.addDiagnostic("warning", "var_empty", path, "var resolver has no name")
		return ""
	}
	value, ok := ctx.lookupVar(name, path)
	if !ok {
		if hasDefault {
			return materializeValueWithResolver(defaultValue, ctx, joinPath(path, "default"))
		}
		ctx.addDiagnostic("warning", "var_missing", path, fmt.Sprintf("missing var %q; resolved as empty string", name))
		return ""
	}
	return value
}

func (ctx *valueResolverContext) lookupVar(name string, path string) (interface{}, bool) {
	if value, ok := ctx.resolvedVars[name]; ok {
		return value, true
	}
	if ctx.resolvingVars[name] {
		ctx.addDiagnostic("warning", "var_cycle", path, fmt.Sprintf("cyclic var reference %q; resolved as empty string", name))
		return "", true
	}
	raw, ok := lookupDotted(ctx.vars, strings.Split(name, "."))
	if !ok {
		return nil, false
	}
	ctx.resolvingVars[name] = true
	value := materializeValueWithResolver(raw, ctx, "vars."+name)
	delete(ctx.resolvingVars, name)
	ctx.resolvedVars[name] = value
	return value, true
}

func (ctx *valueResolverContext) resolveEnv(raw interface{}, path string) interface{} {
	name, defaultValue, hasDefault := resolverNameDefault(raw, "name")
	required := resolverBool(raw, "required")
	if name == "" {
		ctx.addDiagnostic("warning", "env_empty", path, "env resolver has no name")
		return ""
	}
	if value, ok := ctx.opts.Env[name]; ok {
		return value
	}
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	if hasDefault {
		return materializeValueWithResolver(defaultValue, ctx, joinPath(path, "default"))
	}
	level := "warning"
	if required {
		level = "error"
	}
	ctx.addDiagnostic(level, "env_missing", path, fmt.Sprintf("missing env %q; resolved as empty string", name))
	return ""
}

func (ctx *valueResolverContext) resolveConfig(raw interface{}, path string) interface{} {
	name, defaultValue, hasDefault := resolverNameDefault(raw, "path")
	if name == "" {
		ctx.addDiagnostic("warning", "config_empty", path, "config resolver has no path")
		return ""
	}
	value, ok := lookupConfig(ctx.opts.Config, strings.Split(name, "."))
	if !ok {
		if hasDefault {
			return materializeValueWithResolver(defaultValue, ctx, joinPath(path, "default"))
		}
		ctx.addDiagnostic("warning", "config_missing", path, fmt.Sprintf("missing config %q; resolved as empty string", name))
		return ""
	}
	return materializeValueWithResolver(value, ctx, path)
}

func (ctx *valueResolverContext) resolveFormat(value map[string]interface{}, path string) interface{} {
	format := fmt.Sprint(materializeValueWithResolver(value["format"], ctx, joinPath(path, "format")))
	var args []interface{}
	if rawArgs, ok := value["args"]; ok {
		rawList, ok := rawArgs.([]interface{})
		if !ok {
			ctx.addDiagnostic("warning", "format_args_invalid", path, "format args must be a sequence")
			return format
		}
		args = make([]interface{}, 0, len(rawList))
		for index, arg := range rawList {
			args = append(args, materializeValueWithResolver(arg, ctx, fmt.Sprintf("%s.args[%d]", path, index)))
		}
	}
	return fmt.Sprintf(format, args...)
}

func (ctx *valueResolverContext) resolvePath(raw interface{}, path string) interface{} {
	resolved, ok := ctx.resolvePathSpec(raw, path)
	if !ok {
		ctx.addDiagnostic("warning", "path_invalid", path, "path resolver must be a string or mapping")
		return ""
	}
	return resolved
}

func (ctx *valueResolverContext) resolveFile(raw interface{}, path string) interface{} {
	filePath, ok := ctx.resolvePathSpec(raw, path)
	if !ok || strings.TrimSpace(filePath) == "" {
		ctx.addDiagnostic("warning", "file_invalid", path, "file resolver must resolve to a file path")
		return ""
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		ctx.addDiagnostic("warning", "file_missing", path, fmt.Sprintf("file %q could not be read: %v", filePath, err))
		return ""
	}
	return string(content)
}

func (ctx *valueResolverContext) resolvePathSpec(raw interface{}, path string) (string, bool) {
	switch typed := raw.(type) {
	case string:
		return typed, true
	case map[string]interface{}:
		baseName, _ := stringField(typed, "base")
		base := ctx.pathBase(baseName, joinPath(path, "base"))
		relative := ctx.resolveRelativePathSpec(typed, path)
		if base == "" {
			return relative, true
		}
		if relative == "" {
			return base, true
		}
		return filepath.Join(base, relative), true
	default:
		return "", false
	}
}

func (ctx *valueResolverContext) resolveRelativePathSpec(raw interface{}, path string) string {
	switch typed := raw.(type) {
	case string:
		return typed
	case map[string]interface{}:
		if path, ok := stringField(typed, "path"); ok {
			return path
		}
		if rawParts, ok := typed["parts"].([]interface{}); ok {
			parts := make([]string, 0, len(rawParts))
			for index, part := range rawParts {
				parts = append(parts, fmt.Sprint(materializeValueWithResolver(part, ctx, fmt.Sprintf("%s.parts[%d]", path, index))))
			}
			return filepath.Join(parts...)
		}
	}
	return ""
}

func (ctx *valueResolverContext) pathBase(name string, path string) string {
	switch strings.TrimSpace(name) {
	case "":
		return ""
	case "module_root":
		return ctx.opts.Paths.ModuleRoot
	case "root":
		return ctx.opts.Paths.Root
	case "module":
		return ctx.opts.Paths.Module
	case "resources":
		return ctx.opts.Paths.Resources
	case "templates":
		return ctx.opts.Paths.Templates
	case "static":
		return ctx.opts.Paths.Static
	case "hyperbricks":
		return ctx.opts.Paths.HyperBricks
	case "render":
		return ctx.opts.Paths.Render
	default:
		ctx.addDiagnostic("warning", "path_base_unknown", path, fmt.Sprintf("unknown path base %q", name))
		return ""
	}
}

func resolverNameDefault(raw interface{}, nameKey string) (string, interface{}, bool) {
	switch typed := raw.(type) {
	case string:
		return strings.TrimSpace(typed), nil, false
	case map[string]interface{}:
		name, _ := stringField(typed, nameKey)
		if name == "" && nameKey != "name" {
			name, _ = stringField(typed, "name")
		}
		defaultValue, hasDefault := typed["default"]
		return strings.TrimSpace(name), defaultValue, hasDefault
	default:
		return "", nil, false
	}
}

func resolverBool(raw interface{}, key string) bool {
	typed, ok := raw.(map[string]interface{})
	if !ok {
		return false
	}
	value, ok := typed[key]
	if !ok {
		return false
	}
	return strings.EqualFold(fmt.Sprint(value), "true")
}

func stringField(values map[string]interface{}, key string) (string, bool) {
	value, ok := values[key]
	if !ok {
		return "", false
	}
	return fmt.Sprint(value), true
}

func lookupDotted(values map[string]interface{}, parts []string) (interface{}, bool) {
	if len(parts) == 0 {
		return nil, false
	}
	current := interface{}(values)
	for _, part := range parts {
		currentMap, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = currentMap[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func (ctx *valueResolverContext) addDiagnostic(level string, code string, path string, message string) {
	ctx.diagnostics = append(ctx.diagnostics, Diagnostic{
		Level:   level,
		Code:    code,
		Message: message,
		Path:    path,
	})
}
