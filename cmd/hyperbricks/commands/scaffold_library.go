package commands

import (
	"fmt"
	"os"
	"sort"

	"go.yaml.in/yaml/v4"
)

const scaffoldLibraryPath = "assets/scaffold/library.yaml"

const (
	scaffoldRouteMarker = "__SCAFFOLD_ROUTE__"
	scaffoldTitleMarker = "__SCAFFOLD_TITLE__"
)

type scaffoldLibraryAsset struct{ Source, Directory, Target string }

var scaffoldLibraryAssets = map[string][]scaffoldLibraryAsset{
	"esbuild":       {{"assets/scaffold/resources/js/scaffold_3f2k.js", "resources", "js/scaffold_3f2k.js"}},
	"image":         {{"assets/scaffold/resources/images/image_192a.png", "resources", "images/image_192a.png"}},
	"images":        {{"assets/scaffold/resources/images/image_192a.png", "resources", "images/image_192a.png"}},
	"json_render":   {{"assets/scaffold/resources/data/items_2sa5.json", "resources", "data/items_2sa5.json"}},
	"styles":        {{"assets/scaffold/resources/css/scaffold_4d9m.css", "resources", "css/scaffold_4d9m.css"}},
	"template_file": {{"assets/scaffold/templates/template_2sa5.html", "templates", "template_2sa5.html"}},
	"markdown_file": {{"assets/scaffold/resources/content/markdown_3b7e.md", "resources", "content/markdown_3b7e.md"}},
}

var scaffoldLibraryRootNames = map[string]string{
	"esbuild":       "esbuild_3f2k",
	"image":         "image_192a",
	"images":        "images_192a",
	"json_render":   "json_render_2sa5",
	"styles":        "styles_4d9m",
	"template_file": "template_2sa5",
	"markdown_file": "markdown_3b7e",
}

func scaffoldLibraryRootName(starter, kind string) string {
	if name := scaffoldLibraryRootNames[starter]; name != "" {
		return name
	}
	return "new_" + kind
}

type scaffoldLibraryTemplate struct {
	Starter string
	Type    string
	Node    *yaml.Node
}

func scaffoldLibraryTemplates() (map[string]scaffoldLibraryTemplate, error) {
	raw, err := embeddedFiles.ReadFile(scaffoldLibraryPath)
	if err != nil {
		return nil, fmt.Errorf("read scaffold template library: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse scaffold template library: %w", err)
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("scaffold template library must be a mapping")
	}
	templates := yget(doc.Content[0], "templates")
	if templates == nil || templates.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("scaffold template library requires a templates mapping")
	}
	result := map[string]scaffoldLibraryTemplate{}
	for i := 0; i < len(templates.Content); i += 2 {
		starter, node := templates.Content[i].Value, templates.Content[i+1]
		if node.Kind != yaml.SequenceNode || yget(node, "type") == nil {
			return nil, fmt.Errorf("scaffold template %s must be an ordered component", starter)
		}
		kind := yget(node, "type").Value
		if _, err := scaffoldDefinition(kind); err != nil {
			return nil, fmt.Errorf("scaffold template %s: %w", starter, err)
		}
		result[starter] = scaffoldLibraryTemplate{Starter: starter, Type: kind, Node: node}
	}
	return result, nil
}

func scaffoldLibraryTypes(category string) ([]string, error) {
	templates, err := scaffoldLibraryTemplates()
	if err != nil {
		return nil, err
	}
	types := []string{}
	for starter, template := range templates {
		def, _ := scaffoldDefinition(template.Type)
		if category == "" || scaffoldCategory(def) == category {
			types = append(types, starter)
		}
	}
	sort.Strings(types)
	return types, nil
}

func scaffoldLibraryNode(starter, route, title string) (*yaml.Node, error) {
	templates, err := scaffoldLibraryTemplates()
	if err != nil {
		return nil, err
	}
	template, ok := templates[starter]
	if !ok {
		return nil, fmt.Errorf("no fast scaffold template for starter %q", starter)
	}
	// Re-parse the selected native YAML node so preview mutations never change the
	// embedded library representation used by later wizard runs.
	raw, err := yaml.Marshal(template.Node)
	if err != nil {
		return nil, err
	}
	var node yaml.Node
	if err := yaml.Unmarshal(raw, &node); err != nil {
		return nil, fmt.Errorf("copy scaffold template %s: %w", starter, err)
	}
	if len(node.Content) != 1 {
		return nil, fmt.Errorf("copy scaffold template %s: expected one YAML node", starter)
	}
	n := node.Content[0]
	scaffoldReplaceLibraryMarkers(n, route, title)
	return n, nil
}

func scaffoldReplaceLibraryMarkers(n *yaml.Node, route, title string) {
	if n.Kind == yaml.MappingNode {
		content := make([]*yaml.Node, 0, len(n.Content))
		for i := 0; i < len(n.Content); i += 2 {
			key, value := n.Content[i], n.Content[i+1]
			if (value.Value == scaffoldRouteMarker && route == "") || (value.Value == scaffoldTitleMarker && title == "") {
				continue
			}
			content = append(content, key, value)
		}
		n.Content = content
	}
	if n.Kind == yaml.ScalarNode {
		switch n.Value {
		case scaffoldRouteMarker:
			n.Value = route
		case scaffoldTitleMarker:
			n.Value = title
		}
	}
	content := make([]*yaml.Node, 0, len(n.Content))
	for _, child := range n.Content {
		scaffoldReplaceLibraryMarkers(child, route, title)
		if n.Kind == yaml.SequenceNode && child.Kind == yaml.MappingNode && len(child.Content) == 0 {
			continue
		}
		content = append(content, child)
	}
	n.Content = content
}

func prepareScaffoldLibrary(spec scaffoldSpec) (*scaffoldPlan, error) {
	starter := spec.Starter
	if starter == "" {
		starter = spec.Type
	}
	node, err := scaffoldLibraryNode(starter, spec.Route, spec.Title)
	if err != nil {
		return nil, err
	}
	m, err := loadAuthoringModule(spec.Module, spec.Config)
	if err != nil {
		return nil, err
	}
	p, err := newScaffoldPlan(m)
	if err != nil {
		return nil, err
	}
	for _, asset := range scaffoldLibraryAssets[starter] {
		target, err := authoringPath(m.Directories[asset.Directory], asset.Target)
		if err != nil {
			return nil, err
		}
		if _, err := p.read(target); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		raw, err := embeddedFiles.ReadFile(asset.Source)
		if err != nil {
			return nil, fmt.Errorf("read scaffold asset %s: %w", asset.Source, err)
		}
		if err := p.add(target, raw); err != nil {
			return nil, err
		}
	}
	return prepareScaffoldNodeInPlanValidated(spec, node, p, true)
}
