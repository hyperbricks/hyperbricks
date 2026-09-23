package commands

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/schema"
	"github.com/hyperbricks/hyperbricks/pkg/spaces"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v4"
)

type authorBrick struct {
	Name          string                 `json:"name"`
	Type          string                 `json:"type"`
	Inherit       string                 `json:"inherit,omitempty"`
	Properties    map[string]interface{} `json:"properties,omitempty"`
	PropertyOrder []string               `json:"property_order,omitempty"`
	Children      []authorBrick          `json:"children,omitempty"`
	Slot          string                 `json:"slot,omitempty"`
}

type authorSpec struct {
	Version    int          `json:"version"`
	Revision   string       `json:"revision,omitempty"`
	Operation  string       `json:"operation"`
	File       string       `json:"file,omitempty"`
	ImportInto string       `json:"import_into,omitempty"`
	Target     string       `json:"target,omitempty"`
	Brick      authorBrick  `json:"brick"`
	Source     string       `json:"source,omitempty"`
	Title      string       `json:"title,omitempty"`
	Route      string       `json:"route,omitempty"`
	Operations []authorSpec `json:"operations,omitempty"`
}

type authorSource struct {
	Path string `json:"path"`
	YAML string `json:"yaml"`
}

type authorRoot struct {
	Name      string                 `json:"name"`
	File      string                 `json:"file"`
	Effective map[string]interface{} `json:"effective"`
}

type authorContext struct {
	Version      int                 `json:"version"`
	Revision     string              `json:"revision"`
	Module       string              `json:"module"`
	Config       string              `json:"config"`
	PackageYAML  string              `json:"package_yaml"`
	Directories  map[string]string   `json:"directories"`
	Files        []authorSource      `json:"files"`
	Roots        []authorRoot        `json:"roots"`
	Assets       map[string][]string `json:"assets"`
	SpaceSources []spaces.Source     `json:"space_sources"`
}

func authorProject(module, config string) (*authorContext, error) {
	m, err := loadAuthoringModule(module, config)
	if err != nil {
		return nil, err
	}
	p, err := newScaffoldPlan(m)
	if err != nil {
		return nil, err
	}
	values, owners, err := p.graph(false)
	if err != nil {
		return nil, err
	}
	ctx := &authorContext{Version: 2, Module: m.Root, Config: m.ConfigPath, PackageYAML: string(m.PackageBytes), Directories: m.Directories, Files: []authorSource{}, Roots: []authorRoot{}, Assets: map[string][]string{}}
	var paths []string
	for path := range p.inputs {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, path := range paths {
		rel, _ := filepath.Rel(m.Root, path)
		// Length-delimited JSON avoids ambiguous path/content hash boundaries.
		b, _ := json.Marshal([]string{filepath.ToSlash(rel), string(p.inputs[path])})
		h.Write(b)
		if path != m.ConfigPath {
			ctx.Files = append(ctx.Files, authorSource{filepath.ToSlash(rel), string(p.inputs[path])})
		}
	}
	var names []string
	for name := range owners {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		rel, _ := filepath.Rel(m.Directories["hyperbricks"], owners[name])
		ctx.Roots = append(ctx.Roots, authorRoot{name, filepath.ToSlash(rel), values[name]})
	}
	for _, key := range []string{"templates", "resources", "static"} {
		ctx.Assets[key] = []string{}
		err := filepath.WalkDir(m.Directories[key], func(path string, d fs.DirEntry, err error) error {
			if os.IsNotExist(err) && path == m.Directories[key] {
				return nil
			}
			if err != nil {
				return err
			}
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("asset symlink is not supported: %s", path)
			}
			if d.IsDir() {
				return nil
			}
			if !d.Type().IsRegular() {
				return fmt.Errorf("expected regular asset: %s", path)
			}
			rel, _ := filepath.Rel(m.Directories[key], path)
			ctx.Assets[key] = append(ctx.Assets[key], filepath.ToSlash(rel))
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			digest := sha256.Sum256(b)
			entry, _ := json.Marshal([]string{key, filepath.ToSlash(rel), hex.EncodeToString(digest[:])})
			h.Write(entry)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	creator, err := spaces.NewCreator(m.Root, m.Directories, m.Config)
	if err != nil {
		return nil, err
	}
	ctx.SpaceSources, err = creator.Sources()
	if err != nil {
		return nil, err
	}
	ctx.Revision = hex.EncodeToString(h.Sum(nil))
	if err := p.verify(); err != nil {
		return nil, err
	}
	return ctx, nil
}

func decodeAuthor(r io.Reader) (authorSpec, error) {
	var s authorSpec
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	if err := d.Decode(&s); err != nil {
		return s, err
	}
	var extra interface{}
	if err := d.Decode(&extra); err != io.EOF {
		return s, fmt.Errorf("spec must contain exactly one JSON object")
	}
	if s.Version != 2 {
		return s, fmt.Errorf("spec version must be 2")
	}
	return s, nil
}

func authorNode(b authorBrick) (*yaml.Node, error) {
	if !scaffoldNamePattern.MatchString(b.Name) || b.Name == "imports" || b.Name == "vars" || b.Name == "type" || b.Name == "inherit" {
		return nil, fmt.Errorf("invalid brick name %q", b.Name)
	}
	if b.Type == "" {
		return nil, fmt.Errorf("brick %s requires type, including when inheriting", b.Name)
	}
	def, err := scaffoldDefinition(b.Type)
	if err != nil {
		return nil, err
	}
	kind := scaffoldTypeName(def)
	allowed := map[string]bool{}
	for _, f := range schema.ExtractDefinition(def).Fields {
		allowed[strings.Split(f.Path, ".")[0]] = true
	}
	for key := range b.Properties {
		if key == "type" || key == "inherit" || !allowed[key] {
			return nil, fmt.Errorf("%s: unsupported property %q", b.Name, key)
		}
	}
	n := yseq()
	yput(n, "type", ystr(kind))
	if b.Inherit != "" {
		yput(n, "inherit", ystr(b.Inherit))
	}
	seen := map[string]bool{}
	order := append([]string{}, b.PropertyOrder...)
	for _, key := range order {
		if _, ok := b.Properties[key]; !ok || seen[key] {
			return nil, fmt.Errorf("%s: property_order contains absent or repeated key %q", b.Name, key)
		}
		seen[key] = true
	}
	for _, key := range []string{"route", "title", "section", "index", "template", "inline", "file", "content", "value", "editable", "values"} {
		if _, ok := b.Properties[key]; ok && !seen[key] {
			order = append(order, key)
			seen[key] = true
		}
	}
	var rest []string
	for key := range b.Properties {
		if !seen[key] {
			rest = append(rest, key)
		}
	}
	sort.Strings(rest)
	order = append(order, rest...)
	for _, key := range order {
		v := &yaml.Node{}
		if err := v.Encode(b.Properties[key]); err != nil {
			return nil, err
		}
		if key == "template" && v.Kind == yaml.ScalarNode && v.Tag == "!!str" {
			mount := ymap()
			yput(mount, "file", v)
			v = mount
		}
		authorScalarStyle(v)
		yput(n, key, v)
	}
	for _, child := range b.Children {
		cn, err := authorNode(child)
		if err != nil {
			return nil, err
		}
		if err := authorMount(n, kind, def.ChildModel, child, cn); err != nil {
			return nil, fmt.Errorf("%s: %w", b.Name, err)
		}
	}
	return n, nil
}

func authorScalarStyle(n *yaml.Node) {
	if n.Kind == yaml.ScalarNode && n.Tag == "!!str" && strings.Contains(n.Value, "\n") {
		n.Style = yaml.LiteralStyle
	}
	for _, child := range n.Content {
		authorScalarStyle(child)
	}
}

func authorMount(n *yaml.Node, kind string, model schema.ChildModel, b authorBrick, child *yaml.Node) error {
	if n.Kind != yaml.SequenceNode {
		return fmt.Errorf("target must use the ordered-list DSL")
	}
	if model == schema.ChildModelNone {
		return fmt.Errorf("%s does not accept children", kind)
	}
	destination := n
	switch b.Slot {
	case "", "body":
		if b.Slot == "body" && kind != "hypermedia" {
			return fmt.Errorf("body slot requires hypermedia")
		}
	case "head":
		if kind != "hypermedia" {
			return fmt.Errorf("head slot requires hypermedia")
		}
		destination = yget(n, "head")
		if destination == nil {
			destination = yseq()
			yput(destination, "type", ystr("head"))
			yput(n, "head", destination)
		}
		if destination.Kind != yaml.SequenceNode {
			return fmt.Errorf("existing head is not a child brick")
		}
	case "values":
		if model != schema.ChildModelValues {
			return fmt.Errorf("values slot requires a values child model")
		}
	default:
		return fmt.Errorf("unknown slot %q", b.Slot)
	}
	if model == schema.ChildModelValues {
		if b.Slot != "" && b.Slot != "values" {
			return fmt.Errorf("%s children belong in values", kind)
		}
		destination = yget(n, "values")
		if destination == nil {
			destination = ymap()
			yput(n, "values", destination)
		}
		if destination.Kind != yaml.MappingNode {
			return fmt.Errorf("values must be a mapping")
		}
	}
	if yget(destination, b.Name) != nil {
		return fmt.Errorf("child/property %s already exists", b.Name)
	}
	if destination == n {
		def, _ := scaffoldDefinition(kind)
		for _, f := range schema.ExtractDefinition(def).Fields {
			if strings.Split(f.Path, ".")[0] == b.Name && !(kind == "hypermedia" && b.Name == "head") {
				return fmt.Errorf("child %s conflicts with a property", b.Name)
			}
		}
	}
	yput(destination, b.Name, child)
	return nil
}

func prepareAuthorChild(module, config string, s authorSpec, child *yaml.Node) (*scaffoldPlan, error) {
	return prepareAuthorChildInPlan(module, config, s, child, nil)
}

func prepareAuthorChildInPlan(module, config string, s authorSpec, child *yaml.Node, p *scaffoldPlan) (*scaffoldPlan, error) {
	var err error
	if p == nil {
		m, err := loadAuthoringModule(module, config)
		if err != nil {
			return nil, err
		}
		p, err = newScaffoldPlan(m)
		if err != nil {
			return nil, err
		}
	}
	m := p.module
	values, owners, err := p.graph(p.staged)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(s.Target, ".")
	owner := owners[parts[0]]
	if owner == "" {
		return nil, fmt.Errorf("target root %q is not loaded", parts[0])
	}
	if strings.HasPrefix(filepath.ToSlash(strings.TrimPrefix(owner, m.Directories["hyperbricks"]+string(filepath.Separator))), "spaces/") && filepath.Base(owner) == "index.hyperbricks.yaml" {
		return nil, fmt.Errorf("managed Spaces indexes contain imports only")
	}
	raw, err := p.overlay(owner)
	if err != nil {
		return nil, err
	}
	doc, err := sourceYAML(raw)
	if err != nil {
		return nil, err
	}
	target := yget(doc, parts[0])
	effective := values[parts[0]]
	for _, part := range parts[1:] {
		if target != nil {
			target = yget(target, part)
		}
		effective, _ = effective[part].(map[string]interface{})
	}
	if target == nil || effective == nil {
		return nil, fmt.Errorf("target %q must exist in its owning source; inherited-only paths require an explicit override", s.Target)
	}
	token, _ := effective["@type"].(string)
	def, err := scaffoldDefinition(token)
	if err != nil {
		return nil, err
	}
	// Do not silently replace children inherited from another brick.
	mount := effective
	if s.Brick.Slot == "head" {
		mount, _ = effective["head"].(map[string]interface{})
	}
	if def.ChildModel == schema.ChildModelValues {
		mount, _ = effective["values"].(map[string]interface{})
	}
	if _, exists := mount[s.Brick.Name]; exists {
		return nil, fmt.Errorf("child/property %s already exists in effective target", s.Brick.Name)
	}
	if err := authorMount(target, scaffoldTypeName(def), def.ChildModel, s.Brick, child); err != nil {
		return nil, err
	}
	out, err := yamlBytes(doc)
	if err != nil {
		return nil, err
	}
	if err := p.add(owner, out); err != nil {
		return nil, err
	}
	updated, _, err := p.graph(true)
	if err != nil {
		return nil, err
	}
	// Validate every effective root: changing a source can affect its inheritors.
	for _, obj := range updated {
		if err := validateScaffoldObjects(p, obj); err != nil {
			return nil, err
		}
	}
	p.Name, p.Type = s.Brick.Name, scaffoldTypeName(def)
	return p, nil
}

func NewAuthorCommand() *cobra.Command {
	var module, config string
	var jsonOutput bool
	cmd := &cobra.Command{Use: "author", Short: "Inspect and change existing HyperBricks projects"}
	cmd.PersistentFlags().StringVarP(&module, "module", "m", "", "module name or directory")
	cmd.PersistentFlags().StringVar(&config, "config", PackageConfigFileName, "package profile relative to module")
	cmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "machine-readable output")
	report := func(c *cobra.Command, value interface{}, err error) error {
		if err != nil {
			return scaffoldReport(c, nil, err, true, jsonOutput)
		}
		Exit, ExitCode = true, 0
		e := json.NewEncoder(c.OutOrStdout())
		if !jsonOutput {
			e.SetIndent("", "  ")
		}
		return e.Encode(value)
	}
	var contextTarget string
	var contextList bool
	context := &cobra.Command{Use: "context", Short: "Discover loaded configuration, assets, and Space sources", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		v, err := authorProject(module, config)
		if err == nil && contextList {
			return report(c, listAuthorContext(v), nil)
		}
		if err == nil && contextTarget != "" {
			focused, err := focusAuthorContext(v, contextTarget)
			return report(c, focused, err)
		}
		return report(c, v, err)
	}}
	context.Flags().StringVar(&contextTarget, "target", "", "focus on an effective brick, relevant templates, and a matching page example")
	context.Flags().BoolVar(&contextList, "list", false, "list root types, routes, owning files, and page/Space sources without source bodies")
	context.MarkFlagsMutuallyExclusive("list", "target")
	var inspectTargets []string
	inspect := &cobra.Command{Use: "inspect", Short: "Inspect loaded ownership, bindings, and editing contracts without source bodies", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		if len(inspectTargets) == 0 {
			return report(c, nil, fmt.Errorf("--target is required"))
		}
		ctx, err := authorProject(module, config)
		if err != nil {
			return report(c, nil, err)
		}
		if len(inspectTargets) == 1 {
			value, err := inspectAuthorContext(ctx, inspectTargets[0])
			return report(c, value, err)
		}
		value, failed := inspectAuthorTargets(ctx, inspectTargets)
		reportErr := report(c, value, nil)
		if failed {
			Exit, ExitCode = true, 1
		}
		return reportErr
	}}
	inspect.Flags().StringArrayVar(&inspectTargets, "target", nil, "effective root or nested component to inspect; repeat for multiple targets")
	recipes := &cobra.Command{Use: "recipes", Short: "List curated authoring spec examples", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error { return report(c, authorRecipes(), nil) }}
	explain := &cobra.Command{Use: "explain-type TYPE", Short: "Describe current schema fields and authoring hints", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		def, err := scaffoldDefinition(args[0])
		if err != nil {
			return report(c, nil, err)
		}
		return report(c, schema.ExtractDefinition(def), nil)
	}}
	var specPath string
	var dry, compact, summary bool
	apply := &cobra.Command{Use: "apply", Short: "Validate and preview or apply one authoring spec", Args: cobra.NoArgs}
	apply.Flags().StringVar(&specPath, "spec", "", "JSON spec file, or - for stdin")
	apply.Flags().BoolVar(&dry, "dry-run", false, "preview without writing")
	apply.Flags().BoolVar(&compact, "compact", false, "report emitted blocks, changed files, and diagnostics without full before/after")
	apply.Flags().BoolVar(&summary, "summary", false, "report applied operations, changed files, and diagnostics without source text")
	apply.MarkFlagsMutuallyExclusive("summary", "compact")
	apply.MarkFlagsMutuallyExclusive("summary", "dry-run")
	apply.RunE = func(c *cobra.Command, _ []string) error {
		var raw []byte
		var err error
		if specPath == "-" {
			raw, err = io.ReadAll(c.InOrStdin())
		} else if specPath != "" {
			raw, err = os.ReadFile(specPath)
		} else {
			err = fmt.Errorf("--spec is required")
		}
		if err != nil {
			return scaffoldReport(c, nil, err, dry, jsonOutput)
		}
		s, err := decodeAuthor(bytes.NewReader(raw))
		if err == nil && s.Revision != "" {
			var ctx *authorContext
			ctx, err = authorProject(module, config)
			if err == nil && ctx.Revision != s.Revision {
				err = fmt.Errorf("project changed since context; discover and prepare again")
			}
		}
		if err != nil {
			return scaffoldReport(c, nil, err, dry, jsonOutput)
		}
		if s.Operations != nil {
			p, err := prepareAuthorBatch(module, config, s)
			if err == nil && !dry {
				err = p.apply()
			}
			return reportAuthorApply(c, p, err, dry, jsonOutput, compact, s, summary)
		}
		var p *scaffoldPlan
		var applyPlan func() error
		switch s.Operation {
		case "create-space":
			if s.File != "" || s.ImportInto != "" || s.Target != "" || s.Brick.Type != "" || s.Brick.Inherit != "" || len(s.Brick.Properties) != 0 || len(s.Brick.Children) != 0 || len(s.Brick.PropertyOrder) != 0 || s.Brick.Slot != "" {
				err = fmt.Errorf("create-space accepts source, brick.name, title, and route only")
				break
			}
			var sp *localSpacePlan
			sp, err = prepareLocalSpace(spaceOptions{Module: module, Config: config, Source: s.Source, Name: s.Brick.Name, Title: s.Title, Route: s.Route})
			if err == nil {
				p, applyPlan = sp.preview, sp.apply
			}
		case "add-root", "add-child":
			if s.Source != "" || s.Title != "" || s.Route != "" {
				err = fmt.Errorf("brick operations put title and route in brick.properties and do not accept source")
				break
			}
			var n *yaml.Node
			n, err = authorNode(s.Brick)
			if err != nil {
				break
			}
			if s.Operation == "add-root" {
				if s.Target != "" || s.Brick.Slot != "" {
					err = fmt.Errorf("add-root does not accept target or slot")
					break
				}
				p, err = prepareScaffoldNode(scaffoldSpec{Module: module, Config: config, Type: s.Brick.Type, Name: s.Brick.Name, File: s.File, ImportInto: s.ImportInto}, n)
			} else {
				if s.Target == "" || s.File != "" || s.ImportInto != "" {
					err = fmt.Errorf("add-child requires target and derives its file from source ownership")
					break
				}
				p, err = prepareAuthorChild(module, config, s, n)
			}
			if err == nil {
				err = validateAuthorSources(p)
			}
			if err == nil {
				applyPlan = p.apply
			}
		default:
			err = fmt.Errorf("operation must be add-root, add-child, or create-space")
		}
		if p != nil {
			p.Command = "hyperbricks author apply --spec <spec-file>"
		}
		if err == nil && !dry {
			err = applyPlan()
		}
		return reportAuthorApply(c, p, err, dry, jsonOutput, compact, s, summary)
	}
	for _, child := range []*cobra.Command{context, inspect, recipes, explain, apply} {
		authoringCommandErrors(child, &jsonOutput)
		cmd.AddCommand(child)
	}
	authoringCommandErrors(cmd, &jsonOutput)
	return cmd
}

func authorRecipes() map[string]authorSpec {
	return map[string]authorSpec{
		"inherited-shell-page": inheritedAuthorPage("scaffold_page", "hello-world.hyperbricks.yaml"),
		"markdown-page":        {Version: 2, Operation: "add-root", File: "about.hyperbricks.yaml", Brick: authorBrick{Name: "about_page", Type: "hypermedia", Properties: map[string]interface{}{"route": "about", "title": "About"}, Children: []authorBrick{{Name: "content", Type: "markdown", Properties: map[string]interface{}{"content": "# About\n\nHello.\n"}}}}},
		"html-fragment":        {Version: 2, Operation: "add-root", File: "status.hyperbricks.yaml", Brick: authorBrick{Name: "status_fragment", Type: "fragment", Properties: map[string]interface{}{"route": "status"}, Children: []authorBrick{{Name: "content", Type: "html", Properties: map[string]interface{}{"value": "<p>Ready</p>"}}}}},
		"space-source":         {Version: 2, Operation: "add-root", File: "page-source.hyperbricks.yaml", Brick: authorBrick{Name: "page_source", Type: "hypermedia", Properties: map[string]interface{}{"title": "Page source"}, Children: []authorBrick{{Name: "content", Type: "template", Properties: map[string]interface{}{"inline": "<article>{{.body}}</article>", "editable": []string{"body"}, "values": map[string]interface{}{"body": "Page content"}}}}}},
		"space-instance":       {Version: 2, Operation: "create-space", Source: "page_source", Brick: authorBrick{Name: "about"}, Title: "About", Route: "about"},
		"template-markdown":    {Version: 2, Operation: "add-root", File: "article.hyperbricks.yaml", Brick: authorBrick{Name: "article", Type: "template", Properties: map[string]interface{}{"inline": "<article>{{.body}}</article>"}, Children: []authorBrick{{Name: "body", Type: "markdown", Slot: "values", Properties: map[string]interface{}{"content": "# Article\n"}}}}},
		"append-text":          {Version: 2, Operation: "add-child", Target: "about_page", Brick: authorBrick{Name: "footer", Type: "text", Properties: map[string]interface{}{"value": "Copyright"}}},
		"ordered-tree":         {Version: 2, Operation: "add-root", File: "summary.hyperbricks.yaml", Brick: authorBrick{Name: "summary", Type: "tree", Children: []authorBrick{{Name: "heading", Type: "html", Properties: map[string]interface{}{"value": "<h2>Summary</h2>"}}, {Name: "body", Type: "text", Properties: map[string]interface{}{"value": "Summary content"}}}}},
		"section-menu":         {Version: 2, Operation: "add-root", File: "navigation.hyperbricks.yaml", Brick: authorBrick{Name: "navigation", Type: "menu", Properties: map[string]interface{}{"section": "main_navigation", "sort": "index", "item": `<a href="{{if eq .Route "index"}}/{{else}}/{{.Route}}{{end}}">{{.Title}}</a>`, "active": `<a href="{{if eq .Route "index"}}/{{else}}/{{.Route}}{{end}}" aria-current="page">{{.Title}}</a>`}}},
		"page-styles":          {Version: 2, Operation: "add-child", Target: "about_page", Brick: authorBrick{Name: "styles", Type: "css", Slot: "head", Properties: map[string]interface{}{"inline": "article { max-width: 60rem; }"}}},
	}
}
