package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/render"
	"github.com/hyperbricks/hyperbricks/pkg/renderer"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func starterAuthorModule(t *testing.T) string {
	t.Helper()
	t.Chdir(t.TempDir())
	if err := initializeModule("demo"); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("modules/demo")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestAuthorFocusedBatchAcceptance(t *testing.T) {
	root := starterAuthorModule(t)
	ctx, err := authorProject(root, "")
	if err != nil {
		t.Fatal(err)
	}
	focus, err := focusAuthorContext(ctx, "scaffold_page")
	if err != nil {
		t.Fatal(err)
	}
	if focus.Example == nil || len(focus.Templates) != 1 || !strings.Contains(focus.Templates[0]["source"], "{{.body}}") {
		t.Fatalf("missing shell/example: %+v", focus)
	}
	fullBytes, _ := json.Marshal(ctx)
	focusBytes, _ := json.Marshal(focus)
	if len(focusBytes) >= len(fullBytes) {
		t.Fatal("focused output is not smaller")
	}
	first := *focus.Example
	first.Version, first.Revision = 0, ""
	first.Brick.Name = "projects_page"
	first.Brick.Properties["route"], first.Brick.Properties["title"] = "projects", "Projects"
	first.Brick.Children[0].Children[0].Children[0].Properties["content"] = "# Projects\n\nProjects acceptance marker.\n"
	// Clone through JSON so mutations for the second page cannot alter the first.
	b, _ := json.Marshal(first)
	var second authorSpec
	if err := json.Unmarshal(b, &second); err != nil {
		t.Fatal(err)
	}
	second.Brick.Name = "launch_page"
	second.Brick.Properties["route"], second.Brick.Properties["title"], second.Brick.Properties["index"] = "launch", "Launch", 50
	second.Brick.Children[0].Children[0].Children[0].Properties["content"] = "# Launch\n\nLaunch acceptance marker.\n"
	spec := authorSpec{Version: 2, Revision: ctx.Revision, Operations: []authorSpec{first, second}}
	before := scaffoldState(t, root)
	p, err := prepareAuthorBatch(root, "", spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Files) != 1 {
		t.Fatalf("expected one combined write, got %d", len(p.Files))
	}
	if !reflect.DeepEqual(before, scaffoldState(t, root)) {
		t.Fatal("planning wrote files")
	}
	var full, compact bytes.Buffer
	c := NewAuthorCommand()
	c.SetOut(&full)
	if err := reportAuthorApply(c, p, nil, true, true, false, spec); err != nil {
		t.Fatal(err)
	}
	c.SetOut(&compact)
	if err := reportAuthorApply(c, p, nil, true, true, true, spec); err != nil {
		t.Fatal(err)
	}
	if compact.Len() >= full.Len() || strings.Contains(compact.String(), `"before"`) || strings.Contains(compact.String(), `"after"`) {
		t.Fatal("compact preview includes full files")
	}
	t.Logf("context bytes: full=%d focused=%d; preview bytes: full=%d compact=%d", len(fullBytes), len(focusBytes), full.Len(), compact.Len())
	if r := runAuthor(t, root, spec, true); r.Status != "preview" {
		t.Fatal(r.Error)
	}
	if r := runAuthor(t, root, spec, false); r.Status != "created" {
		t.Fatal(r.Error)
	}
	updated, err := authorProject(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range updated.Roots {
		if page.Name != "projects_page" && page.Name != "launch_page" {
			continue
		}
		content := page.Effective["content"].(map[string]interface{})
		if content["template"] != "shell.html" || content["values"].(map[string]interface{})["navigation"] == nil || page.Effective["head"] == nil {
			t.Fatalf("inherited shell lost: %s", page.Name)
		}
		if page.Effective["route"] != strings.TrimSuffix(page.Name, "_page") {
			t.Fatalf("missing route: %s", page.Name)
		}
		output, errs := renderAuthorShell(t, root, content, updated.Roots)
		marker := "Projects acceptance marker."
		if page.Name == "launch_page" {
			marker = "Launch acceptance marker."
		}
		if len(errs) != 0 || !strings.Contains(output, marker) || !strings.Contains(output, `id="scaffold-content"`) || !strings.Contains(output, `href="/projects"`) || !strings.Contains(output, `href="/launch"`) {
			t.Fatalf("%s failed rendering: %v\n%s", page.Name, errs, output)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			t.Fatalf("unexpected module-root artifact: %s", e.Name())
		}
	}
}

func renderAuthorShell(t *testing.T, root string, content map[string]interface{}, roots []authorRoot) (string, []error) {
	t.Helper()
	shared.Init_configuration()
	rm := render.NewRenderManager()
	rm.RegisterComponent(component.MarkdownConfigGetName(), &component.MarkdownRenderer{}, reflect.TypeOf(component.MarkdownConfig{}))
	rm.RegisterComponent(component.HTMLConfigGetName(), &component.HTMLRenderer{}, reflect.TypeOf(component.HTMLConfig{}))
	rm.RegisterComponent(composite.TreeRendererConfigGetName(), &composite.TreeRenderer{CompositeRenderer: renderer.CompositeRenderer{RenderManager: rm}}, reflect.TypeOf(composite.TreeConfig{}))
	sections := map[string][]composite.HyperMediaConfig{}
	for _, page := range roots {
		if page.Effective["@type"] != composite.HyperMediaConfigGetName() {
			continue
		}
		section, _ := page.Effective["section"].(string)
		route, _ := page.Effective["route"].(string)
		title, _ := page.Effective["title"].(string)
		index, _ := page.Effective["index"].(int)
		if section != "" && route != "" {
			sections[section] = append(sections[section], composite.HyperMediaConfig{Title: title, Route: route, Section: section, Index: index})
		}
	}
	rm.RegisterComponent(component.MenuConfigGetName(), &component.MenuRenderer{HyperMediasBySection: sections}, reflect.TypeOf(component.MenuConfig{}))
	provider := func(name string) (string, bool) {
		b, err := os.ReadFile(filepath.Join(root, "templates", name))
		return string(b), err == nil
	}
	rm.RegisterComponent(composite.TemplateConfigGetName(), &composite.TemplateRenderer{CompositeRenderer: renderer.CompositeRenderer{RenderManager: rm, TemplateProvider: provider}}, reflect.TypeOf(composite.TemplateConfig{}))
	return rm.Render(composite.TemplateConfigGetName(), content, context.Background())
}

func TestAuthorBatchRejectsWithoutWrites(t *testing.T) {
	root := scaffoldFixtureModule(t)
	first := authorRecipes()["markdown-page"]
	first.Version = 0
	second := first
	second.Brick.Name = "second_page"
	s := authorSpec{Version: 2, Operations: []authorSpec{first, second}}
	before := scaffoldState(t, root)
	if r := runAuthor(t, root, s, false); r.Status != "error" || !strings.Contains(r.Error, "operations[1]") {
		t.Fatalf("duplicate route accepted: %+v", r)
	}
	if !reflect.DeepEqual(before, scaffoldState(t, root)) {
		t.Fatal("failed batch wrote files")
	}
	s.Operations = []authorSpec{}
	if r := runAuthor(t, root, s, false); r.Status != "error" {
		t.Fatal("empty batch accepted")
	}
	s.Operations = []authorSpec{first}
	s.Operation = "add-root"
	if r := runAuthor(t, root, s, false); r.Status != "error" {
		t.Fatal("mixed envelope accepted")
	}
}

func TestAuthorBatchCanAttachToPlannedRoot(t *testing.T) {
	root := scaffoldFixtureModule(t)
	first := authorRecipes()["markdown-page"]
	first.Version = 0
	second := authorRecipes()["append-text"]
	second.Version = 0
	s := authorSpec{Version: 2, Operations: []authorSpec{first, second}}
	if r := runAuthor(t, root, s, false); r.Status != "created" || len(r.Plan.Files) != 1 {
		t.Fatalf("failed planned child: %+v", r)
	}
}

func TestAuthorWarnings(t *testing.T) {
	s := authorSpec{Brick: authorBrick{Properties: map[string]interface{}{"values": map[string]interface{}{"body": map[string]interface{}{"type": "tree", "children": []interface{}{}}, "valid_data": map[string]interface{}{"type": "product", "inherit": "catalog"}}}}}
	d := authorWarnings(s)
	if len(d) != 1 || d[0].Path != "brick.properties.values.body" || d[0].Code != "brick_emitted_as_data" {
		t.Fatalf("unexpected warnings: %+v", d)
	}
}

func TestAuthorBatchNestedImportSummary(t *testing.T) {
	root := scaffoldFixtureModule(t)
	first := authorRecipes()["markdown-page"]
	first.Version, first.File, first.ImportInto = 0, "partials/pages.hyperbricks.yaml", "app.hyperbricks.yaml"
	b, _ := json.Marshal(first)
	var second authorSpec
	if err := json.Unmarshal(b, &second); err != nil {
		t.Fatal(err)
	}
	second.Brick.Name = "contact_page"
	second.Brick.Properties["route"] = "contact"
	p, err := prepareAuthorBatch(root, "", authorSpec{Operations: []authorSpec{first, second}})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Files) != 2 {
		t.Fatalf("expected leaves and parent, got %d files", len(p.Files))
	}
	var added []string
	for _, f := range p.Files {
		added = append(added, summarizeAuthorFile(f).ImportsAdded...)
	}
	if !reflect.DeepEqual(added, []string{"partials/pages.hyperbricks.yaml"}) {
		t.Fatalf("incorrect import changes: %v", added)
	}
	if err := p.apply(); err != nil {
		t.Fatal(err)
	}
}

func TestAuthorDiscoveryAndUnknownTarget(t *testing.T) {
	root := starterAuthorModule(t)
	ctx, err := authorProject(root, "")
	if err != nil {
		t.Fatal(err)
	}
	before := scaffoldState(t, root)
	cmd := NewAuthorCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"context", "-m", root, "--list", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var discovery authorDiscovery
	if err := json.Unmarshal(out.Bytes(), &discovery); err != nil {
		t.Fatal(err)
	}
	if discovery.Revision != ctx.Revision || len(discovery.Roots) != len(ctx.Roots) {
		t.Fatalf("incomplete discovery: %+v", discovery)
	}
	found := false
	sources := map[string]bool{}
	for _, source := range ctx.SpaceSources {
		sources[source.Name] = true
	}
	for _, r := range discovery.Roots {
		if r.SpaceSource != sources[r.Name] {
			t.Fatalf("incorrect Space discovery: %+v", r)
		}
		if r.Name == "scaffold_page" {
			found = r.Type == "hypermedia" && r.PageSource && r.File != ""
		}
	}
	fixture, err := authorProject(scaffoldFixtureModule(t), "")
	if err != nil {
		t.Fatal(err)
	}
	listed := listAuthorContext(fixture)
	if len(listed.Roots) != 1 || !listed.Roots[0].SpaceSource || !listed.Roots[0].PageSource {
		t.Fatalf("editable reusable page source missing: %+v", listed)
	}
	if !found {
		t.Fatal("shared page source missing")
	}
	full, _ := json.Marshal(ctx)
	if out.Len() >= len(full) || strings.Contains(out.String(), `"effective"`) || strings.Contains(out.String(), `"yaml"`) {
		t.Fatal("discovery includes source bodies")
	}
	t.Logf("discovery bytes=%d full context bytes=%d", out.Len(), len(full))
	for _, target := range []string{"site_shell", "scaffold_page.missing"} {
		if _, err := focusAuthorContext(ctx, target); err == nil || !strings.Contains(err.Error(), "scaffold_page") || !strings.Contains(err.Error(), "--list --json") {
			t.Fatalf("unhelpful target error: %v", err)
		}
	}
	cmd = NewAuthorCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"context", "-m", root, "--list", "--target", "scaffold_page"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("mutually exclusive views accepted")
	}
	if !reflect.DeepEqual(before, scaffoldState(t, root)) {
		t.Fatal("discovery wrote files")
	}
}

func TestAuthorCustomImportHandoff(t *testing.T) {
	root := scaffoldFixtureModule(t)
	scaffoldWrite(t, filepath.Join(root, "source", "app.hyperbricks.yaml"), "imports: [layouts/site.hyperbricks.yaml]\n")
	scaffoldWrite(t, filepath.Join(root, "source", "layouts", "site.hyperbricks.yaml"), "coastal_shell:\n  - type: hypermedia\n  - content:\n      - type: template\n      - template: {file: coastal.html}\n      - values: {body: Welcome}\n")
	scaffoldWrite(t, filepath.Join(root, "views", "coastal.html"), "<main>{{.body}}</main>")
	ctx, err := authorProject(root, "")
	if err != nil {
		t.Fatal(err)
	}
	focus, err := focusAuthorContext(ctx, "coastal_shell")
	if err != nil {
		t.Fatal(err)
	}
	if focus.Example == nil || focus.Example.File != "app.hyperbricks.yaml" || len(focus.Templates) != 1 {
		t.Fatalf("custom scope handoff failed: %+v", focus)
	}
	spec := *focus.Example
	spec.File, spec.ImportInto = "pages/journal.hyperbricks.yaml", "app.hyperbricks.yaml"
	spec.Brick.Name = "journal"
	spec.Brick.Properties["route"] = "journal"
	if r := runAuthor(t, root, spec, false); r.Status != "created" {
		t.Fatal(r.Error)
	}
	updated, err := authorProject(root, "")
	if err != nil {
		t.Fatal(err)
	}
	page, err := focusAuthorContext(updated, "journal")
	if err != nil || page.Effective["content"].(map[string]interface{})["template"] != "coastal.html" {
		t.Fatalf("custom inheritance lost: %v", err)
	}
}

func TestAuthorMissingAndInvalidTemplates(t *testing.T) {
	for _, body := range []string{"", "{{if}}"} {
		t.Run(map[bool]string{true: "missing", false: "invalid"}[body == ""], func(t *testing.T) {
			root := scaffoldFixtureModule(t)
			if body != "" {
				scaffoldWrite(t, filepath.Join(root, "views", "page.html"), body)
			}
			spec := authorSpec{Version: 2, Operation: "add-root", File: "app.hyperbricks.yaml", Brick: authorBrick{Name: "page_body", Type: "template", Properties: map[string]interface{}{"template": map[string]interface{}{"file": "page.html"}}}}
			before := scaffoldState(t, root)
			for _, dry := range []bool{true, false} {
				if r := runAuthor(t, root, spec, dry); r.Status != "error" || !strings.Contains(r.Error, "template") {
					t.Fatalf("invalid template accepted: %+v", r)
				}
			}
			if !reflect.DeepEqual(before, scaffoldState(t, root)) {
				t.Fatal("template failure wrote files")
			}
		})
	}
}

func editableAuthorModule(t *testing.T, declaration string) string {
	t.Helper()
	root := scaffoldFixtureModule(t)
	scaffoldWrite(t, filepath.Join(root, "source", "app.hyperbricks.yaml"), "base:\n  - type: hypermedia\n  - content:\n      - type: template\n      - inline: |\n          <main>{{.body}}<p>{{.caption}}</p></main>\n      - values: {body: Plan your coastal weekend., caption: Coastal journal}\n      - editable: "+declaration+"\n")
	return root
}

func TestAuthorEditableExamplePreservesContract(t *testing.T) {
	for _, declaration := range []string{"[body, caption]", "{body: {type: textarea, max: 1000}, caption: text}"} {
		t.Run(declaration, func(t *testing.T) {
			root := editableAuthorModule(t, declaration)
			ctx, err := authorProject(root, "")
			if err != nil {
				t.Fatal(err)
			}
			focus, err := focusAuthorContext(ctx, "base")
			if err != nil || focus.Example == nil {
				t.Fatalf("missing example: %v", err)
			}
			content := focus.Example.Brick.Children[0]
			if len(content.Children) != 0 || content.Properties["values"].(map[string]interface{})["body"] != "Plan your coastal weekend." {
				t.Fatalf("example replaced scalar editing contract: %+v", content)
			}
			if r := runAuthor(t, root, *focus.Example, false); r.Status != "created" {
				t.Fatal(r.Error)
			}
			updated, err := authorProject(root, "")
			if err != nil {
				t.Fatal(err)
			}
			page, err := focusAuthorContext(updated, focus.Example.Brick.Name)
			if err != nil {
				t.Fatal(err)
			}
			original := focus.Effective["content"].(map[string]interface{})["editable"]
			actual := page.Effective["content"].(map[string]interface{})["editable"]
			if !reflect.DeepEqual(original, actual) {
				t.Fatalf("editable metadata changed: %v -> %v", original, actual)
			}
		})
	}
}

func TestAuthorRejectsEditableComponentBeforeWrites(t *testing.T) {
	for _, operation := range []string{"single-root", "batch", "child"} {
		t.Run(operation, func(t *testing.T) {
			root := editableAuthorModule(t, "[body, caption]")
			op := inheritedAuthorPage("base", "app.hyperbricks.yaml")
			spec := op
			if operation == "batch" {
				op.Version = 0
				spec = authorSpec{Version: 2, Operations: []authorSpec{op}}
			}
			if operation == "child" {
				spec = authorSpec{Version: 2, Operation: "add-child", Target: "base.content", Brick: authorBrick{Name: "extra", Type: "template", Slot: "values", Properties: map[string]interface{}{"inline": "<p>{{.copy}}</p>", "editable": []interface{}{"copy"}}, Children: []authorBrick{{Name: "copy", Type: "markdown", Slot: "values", Properties: map[string]interface{}{"content": "## Coastal notes\n"}}}}}
			}
			before := scaffoldState(t, root)
			for _, dry := range []bool{true, false} {
				if r := runAuthor(t, root, spec, dry); r.Status != "error" || !strings.Contains(r.Error, "scalar") || !strings.Contains(r.Error, "source") {
					t.Fatalf("editable component accepted: %+v", r)
				}
			}
			if !reflect.DeepEqual(before, scaffoldState(t, root)) {
				t.Fatal("editing-contract failure wrote files")
			}
			if _, err := authorProject(root, ""); err != nil {
				t.Fatalf("rejected plan broke discovery: %v", err)
			}
			if operation == "single-root" {
				spec.Brick.Children[0].Properties = map[string]interface{}{"editable": []interface{}{}}
				if r := runAuthor(t, root, spec, false); r.Status != "created" {
					t.Fatalf("explicit contract override rejected: %+v", r)
				}
				if _, err := authorProject(root, ""); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
