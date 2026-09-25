package commands

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

func runAuthor(t *testing.T, root string, spec authorSpec, dry bool) scaffoldResult {
	t.Helper()
	b, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	cmd := NewAuthorCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(bytes.NewReader(b))
	args := []string{"apply", "-m", root, "--spec", "-", "--json"}
	if dry {
		args = append(args, "--dry-run")
	}
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result scaffoldResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("%s: %v", out.String(), err)
	}
	return result
}

func TestAuthorCommandIdentity(t *testing.T) {
	cmd := NewAuthorCommand()
	if cmd.Name() != "author" || len(cmd.Aliases) != 0 {
		t.Fatalf("unexpected author command identity: name=%q aliases=%v", cmd.Name(), cmd.Aliases)
	}
}

func TestAuthorContextAndStaleSpec(t *testing.T) {
	root := scaffoldFixtureModule(t)
	scaffoldWrite(t, filepath.Join(root, "source", "app.hyperbricks.yaml"), "imports: [partials/shared.hyperbricks.yaml]\n"+scaffoldFixture)
	scaffoldWrite(t, filepath.Join(root, "source", "partials", "shared.hyperbricks.yaml"), "shared:\n  - type: text\n  - value: Shared\n")
	scaffoldWrite(t, filepath.Join(root, "views", "shell.html"), "<main>{{.body}}</main>")
	before := scaffoldState(t, root)
	ctx, err := authorProject(root, "")
	if err != nil {
		t.Fatal(err)
	}
	again, err := authorProject(root, "")
	if err != nil || ctx.Revision != again.Revision {
		t.Fatalf("unstable context: %v", err)
	}
	if len(ctx.Files) != 2 || len(ctx.Roots) != 2 || len(ctx.SpaceSources) != 1 || len(ctx.Assets["templates"]) != 1 {
		t.Fatalf("incomplete context: %+v", ctx)
	}
	if !reflect.DeepEqual(before, scaffoldState(t, root)) {
		t.Fatal("context wrote files")
	}
	s := authorRecipes()["markdown-page"]
	s.Revision = ctx.Revision
	preview := runAuthor(t, root, s, true)
	if preview.Status != "preview" {
		t.Fatal(preview.Error)
	}
	if !reflect.DeepEqual(before, scaffoldState(t, root)) {
		t.Fatal("preview wrote files")
	}
	scaffoldWrite(t, filepath.Join(root, "views", "shell.html"), "changed")
	beforeStale := scaffoldState(t, root)
	if got := runAuthor(t, root, s, false); got.Status != "error" || !strings.Contains(got.Error, "project changed") {
		t.Fatalf("stale spec accepted: %+v", got)
	}
	if !reflect.DeepEqual(beforeStale, scaffoldState(t, root)) {
		t.Fatal("stale spec wrote files")
	}
}

func TestAuthorRootChildrenAndSpace(t *testing.T) {
	root := scaffoldFixtureModule(t)
	s := authorRecipes()["markdown-page"]
	s.File = "partials/about.hyperbricks.yaml"
	s.ImportInto = "app.hyperbricks.yaml"
	if r := runAuthor(t, root, s, false); r.Status != "created" {
		t.Fatal(r.Error)
	}
	parent := scaffoldRead(t, filepath.Join(root, "source", "app.hyperbricks.yaml"))
	if !strings.Contains(parent, "# Keep this source comment") || !strings.Contains(parent, "partials/about.hyperbricks.yaml") {
		t.Fatal(parent)
	}
	page := scaffoldRead(t, filepath.Join(root, "source", s.File))
	if strings.Index(page, "route:") > strings.Index(page, "title:") || !strings.Contains(page, "content: |") {
		t.Fatal(page)
	}
	styles := authorRecipes()["page-styles"]
	if r := runAuthor(t, root, styles, false); r.Status != "created" {
		t.Fatal(r.Error)
	}
	if r := runAuthor(t, root, styles, false); r.Status != "error" {
		t.Fatal("duplicate child accepted")
	}
	child := authorSpec{Version: 2, Operation: "add-child", Target: "base.content", Brick: authorBrick{Name: "article", Type: "markdown", Slot: "values", Properties: map[string]interface{}{"content": "# Article\n"}}}
	if r := runAuthor(t, root, child, false); r.Status != "created" {
		t.Fatal(r.Error)
	}
	doc, err := yamlparser.ParseBytes([]byte(scaffoldRead(t, filepath.Join(root, "source", "app.hyperbricks.yaml"))))
	if err != nil {
		t.Fatal(err)
	}
	values, err := doc.Materialize()
	if err != nil {
		t.Fatal(err)
	}
	base := values["base"].(map[string]interface{})
	content := base["content"].(map[string]interface{})
	if content["values"].(map[string]interface{})["article"] == nil {
		t.Fatal("child not mounted in values")
	}
	source := authorRecipes()["space-source"]
	if r := runAuthor(t, root, source, false); r.Status != "created" {
		t.Fatal(r.Error)
	}
	instance := authorRecipes()["space-instance"]
	instance.Route = "contact"
	instance.Brick.Name = "contact"
	if r := runAuthor(t, root, instance, true); r.Status != "preview" || len(r.Plan.Files) == 0 {
		t.Fatalf("Space preview failed: %+v", r)
	}
	if r := runAuthor(t, root, instance, false); r.Status != "created" {
		t.Fatal(r.Error)
	}
}

func TestAuthorRejectsInvalidSpecs(t *testing.T) {
	for _, raw := range []string{`{"version":1}`, `{"version":2,"unknown":true}`, `{"version":2,"brick":{"typo":1}}`, `{"version":2} {}`} {
		if _, err := decodeAuthor(strings.NewReader(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	root := scaffoldFixtureModule(t)
	page := authorRecipes()["markdown-page"]
	page.Brick.Properties["route"] = "index"
	scaffoldWrite(t, filepath.Join(root, "source", "home.hyperbricks.yaml"), "home:\n  - type: hypermedia\n  - route: index\n")
	before := scaffoldState(t, root)
	if r := runAuthor(t, root, page, false); r.Status != "error" || !strings.Contains(r.Error, "home.hyperbricks.yaml") || !strings.Contains(r.Error, "do not replace") {
		t.Fatalf("duplicate route accepted: %+v", r)
	}
	bad := authorSpec{Version: 2, Operation: "add-child", Target: "base.missing", Brick: authorBrick{Name: "child", Type: "text", Properties: map[string]interface{}{"value": "hello"}}}
	if r := runAuthor(t, root, bad, false); r.Status != "error" {
		t.Fatal("missing target accepted")
	}
	bad.Target = "base.content"
	bad.Brick.Slot = "head"
	if r := runAuthor(t, root, bad, false); r.Status != "error" {
		t.Fatal("invalid slot accepted")
	}
	bad.Operation, bad.File = "add-root", "../escape.hyperbricks.yaml"
	bad.Target, bad.Brick.Slot = "", ""
	if r := runAuthor(t, root, bad, false); r.Status != "error" {
		t.Fatal("escaping path accepted")
	}
	if !reflect.DeepEqual(before, scaffoldState(t, root)) {
		t.Fatal("failed operations wrote files")
	}
}

func TestAuthorRootRecipes(t *testing.T) {
	for name, spec := range authorRecipes() {
		if spec.Operation != "add-root" {
			continue
		}
		if name == "inherited-shell-page" {
			continue
		} // Exercised against init's shell by the batch acceptance test.
		t.Run(name, func(t *testing.T) {
			root := scaffoldFixtureModule(t)
			result := runAuthor(t, root, spec, false)
			if result.Status != "created" {
				t.Fatal(result.Error)
			}
		})
	}
}

func TestAuthorInheritAndGuardedApply(t *testing.T) {
	root := scaffoldFixtureModule(t)
	b := authorBrick{Name: "derived", Type: "hypermedia", Inherit: "base", Properties: map[string]interface{}{"route": "derived", "title": "Derived"}, PropertyOrder: []string{"title", "route"}}
	n, err := authorNode(b)
	if err != nil {
		t.Fatal(err)
	}
	p, err := prepareScaffoldNode(scaffoldSpec{Module: root, Type: b.Type, Name: b.Name, File: "app.hyperbricks.yaml"}, n)
	if err != nil {
		t.Fatal(err)
	}
	scaffoldWrite(t, filepath.Join(root, "source", "app.hyperbricks.yaml"), scaffoldFixture+"# changed\n")
	if err := p.apply(); err == nil {
		t.Fatal("changed source accepted")
	}
	b.PropertyOrder = []string{"title", "title"}
	if _, err := authorNode(b); err == nil {
		t.Fatal("duplicate order accepted")
	}
}
