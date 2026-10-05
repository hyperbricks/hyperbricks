package renderplan

import (
	"bytes"
	"html/template"
	"io"
	"mime"
	"slices"
	"strconv"
	"strings"
	"text/template/parse"
)

// combinedTemplateNode executes an eligible template/tree graph in one Go
// template. Bindings retain each original template's query-set scope; the
// original graph remains the authority for diagnostics if execution fails.
type combinedTemplateNode struct {
	template *template.Template
	bindings []combinedTemplateBinding
	original renderNode
}

type combinedTemplateBinding struct {
	name     string
	querySet int // -1 for an immutable string value
	value    string
}

func (n *combinedTemplateNode) Render(state *renderState) (string, []error) {
	data := make(map[string]interface{}, len(n.bindings))
	for _, binding := range n.bindings {
		if binding.querySet < 0 {
			data[binding.name] = binding.value
		} else if params, ok := state.params(binding.querySet, false); ok {
			data[binding.name] = params
		}
	}
	var output strings.Builder
	if err := n.template.Execute(&output, data); err != nil {
		return n.original.Render(state)
	}
	return output.String(), nil
}

type combinedActionCheck struct {
	node     *parse.ActionNode
	escapers []string
}

type combinedTextCheck struct {
	node *parse.TextNode
	text []byte
}

type templateCombiner struct {
	sources       map[*templateNode]string
	bindings      []combinedTemplateBinding
	queryBindings map[int]string
	actions       []combinedActionCheck
	texts         []combinedTextCheck
	templates     int
	nodes         int
	root          *parse.Tree
}

// combineTemplates is deliberately limited to pure, unconditional field
// interpolation. Preparation never calls author functions or custom renderers.
// Private Go templates prove that each source is valid and that combining its
// nodes preserves both literal output and contextual escaping.
func combineTemplates(root renderNode, sources map[*templateNode]string) *combinedTemplateNode {
	c := templateCombiner{sources: sources, queryBindings: make(map[int]string)}
	nodes, ok := c.combine(root, 0)
	if !ok || c.templates < 2 || c.root == nil {
		return nil
	}
	tree := c.root.Copy()
	tree.Root.Nodes = nodes
	combined, err := template.New(tree.Name).AddParseTree(tree.Name, tree)
	if err != nil {
		return nil
	}
	data := make(map[string]interface{}, len(c.bindings))
	for _, binding := range c.bindings {
		if binding.querySet < 0 {
			data[binding.name] = binding.value
		} else {
			data[binding.name] = map[string]interface{}{}
		}
	}
	if err := combined.Execute(io.Discard, data); err != nil {
		return nil
	}
	for _, check := range c.actions {
		if !slices.Equal(actionEscapers(check.node), check.escapers) {
			return nil
		}
	}
	for _, check := range c.texts {
		if !bytes.Equal(check.node.Text, check.text) {
			return nil
		}
	}
	return &combinedTemplateNode{template: combined, bindings: c.bindings, original: root}
}

func (c *templateCombiner) combine(node renderNode, depth int) ([]parse.Node, bool) {
	// Bound optimizer preparation, independently of response size. Larger graphs
	// still use the existing immutable render plan.
	c.nodes++
	if depth > 64 || c.nodes > 1024 {
		return nil, false
	}
	switch typed := node.(type) {
	case *treeNode:
		if typed.enclose != "" || len(typed.warnings) != 0 {
			return nil, false
		}
		var result []parse.Node
		for _, child := range typed.children {
			parts, ok := c.combine(child.node, depth+1)
			if !ok {
				return nil, false
			}
			result = append(result, parts...)
		}
		return result, true
	case *templateNode:
		return c.combineTemplate(typed, depth)
	default:
		return nil, false
	}
}

func (c *templateCombiner) combineTemplate(node *templateNode, depth int) ([]parse.Node, bool) {
	config := node.config
	if config.Enclose != "" || len(node.warnings) != 0 || len(config.Validate()) != 0 || node.params == paramsPrivate {
		return nil, false
	}
	if config.OutputContentType != "" {
		kind, _, err := mime.ParseMediaType(config.OutputContentType)
		if err != nil || kind != "text/html" {
			return nil, false
		}
	}
	source, found := c.sources[node]
	if !found {
		return nil, false
	}
	// No function map: a function call is ineligible before it can execute.
	private, err := template.New(node.template.Name()).Parse(source)
	if err != nil || len(private.Templates()) != 1 || private.Tree == nil || private.Tree.Root == nil {
		return nil, false
	}
	values := make(map[string]templateValue, len(node.values))
	seed := make(map[string]interface{}, len(node.values)+1)
	for _, value := range node.values {
		// Params is request-dependent and may override a configured value. Keep
		// that collision on the original path, including requests without a URL.
		if value.key == "Params" {
			return nil, false
		}
		values[value.key] = value
		if value.child != nil {
			seed[value.key] = template.HTML("combined-child")
		} else if text, ok := value.static.(string); ok {
			seed[value.key] = text
		} else {
			return nil, false
		}
	}
	if node.querySet >= 0 {
		seed["Params"] = map[string]interface{}{}
	}
	for _, part := range private.Tree.Root.Nodes {
		switch typed := part.(type) {
		case *parse.TextNode:
		case *parse.ActionNode:
			field := directTemplateField(typed)
			if field == nil {
				return nil, false
			}
			if len(field.Ident) == 2 && field.Ident[0] == "Params" && node.querySet >= 0 {
				continue
			}
			if len(field.Ident) != 1 {
				return nil, false
			}
			if _, exists := values[field.Ident[0]]; !exists {
				return nil, false
			}
		default:
			return nil, false
		}
	}
	// Go performs comment removal and escaping on this private tree. Starting
	// from its escaped text also preserves boundaries such as "<" + "b>".
	if err := private.Execute(io.Discard, seed); err != nil {
		return nil, false
	}
	c.templates++
	if c.root == nil {
		c.root = private.Tree
	}
	used := make(map[string]int, len(values))
	staticNames := make(map[string]string)
	var result []parse.Node
	for _, part := range private.Tree.Root.Nodes {
		switch typed := part.(type) {
		case *parse.TextNode:
			text := typed.Copy().(*parse.TextNode)
			c.texts = append(c.texts, combinedTextCheck{text, bytes.Clone(text.Text)})
			result = append(result, text)
		case *parse.ActionNode:
			field := typed.Pipe.Cmds[0].Args[0].(*parse.FieldNode)
			if len(field.Ident) == 1 && values[field.Ident[0]].child != nil {
				key := field.Ident[0]
				used[key]++
				// Trusted child HTML passes through only in HTML text context.
				// Any other Go escaper (attributes, JS, CSS, RCDATA, URL, comments)
				// keeps the entire graph on its original path.
				if used[key] != 1 || !slices.Equal(actionEscapers(typed), []string{"_html_template_htmlescaper"}) {
					return nil, false
				}
				child, ok := c.combine(values[key].child, depth+1)
				if !ok {
					return nil, false
				}
				result = append(result, child...)
				continue
			}
			action := typed.Copy().(*parse.ActionNode)
			// Let Go install escaping again on the combined private tree, then
			// require exactly the same escaping sequence as the original source.
			action.Pipe.Cmds = action.Pipe.Cmds[:1]
			renamed := action.Pipe.Cmds[0].Args[0].(*parse.FieldNode)
			if len(field.Ident) == 2 {
				name, exists := c.queryBindings[node.querySet]
				if !exists {
					name = "Query" + strconv.Itoa(len(c.queryBindings))
					c.queryBindings[node.querySet] = name
					c.bindings = append(c.bindings, combinedTemplateBinding{name: name, querySet: node.querySet})
				}
				renamed.Ident = []string{name, field.Ident[1]}
			} else {
				key := field.Ident[0]
				name, exists := staticNames[key]
				if !exists {
					name = "Value" + strconv.Itoa(len(c.bindings))
					staticNames[key] = name
					c.bindings = append(c.bindings, combinedTemplateBinding{name: name, querySet: -1, value: values[key].static.(string)})
				}
				renamed.Ident = []string{name}
			}
			c.actions = append(c.actions, combinedActionCheck{action, actionEscapers(typed)})
			result = append(result, action)
		}
	}
	for key, value := range values {
		// Preserve eager child execution rather than dropping unused components.
		if value.child != nil && used[key] != 1 {
			return nil, false
		}
	}
	return result, true
}

func directTemplateField(action *parse.ActionNode) *parse.FieldNode {
	if action.Pipe == nil || len(action.Pipe.Decl) != 0 || len(action.Pipe.Cmds) != 1 || len(action.Pipe.Cmds[0].Args) != 1 {
		return nil
	}
	field, _ := action.Pipe.Cmds[0].Args[0].(*parse.FieldNode)
	return field
}

func actionEscapers(action *parse.ActionNode) []string {
	var result []string
	for _, command := range action.Pipe.Cmds[1:] {
		result = append(result, command.String())
	}
	return result
}
