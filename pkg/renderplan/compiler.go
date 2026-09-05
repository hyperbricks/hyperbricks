package renderplan

import (
	"fmt"
	htmltemplate "html/template"
	"slices"
	"strings"
	texttemplate "text/template"
	"text/template/parse"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/render"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/shared/apiutil"
	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
)

type templateProvider func(templateName string) (string, bool)

type compiler struct {
	manager          *render.RenderManager
	templateProvider templateProvider
	querySets        [][]string
}

// Compile prepares an eligible route without changing the raw configuration.
// Ineligible routes should continue through RenderManager.Render.
func Compile(
	manager *render.RenderManager,
	raw map[string]interface{},
	provider func(templateName string) (string, bool),
) (*Plan, error) {
	if manager == nil {
		return nil, fmt.Errorf("render plan compiler requires a render manager")
	}
	if raw == nil {
		return nil, notEligible("route configuration is nil")
	}
	rootType, _ := raw["@type"].(string)
	if rootType != composite.HyperMediaConfigGetName() {
		return nil, notEligible("only hypermedia routes are compiled in phase 1")
	}

	c := compiler{manager: manager, templateProvider: provider}
	root, err := c.compileHyperMedia(raw)
	if err != nil {
		return nil, err
	}
	return &Plan{root: root, querySets: c.querySets, needsAPIRequestContext: NeedsAPIRequestContext(raw)}, nil
}

func (c *compiler) compileNode(raw map[string]interface{}) (renderNode, error) {
	typeName, ok := raw["@type"].(string)
	if !ok || typeName == "" {
		return nil, notEligible("component has no valid @type")
	}

	switch typeName {
	case composite.HyperMediaConfigGetName():
		return c.compileHyperMedia(raw)
	case composite.TemplateConfigGetName():
		return c.compileTemplate(raw)
	case composite.TreeRendererConfigGetName():
		return c.compileTree(raw)
	case component.TextConfigGetName(), component.HTMLConfigGetName():
		return c.compileTyped(typeName, raw)
	case component.GojaRenderConfigGetName():
		return c.compileGoja(raw)
	case component.EsbuildConfigGetName():
		return c.compileEsbuild(raw)
	default:
		return nil, notEligible(fmt.Sprintf("component type %s uses legacy rendering", typeName))
	}
}

func (c *compiler) compileHyperMedia(raw map[string]interface{}) (renderNode, error) {
	response, err := c.manager.MakeInstance(typeRequest(composite.HyperMediaConfigGetName(), raw))
	if err != nil {
		return nil, err
	}
	config, ok := response.Instance.(composite.HyperMediaConfig)
	if !ok {
		return nil, fmt.Errorf("compiled hypermedia has type %T", response.Instance)
	}
	if config.Template == nil {
		return nil, notEligible("phase 1 requires template-backed hypermedia")
	}
	if config.Head != nil || config.Title != "" || config.Favicon != "" {
		return nil, notEligible("template-backed hypermedia with head fields remains legacy")
	}

	// Recreate the nested template from the decoded options, just like the
	// legacy HYPERMEDIA renderer. This preserves weak decoding and the legacy
	// normalization of a missing or typed-nil values map.
	templateRaw := config.Template.ToRenderMap()
	templateRaw["hyperbricksfile"] = config.Composite.Meta.HyperBricksFile
	templateRaw["hyperbrickspath"] = config.Composite.Meta.HyperBricksKey + ".template"
	if _, ok := templateRaw["values"].(map[string]interface{}); !ok {
		templateRaw["values"] = make(map[string]interface{})
	}

	templateNode, err := c.compileTemplate(templateRaw)
	if err != nil {
		return nil, err
	}
	return &hyperMediaNode{
		config:   config,
		template: templateNode,
		warnings: append([]string(nil), response.Warnings...),
	}, nil
}

func (c *compiler) compileTemplate(raw map[string]interface{}) (renderNode, error) {
	response, err := c.manager.MakeInstance(typeRequest(composite.TemplateConfigGetName(), raw))
	if err != nil {
		return nil, err
	}
	config, ok := response.Instance.(composite.TemplateConfig)
	if !ok {
		return nil, fmt.Errorf("compiled template has type %T", response.Instance)
	}

	values := make([]templateValue, 0, len(config.Values))
	for _, key := range shared.SortedUniqueKeys(config.Values) {
		value := config.Values[key]
		compiled := templateValue{key: key}
		switch typed := value.(type) {
		case map[string]interface{}:
			_, hasType := typed["@type"].(string)
			if hasType {
				child, compileErr := c.compileNode(typed)
				if compileErr != nil {
					return nil, compileErr
				}
				compiled.child = child
			} else {
				compiled.static = shared.CloneMapDeep(typed)
			}
		case string:
			compiled.static = typed
		case []interface{}:
			stringsOnly := make([]string, 0, len(typed))
			for _, item := range typed {
				if text, ok := item.(string); ok {
					stringsOnly = append(stringsOnly, text)
				}
			}
			compiled.static = stringsOnly
		default:
			continue
		}
		values = append(values, compiled)
	}

	templateContent, err := c.resolveTemplate(config.TemplateOptions)
	if err != nil {
		return nil, err
	}
	parsed, err := shared.ParsedGenericTemplate(templateContent)
	if err != nil {
		return nil, fmt.Errorf("error parsing template: %w", err)
	}

	querySet := -1
	policy := templateParamsPolicy(parsed)
	if policy != paramsUnused {
		allowed := apiutil.DefaultQueryKeys
		if config.AllowedQueryKeys != nil {
			allowed = config.AllowedQueryKeys
		}
		querySet = c.registerQuerySet(allowed)
	}
	config.Values = nil

	node := &templateNode{
		config:   config,
		template: parsed,
		values:   values,
		querySet: querySet,
		params:   policy,
		warnings: append([]string(nil), response.Warnings...),
	}
	if len(values) == 1 && values[0].child != nil && forwardsOnlyChild(templateContent, values[0].key, parsed.Name()) {
		node.forwardChild = values[0].child
	}
	if len(values) == 0 && isLiteralTemplate(templateContent, parsed.Name()) {
		// html/template still owns comment removal, escaping and invalid-context
		// errors. Use a private tree so preparation cannot mutate the shared cache.
		literal, err := htmltemplate.New(parsed.Name()).Parse(templateContent)
		if err == nil {
			var output strings.Builder
			if err := literal.Execute(&output, nil); err == nil {
				text := output.String()
				node.literal = &text
			}
		}
	}
	return node, nil
}

func isLiteralTemplate(source, rootName string) bool {
	parsed, err := texttemplate.New(rootName).Parse(source)
	if err != nil || parsed.Tree == nil || len(parsed.Templates()) != 1 {
		return false
	}
	for _, node := range parsed.Tree.Root.Nodes {
		if _, ok := node.(*parse.TextNode); !ok {
			return false
		}
	}
	return true
}

// A sole component selector already receives trusted HTML in text context.
// Inspect a private parse tree, not the cached tree html/template may escape.
func forwardsOnlyChild(source, key, rootName string) bool {
	if key == "Params" {
		return false
	}
	parsed, err := texttemplate.New(rootName).Parse(source)
	if err != nil || parsed.Tree == nil || len(parsed.Templates()) != 1 || len(parsed.Tree.Root.Nodes) != 1 {
		return false
	}
	action, ok := parsed.Tree.Root.Nodes[0].(*parse.ActionNode)
	if !ok || len(action.Pipe.Decl) != 0 || len(action.Pipe.Cmds) != 1 || len(action.Pipe.Cmds[0].Args) != 1 {
		return false
	}
	field, ok := action.Pipe.Cmds[0].Args[0].(*parse.FieldNode)
	return ok && len(field.Ident) == 1 && field.Ident[0] == key
}

func (c *compiler) registerQuerySet(keys []string) int {
	normalized := append([]string(nil), keys...)
	slices.Sort(normalized)
	normalized = slices.Compact(normalized)
	for index, existing := range c.querySets {
		if slices.Equal(existing, normalized) {
			return index
		}
	}
	c.querySets = append(c.querySets, normalized)
	return len(c.querySets) - 1
}

func (c *compiler) compileTree(raw map[string]interface{}) (renderNode, error) {
	response, err := c.manager.MakeInstance(typeRequest(composite.TreeRendererConfigGetName(), raw))
	if err != nil {
		return nil, err
	}
	config, ok := response.Instance.(composite.TreeConfig)
	if !ok {
		return nil, fmt.Errorf("compiled tree has type %T", response.Instance)
	}

	keys := composite.OrderedTreeKeys(config.Items)
	children := make([]treeChild, 0, len(keys))
	for _, key := range keys {
		childRaw, ok := config.Items[key].(map[string]interface{})
		if !ok {
			return nil, notEligible(fmt.Sprintf("tree child %s is not a component map", key))
		}
		childType, ok := childRaw["@type"].(string)
		if !ok || childType == "" {
			return nil, notEligible(fmt.Sprintf("tree child %s has no valid @type", key))
		}

		localConfig := shared.CloneMapDeep(childRaw)
		localConfig["hyperbrickskey"] = key
		localConfig["hyperbricksfile"] = config.Composite.Meta.HyperBricksFile
		localConfig["hyperbrickspath"] = composite.JoinTreePath(config.Composite.Meta.HyperBricksPath, key)

		child, compileErr := c.compileNode(localConfig)
		if compileErr != nil {
			return nil, compileErr
		}
		children = append(children, treeChild{key: key, node: child})
	}

	return &treeNode{
		enclose:  config.Enclose,
		children: children,
		warnings: append([]string(nil), response.Warnings...),
	}, nil
}

func (c *compiler) compileTyped(typeName string, raw map[string]interface{}) (renderNode, error) {
	response, err := c.manager.MakeInstance(typeRequest(typeName, raw))
	if err != nil {
		return nil, err
	}
	renderer := c.manager.GetRenderComponent(typeName)
	if renderer == nil {
		return nil, fmt.Errorf("renderer %s is not registered", typeName)
	}
	return &typedNode{
		renderer: renderer,
		instance: response.Instance,
		warnings: append([]string(nil), response.Warnings...),
	}, nil
}

func (c *compiler) resolveTemplate(options composite.TemplateOptions) (string, error) {
	if options.Inline != "" {
		return options.Inline, nil
	}
	if options.Template == "" {
		return "", fmt.Errorf(".template or a .inline is required")
	}
	if c.templateProvider != nil {
		if content, found := c.templateProvider(options.Template); found {
			return content, nil
		}
	}
	content, err := composite.GetTemplateFileContent(options.Template)
	if err != nil {
		return "", fmt.Errorf("failed to load template file %q: %w", options.Template, err)
	}
	return content, nil
}

func typeRequest(typeName string, data map[string]interface{}) typefactory.TypeRequest {
	return typefactory.TypeRequest{TypeName: typeName, Data: data}
}
