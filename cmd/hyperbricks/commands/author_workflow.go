package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/spaces"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v4"
)

type authorDiagnostic struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Path     string `json:"path"`
	Message  string `json:"message"`
}

func authorWarnings(s authorSpec) []authorDiagnostic {
	var warnings []authorDiagnostic
	var data func(interface{}, string)
	data = func(value interface{}, path string) {
		switch v := value.(type) {
		case map[string]interface{}:
			_, typed := v["type"].(string)
			_, properties := v["properties"]
			_, children := v["children"]
			if typed && (properties || children) {
				warnings = append(warnings, authorDiagnostic{"warning", "brick_emitted_as_data", path, "This object resembles a v2 brick spec but will be emitted as plain data. For a renderable Template value, use children with slot: values."})
			}
			var keys []string
			for key := range v {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				data(v[key], path+"."+key)
			}
		case []interface{}:
			for i, child := range v {
				data(child, fmt.Sprintf("%s[%d]", path, i))
			}
		}
	}
	var brick func(authorBrick, string)
	brick = func(b authorBrick, path string) {
		data(b.Properties, path+".properties")
		for i, child := range b.Children {
			brick(child, fmt.Sprintf("%s.children[%d]", path, i))
		}
	}
	if s.Operations != nil {
		for i, op := range s.Operations {
			brick(op.Brick, fmt.Sprintf("operations[%d].brick", i))
		}
	} else {
		brick(s.Brick, "brick")
	}
	return warnings
}

func prepareAuthorBatch(module, config string, s authorSpec) (*scaffoldPlan, error) {
	if len(s.Operations) == 0 {
		return nil, fmt.Errorf("operations must not be empty")
	}
	if s.Operation != "" || s.File != "" || s.ImportInto != "" || s.Target != "" || s.Source != "" || s.Title != "" || s.Route != "" || !emptyAuthorBrick(s.Brick) {
		return nil, fmt.Errorf("batch envelope accepts version, revision, and operations only")
	}
	m, err := loadAuthoringModule(module, config)
	if err != nil {
		return nil, err
	}
	p, err := newScaffoldPlan(m)
	if err != nil {
		return nil, err
	}
	p.staged = true
	for i, op := range s.Operations {
		err = stageAuthorOperation(p, op)
		if err != nil {
			return nil, fmt.Errorf("operations[%d]: %w", i, err)
		}
	}
	p.Name, p.Type = "batch", "batch"
	if err := validateAuthorSources(p); err != nil {
		return nil, err
	}
	p.Command = "hyperbricks author apply --spec <spec-file>"
	return p, nil
}

func emptyAuthorBrick(b authorBrick) bool {
	return b.Name == "" && b.Type == "" && b.Inherit == "" && b.Slot == "" && len(b.Properties) == 0 && len(b.Children) == 0 && len(b.PropertyOrder) == 0
}

func validateAuthorSources(p *scaffoldPlan) error {
	values, _, err := p.graph(true)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if values[name]["@type"] != "<HYPERMEDIA>" {
			continue
		}
		if _, err := spaces.SourceFields(values[name], p.module.Directories); err != nil {
			return fmt.Errorf("source %s: %w; preserve declared editable targets or explicitly override the editing contract", name, err)
		}
	}
	return nil
}

func stageAuthorOperation(p *scaffoldPlan, op authorSpec) error {
	if op.Version != 0 || op.Revision != "" || op.Operations != nil {
		return fmt.Errorf("batch operations share the envelope version and revision; nested batches are not supported")
	}
	if op.Source != "" || op.Title != "" || op.Route != "" {
		return fmt.Errorf("batch brick operations use brick.properties for title/route and do not accept source")
	}
	if op.Operation != "add-root" && op.Operation != "add-child" {
		return fmt.Errorf("batches currently support add-root and add-child; use a single create-space spec for Spaces")
	}
	n, err := authorNode(op.Brick)
	if err != nil {
		return err
	}
	switch op.Operation {
	case "add-root":
		if op.Target != "" || op.Brick.Slot != "" {
			return fmt.Errorf("add-root does not accept target or slot")
		}
		_, err = prepareScaffoldNodeInPlan(scaffoldSpec{Type: op.Brick.Type, Name: op.Brick.Name, File: op.File, ImportInto: op.ImportInto}, n, p)
	case "add-child":
		if op.Target == "" || op.File != "" || op.ImportInto != "" {
			return fmt.Errorf("add-child requires target and derives its file from source ownership")
		}
		_, err = prepareAuthorChildInPlan("", "", op, n, p)
	default:
		return fmt.Errorf("batches currently support add-root and add-child; use a single create-space spec for Spaces")
	}
	return err
}

type authorEmission struct {
	Operation string      `json:"operation"`
	Name      string      `json:"name"`
	Target    string      `json:"target,omitempty"`
	Slot      string      `json:"slot,omitempty"`
	Route     interface{} `json:"route,omitempty"`
	Section   interface{} `json:"section,omitempty"`
	YAML      string      `json:"yaml,omitempty"`
}

type authorFileChange struct {
	File           string   `json:"file"`
	Action         string   `json:"action"`
	ImportsAdded   []string `json:"imports_added,omitempty"`
	ImportsRemoved []string `json:"imports_removed,omitempty"`
}

func summarizeAuthorFile(f scaffoldFile) authorFileChange {
	change := authorFileChange{File: f.Path, Action: f.Action}
	if !strings.HasSuffix(f.Path, ".hyperbricks.yaml") {
		return change
	}
	imports := func(raw string) []string {
		doc, err := sourceYAML([]byte(raw))
		if err != nil {
			return nil
		}
		n := yget(doc, "imports")
		if n == nil || n.Tag == "!!null" {
			return nil
		}
		if n.Kind == yaml.ScalarNode {
			return []string{n.Value}
		}
		var refs []string
		for _, item := range n.Content {
			refs = append(refs, item.Value)
		}
		return refs
	}
	before, after := imports(f.Before), imports(f.After)
	for _, ref := range after {
		if !containsString(before, ref) {
			change.ImportsAdded = append(change.ImportsAdded, ref)
		}
	}
	for _, ref := range before {
		if !containsString(after, ref) {
			change.ImportsRemoved = append(change.ImportsRemoved, ref)
		}
	}
	return change
}

func reportAuthorApply(c *cobra.Command, p *scaffoldPlan, err error, dry, jsonOutput, compact bool, s authorSpec, summaryMode ...bool) error {
	summary := len(summaryMode) != 0 && summaryMode[0]
	warnings := authorWarnings(s)
	if !jsonOutput && !compact && !summary {
		for _, warning := range warnings {
			fmt.Fprintf(c.ErrOrStderr(), "warning [%s] %s: %s\n", warning.Code, warning.Path, warning.Message)
		}
		return scaffoldReport(c, p, err, dry, false)
	}
	Exit, ExitCode = true, 0
	status := "created"
	if dry {
		status = "preview"
	}
	if err != nil {
		status, ExitCode = "error", 1
	}
	result := struct {
		Status        string             `json:"status"`
		Plan          *scaffoldPlan      `json:"plan,omitempty"`
		Changes       []authorFileChange `json:"changes,omitempty"`
		Emissions     []authorEmission   `json:"emissions,omitempty"`
		Diagnostics   []authorDiagnostic `json:"diagnostics,omitempty"`
		Error         string             `json:"error,omitempty"`
		InputRevision string             `json:"input_revision,omitempty"`
		Operations    []authorEmission   `json:"operations,omitempty"`
	}{Status: status, Diagnostics: warnings}
	if err != nil {
		result.Error = err.Error()
	}
	if summary {
		result.InputRevision = s.Revision
		ops := s.Operations
		if ops == nil {
			ops = []authorSpec{s}
		}
		for _, op := range ops {
			emission := authorEmission{Operation: op.Operation, Name: op.Brick.Name, Target: op.Target, Slot: op.Brick.Slot, Route: op.Brick.Properties["route"], Section: op.Brick.Properties["section"]}
			if op.Operation == "create-space" {
				emission.Route = op.Route
			}
			result.Operations = append(result.Operations, emission)
		}
		if p != nil {
			for _, f := range p.Files {
				result.Changes = append(result.Changes, summarizeAuthorFile(f))
			}
		}
	} else if !compact {
		result.Plan = p
	} else if p != nil {
		for _, f := range p.Files {
			result.Changes = append(result.Changes, summarizeAuthorFile(f))
		}
		ops := s.Operations
		if ops == nil {
			ops = []authorSpec{s}
		}
		for _, op := range ops {
			if op.Operation == "create-space" {
				// New leaves are small; modified import indexes are summarized above.
				for _, f := range p.Files {
					if f.Action == "create" {
						result.Emissions = append(result.Emissions, authorEmission{Operation: op.Operation, Name: op.Brick.Name, YAML: f.After})
					}
				}
				continue
			}
			n, e := authorNode(op.Brick)
			if e != nil {
				continue
			}
			root := ymap()
			yput(root, op.Brick.Name, n)
			b, e := yamlBytes(root)
			if e == nil {
				result.Emissions = append(result.Emissions, authorEmission{op.Operation, op.Brick.Name, op.Target, op.Brick.Slot, op.Brick.Properties["route"], op.Brick.Properties["section"], string(b)})
			}
		}
	}
	e := json.NewEncoder(c.OutOrStdout())
	if !jsonOutput {
		e.SetIndent("", "  ")
	}
	if err := e.Encode(result); err != nil {
		ExitCode = 1
		return err
	}
	return nil
}

type authorFocusedContext struct {
	Version     int                      `json:"version"`
	Revision    string                   `json:"revision"`
	Directories map[string]string        `json:"directories"`
	Roots       []map[string]interface{} `json:"roots"`
	Target      string                   `json:"target"`
	Effective   map[string]interface{}   `json:"effective"`
	Templates   []map[string]string      `json:"templates"`
	Example     *authorSpec              `json:"example,omitempty"`
}

type authorRootSummary struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	File        string      `json:"file"`
	Route       interface{} `json:"route,omitempty"`
	PageSource  bool        `json:"page_source"`
	SpaceSource bool        `json:"space_source"`
}

type authorDiscovery struct {
	Version  int                 `json:"version"`
	Revision string              `json:"revision"`
	Roots    []authorRootSummary `json:"roots"`
}

func listAuthorContext(ctx *authorContext) authorDiscovery {
	d := authorDiscovery{Version: 2, Revision: ctx.Revision, Roots: []authorRootSummary{}}
	sources := map[string]bool{}
	for _, source := range ctx.SpaceSources {
		sources[source.Name] = true
	}
	for _, root := range ctx.Roots {
		token, _ := root.Effective["@type"].(string)
		route := root.Effective["route"]
		d.Roots = append(d.Roots, authorRootSummary{
			Name: root.Name, Type: strings.ToLower(strings.Trim(token, "<>")),
			File: root.File, Route: route,
			PageSource:  token == "<HYPERMEDIA>" && (route == nil || route == ""),
			SpaceSource: sources[root.Name],
		})
	}
	return d
}

func authorUnknownTarget(ctx *authorContext, target string) error {
	var candidates []string
	rootName := strings.Split(target, ".")[0]
	for _, root := range listAuthorContext(ctx).Roots {
		if root.Name == rootName || root.PageSource || root.SpaceSource || strings.Contains(root.Name, rootName) {
			candidates = append(candidates, root.Name)
		}
	}
	if len(candidates) == 0 {
		for _, root := range ctx.Roots {
			candidates = append(candidates, root.Name)
		}
	}
	if len(candidates) > 5 {
		candidates = candidates[:5]
	}
	return fmt.Errorf("target %q is not an effective loaded object; available targets/page sources: [%s]; discover with: hyperbricks author context -m %q --list --json", target, strings.Join(candidates, ", "), ctx.Module)
}

func focusAuthorContext(ctx *authorContext, target string) (*authorFocusedContext, error) {
	f := &authorFocusedContext{Version: 2, Revision: ctx.Revision, Directories: ctx.Directories, Target: target, Roots: []map[string]interface{}{}, Templates: []map[string]string{}}
	parts := strings.Split(target, ".")
	var owner string
	for _, root := range ctx.Roots {
		entry := map[string]interface{}{"name": root.Name, "file": root.File}
		for _, key := range []string{"route", "title", "section", "index"} {
			if v, ok := root.Effective[key]; ok {
				entry[key] = v
			}
		}
		f.Roots = append(f.Roots, entry)
		if root.Name == parts[0] {
			f.Effective, owner = root.Effective, root.File
		}
	}
	for _, part := range parts[1:] {
		f.Effective, _ = f.Effective[part].(map[string]interface{})
	}
	if f.Effective == nil {
		return nil, authorUnknownTarget(ctx, target)
	}
	seen := map[string]bool{}
	var visit func(map[string]interface{}) error
	visit = func(obj map[string]interface{}) error {
		_, component := obj["@type"].(string)
		if file, ok := obj["template"].(string); ok && component && !seen[file] {
			path, err := authoringPath(ctx.Directories["templates"], file)
			if err != nil {
				return err
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			seen[file] = true
			f.Templates = append(f.Templates, map[string]string{"path": file, "source": string(b)})
		}
		var keys []string
		for key := range obj {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if child, ok := obj[key].(map[string]interface{}); ok {
				if err := visit(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := visit(f.Effective); err != nil {
		return nil, err
	}
	content, _ := f.Effective["content"].(map[string]interface{})
	values, _ := content["values"].(map[string]interface{})
	if len(parts) == 1 && f.Effective["@type"] == "<HYPERMEDIA>" && values["body"] != nil {
		example := inheritedAuthorPage(parts[0], owner)
		for _, source := range ctx.SpaceSources {
			if source.Name != parts[0] {
				continue
			}
			for _, field := range source.Fields {
				if field.ID == "/content/values/body" {
					example.Brick.Children[0].Children = nil
					example.Brick.Children[0].Properties = map[string]interface{}{"values": map[string]interface{}{"body": field.Default}}
				}
			}
		}
		example.Revision = ctx.Revision
		// Pick a top-level scope that already loads the shell so references resolve.
		profile, err := filepath.Rel(ctx.Module, ctx.Config)
		if err != nil {
			return nil, err
		}
		m, err := loadAuthoringModule(ctx.Module, filepath.ToSlash(profile))
		if err != nil {
			return nil, err
		}
		p, err := newScaffoldPlan(m)
		if err != nil {
			return nil, err
		}
		for _, top := range p.tops {
			paths, err := yamlparserDocumentPaths(p, top)
			if err != nil {
				return nil, err
			}
			if containsString(paths, filepath.Join(ctx.Directories["hyperbricks"], filepath.FromSlash(owner))) {
				rel, _ := filepath.Rel(ctx.Directories["hyperbricks"], top)
				example.File = filepath.ToSlash(rel)
				break
			}
		}
		delete(example.Brick.Properties, "section")
		for _, root := range ctx.Roots {
			if root.Effective["section"] == nil || root.Effective["route"] == nil {
				continue
			}
			path := filepath.Join(ctx.Directories["hyperbricks"], filepath.FromSlash(root.File))
			raw, err := p.read(path)
			if err != nil {
				return nil, err
			}
			doc, err := sourceYAML(raw)
			if err != nil {
				return nil, err
			}
			page := yget(doc, root.Name)
			if page != nil {
				ref := yget(page, "inherit")
				if ref != nil && ref.Value == parts[0] {
					example.Brick.Properties["section"] = root.Effective["section"]
					break
				}
			}
		}
		f.Example = &example
	}
	return f, nil
}

func inheritedAuthorPage(shell, file string) authorSpec {
	return authorSpec{Version: 2, Operation: "add-root", File: file, Brick: authorBrick{Name: "about_page", Type: "hypermedia", Inherit: shell, Properties: map[string]interface{}{"route": "about", "title": "About", "section": "scaffold_navigation", "index": 40}, Children: []authorBrick{{Name: "content", Type: "template", Children: []authorBrick{{Name: "body", Type: "tree", Slot: "values", Children: []authorBrick{{Name: "intro", Type: "markdown", Properties: map[string]interface{}{"content": "# About\n\nPage content.\n"}}}}}}}}}
}
