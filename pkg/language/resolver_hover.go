package language

import (
	"strings"

	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
	"go.yaml.in/yaml/v4"
)

type sourceResolver struct {
	kind    string
	key     *yaml.Node
	value   *yaml.Node
	mapping *yaml.Node
}

// resolverHover explains source syntax without materializing values: an editor
// hover must not read environment secrets, file contents, or execute templates.
func (a *Analyzer) resolverHover(uri, text string, position Position) *Hover {
	document, err := decodeYAMLDocument(text)
	if err != nil {
		return nil
	}
	var result *Hover
	visitSourceResolvers(text, document, a.isPackageConfig(uri), func(resolver sourceResolver) {
		if result != nil {
			return
		}
		node := resolver.key
		label := resolver.kind
		if !definitionContainsPosition(text, node, position) {
			node = nil
			if resolver.value.Kind == yaml.MappingNode {
				for i := 0; i+1 < len(resolver.value.Content); i += 2 {
					key := resolver.value.Content[i]
					if definitionContainsPosition(text, key, position) && resolverOptionHelp(resolver.kind, key.Value) != "" {
						node, label = key, key.Value
						break
					}
				}
			}
			if resolver.kind == "format" {
				for i := 0; i+1 < len(resolver.mapping.Content); i += 2 {
					key := resolver.mapping.Content[i]
					if key.Value == "args" && definitionContainsPosition(text, key, position) {
						node, label = key, "args"
					}
				}
			}
		}
		if node == nil {
			return
		}
		description := resolverHelp(resolver.kind)
		if label != resolver.kind {
			description = resolverOptionHelp(resolver.kind, label)
			label = resolver.kind + "." + label
		}
		result = &Hover{Contents: MarkupContent{Kind: "markdown", Value: "**" + label + "** · value resolver\n\n" + description}, Range: rangePointer(definitionSourceRange(text, node))}
	})
	return result
}

func resolverHelp(kind string) string {
	switch kind {
	case "var":
		return "Reads a dotted name from source `vars` or runtime variables. Missing values use `default`, otherwise an empty string.\n\n`{var: page.title}` or `{var: {name: page.title, default: Untitled}}`. Cmd/Ctrl-click the name to open its source declaration."
	case "env":
		return "Reads an environment variable at runtime. Use `default` for a fallback or `required: true` to report a missing value as an error. Environment values are never shown in editor hover.\n\n`{env: {name: SITE_TITLE, default: Untitled}}`"
	case "config":
		return "Reads a dotted path from runtime configuration. Missing values use `default`, otherwise an empty string.\n\n`{config: myconf.site.title}` or `{config: {path: myconf.site.title, default: Untitled}}`. Cmd/Ctrl-click the path to open the selected package configuration."
	case "path":
		return "Joins a named directory `base` with `path` or ordered `parts`; produces a path string, not file contents. A scalar path is returned unchanged.\n\n`{path: {base: resources, path: docs/guide.md}}`"
	case "file":
		return "Reads file contents into this value. Use a named directory `base` with `path` or ordered `parts`. A missing file reports a warning and resolves to an empty string.\n\n`{file: {base: resources, path: docs/guide.md}}`. Cmd/Ctrl-click a static path to open the file."
	case "template.file":
		return "Preloads a Go template from the module's configured templates directory and keeps its relative name as the value. This also works for a value named `template` inside plugin data.\n\n`template: {file: cards/card.html}`. Cmd/Ctrl-click the name to open the template."
	case "format":
		return "Formats a string using Go `fmt.Sprintf`. `args` is an ordered sequence whose items can use other resolvers.\n\n`{format: '%s — %s', args: [{var: page.title}, {config: myconf.site.name}]}`"
	}
	return ""
}

func resolverOptionHelp(kind, key string) string {
	switch key {
	case "base":
		if kind == "path" || kind == "file" {
			return "Named directory: `module_root`, `root`, `module`, `resources`, `templates`, `static`, `hyperbricks`, or `render`. Module directory settings are respected."
		}
	case "path":
		if kind == "config" {
			return "Dotted runtime configuration path, for example `myconf.site.title`."
		}
		if kind == "path" || kind == "file" || kind == "template.file" {
			return "Relative path. If both `path` and `parts` are present, `path` takes precedence. Use `/` between directories."
		}
	case "parts":
		if kind == "path" || kind == "file" || kind == "template.file" {
			return "Ordered path segments, for example `[docs, guide.md]`. Segments can contain resolvers. Navigation is available only when every segment is a literal scalar."
		}
	case "name":
		if kind == "var" || kind == "env" {
			return "The variable name to look up. `var` accepts dotted source names; `env` reads an environment variable at runtime."
		}
	case "default":
		if kind == "var" || kind == "env" || kind == "config" {
			return "Fallback used when the requested value is missing. The fallback can itself contain a resolver."
		}
	case "required":
		if kind == "env" {
			return "When `true`, a missing environment variable without a default reports an error instead of a warning. The value still resolves to an empty string."
		}
	case "args":
		if kind == "format" {
			return "Ordered arguments for Go `fmt.Sprintf`. Each item is resolved before formatting."
		}
	}
	return ""
}

// visitSourceResolvers follows the parser's component/value boundary rather
// than treating component entries such as `- file:` as value resolvers. Parsing
// here is source-only; it does not resolve imports, variables, or resources.
func visitSourceResolvers(text string, document *yaml.Node, configuration bool, visit func(sourceResolver)) {
	components := make(map[[2]int]bool)
	if !configuration {
		if parsed, err := yamlparser.ParseBytesWithOptions([]byte(text), yamlparser.ParseOptions{AllowUnknownTypes: true, RecoverDuplicateChildren: true}); err == nil {
			var collectValue func(interface{})
			collectValue = func(value interface{}) {
				switch typed := value.(type) {
				case *yamlparser.Node:
					components[[2]int{typed.Line, typed.Column}] = true
					for _, child := range typed.Children {
						collectValue(child)
					}
					for _, property := range typed.Props {
						collectValue(property)
					}
				case map[string]interface{}:
					for _, child := range typed {
						collectValue(child)
					}
				case []interface{}:
					for _, child := range typed {
						collectValue(child)
					}
				}
			}
			for _, root := range parsed.Roots {
				collectValue(root)
			}
		}
	}
	var walk func(*yaml.Node, string, bool)
	walk = func(node *yaml.Node, parentKey string, allowResolver bool) {
		if node == nil {
			return
		}
		if node.Kind == yaml.SequenceNode && !configuration && (components[[2]int{node.Line, node.Column}] || isComponentSequence(node)) {
			for _, item := range node.Content {
				if item.Kind != yaml.MappingNode || len(item.Content) != 2 {
					continue
				}
				key := item.Content[0].Value
				if key != "type" && key != "inherit" {
					walk(item.Content[1], key, key != "editable")
				}
			}
			return
		}
		if node.Kind == yaml.MappingNode {
			if kind, key, value := sourceResolverKind(node, parentKey); allowResolver && kind != "" {
				visit(sourceResolver{kind: kind, key: key, value: value, mapping: node})
				switch kind {
				case "var", "env", "config":
					walk(definitionMappingValue(value, "default"), "default", true)
				case "file", "path", "template.file":
					if definitionMappingValue(value, "path") == nil {
						walk(definitionMappingValue(value, "parts"), "parts", true)
					}
				case "format":
					walk(value, "format", true)
					walk(definitionMappingValue(node, "args"), "args", true)
				}
				return
			}
			for i := 0; i+1 < len(node.Content); i += 2 {
				walk(node.Content[i+1], node.Content[i].Value, true)
			}
			return
		}
		for _, child := range node.Content {
			walk(child, parentKey, true)
		}
	}
	body := yamlDocumentBody(document)
	if body == nil || body.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(body.Content); i += 2 {
		key, value := body.Content[i].Value, body.Content[i+1]
		if !configuration && key == "imports" {
			continue
		}
		// The vars mapping names declarations; its keys are not directives.
		walk(value, key, key != "vars")
	}
}

func sourceResolverKind(node *yaml.Node, parentKey string) (string, *yaml.Node, *yaml.Node) {
	if node == nil || node.Kind != yaml.MappingNode {
		return "", nil, nil
	}
	if len(node.Content) == 2 {
		key, value := node.Content[0], node.Content[1]
		kind := strings.TrimSpace(key.Value)
		if parentKey == "template" && kind == "file" {
			return "template.file", key, value
		}
		switch kind {
		case "var", "env", "config", "file", "path", "format":
			return kind, key, value
		}
	}
	if len(node.Content) == 4 && definitionMappingValue(node, "args") != nil {
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == "format" {
				return "format", node.Content[i], node.Content[i+1]
			}
		}
	}
	return "", nil, nil
}
