package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
	"go.yaml.in/yaml/v4"
)

const scaffoldFixture = `# Keep this source comment
base:
  - type: hypermedia
  - title: Base
  - content:
      - type: template
      - inline: '<h1>{{.heading}}</h1>'
      - values:
          heading: Welcome
      - editable: [heading]
`

func scaffoldWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}
func scaffoldFixtureModule(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	scaffoldWrite(t, filepath.Join(root, PackageConfigFileName), "hyperbricks:\n  mode: development\n  directories:\n    hyperbricks: {path: {base: module, path: source}}\n    templates: {path: {base: module, path: views}}\n    resources: {path: {base: module, path: resources}}\n    static: {path: {base: module, path: static}}\n")
	scaffoldWrite(t, filepath.Join(root, "source", "app.hyperbricks.yaml"), scaffoldFixture)
	return root
}
func scaffoldRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func scaffoldState(t *testing.T, root string) map[string]string {
	t.Helper()
	state := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			if d.Type()&os.ModeSymlink != 0 {
				target, err := os.Readlink(path)
				if err != nil {
					return err
				}
				state[path] = "symlink:" + target
			} else {
				state[path] = scaffoldRead(t, path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestScaffoldAppendAndNestedTemplate(t *testing.T) {
	root := scaffoldFixtureModule(t)
	before := scaffoldState(t, root)
	s := scaffoldSpec{Module: root, File: "partials/cards.hyperbricks.yaml", ImportInto: "app.hyperbricks.yaml", Type: "template", Name: "card", Template: "cards/card.html", TemplateMode: "create", Children: `[{"body":[{"type":"markdown"},{"content":"**Hello**"}]}]`}
	p, err := prepareScaffold(s)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, scaffoldState(t, root)) {
		t.Fatal("preview changed files")
	}
	if len(p.Files) != 3 || p.Files[0].Path != filepath.Join(root, "views", "cards", "card.html") {
		t.Fatalf("bad file order: %+v", p.Files)
	}
	if err := p.apply(); err != nil {
		t.Fatal(err)
	}
	source := scaffoldRead(t, filepath.Join(root, "source", "app.hyperbricks.yaml"))
	if !strings.Contains(source, "# Keep this source comment") || !strings.Contains(source, "partials/cards.hyperbricks.yaml") {
		t.Fatal(source)
	}
	m, _ := loadAuthoringModule(root, "")
	result, err := yamlparser.ProcessFile(filepath.Join(root, "source", "app.hyperbricks.yaml"), m.options())
	if err != nil {
		t.Fatal(err)
	}
	card := result.Materialized["card"].(map[string]interface{})
	if card["template"] != "cards/card.html" {
		t.Fatal(card)
	}
	body := card["values"].(map[string]interface{})["body"].(map[string]interface{})
	if body["@type"] != "<MARKDOWN>" {
		t.Fatal(body)
	}
	if !strings.Contains(scaffoldRead(t, p.Files[0].Path), "{{.body}}") {
		t.Fatal("template lost child binding")
	}
	p, err = prepareScaffold(scaffoldSpec{Module: root, File: "partials/cards.hyperbricks.yaml", Type: "text", Name: "caption"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Files) != 1 {
		t.Fatal("loaded file gained unnecessary import")
	}
	if err = p.apply(); err != nil {
		t.Fatal(err)
	}
	if err = p.apply(); err == nil {
		t.Fatal("plan could be applied twice")
	}
}

func TestScaffoldRootsAllCategoriesAndDefaults(t *testing.T) {
	for _, kind := range []string{"hypermedia", "fragment", "tree", "head", "template", "text", "html", "markdown", "css", "javascript", "goja_render"} {
		t.Run(kind, func(t *testing.T) {
			root := scaffoldFixtureModule(t)
			p, err := prepareScaffold(scaffoldSpec{Module: root, File: "new.hyperbricks.yaml", Name: "example", Type: kind})
			if err != nil {
				t.Fatal(err)
			}
			if err = p.apply(); err != nil {
				t.Fatal(err)
			}
			result, err := yamlparser.ProcessFile(filepath.Join(root, "source", "new.hyperbricks.yaml"), p.module.options())
			if err != nil {
				t.Fatal(err)
			}
			if _, route := result.Materialized["example"].(map[string]interface{})["route"]; route {
				t.Fatal("invented route")
			}
		})
	}
	for _, def := range scaffoldTypes("composite") {
		if !strings.HasSuffix(def.ConfigType.PkgPath(), "/composite") {
			t.Fatal(def)
		}
	}
	for _, def := range scaffoldTypes("component") {
		if strings.HasSuffix(def.ConfigType.PkgPath(), "/composite") {
			t.Fatal(def)
		}
	}
}

func TestScaffoldInvalidPlansDoNotWrite(t *testing.T) {
	cases := []struct {
		name string
		edit func(*scaffoldSpec)
	}{
		{"name", func(s *scaffoldSpec) { s.Name = "base" }},
		{"reserved-name", func(s *scaffoldSpec) { s.Name = "imports" }},
		{"type", func(s *scaffoldSpec) { s.Type = "space" }},
		{"category", func(s *scaffoldSpec) { s.Category = "component"; s.Type = "tree" }},
		{"destination", func(s *scaffoldSpec) { s.File = "../escape.hyperbricks.yaml" }},
		{"package", func(s *scaffoldSpec) { s.File = PackageConfigFileName }},
		{"managed-index", func(s *scaffoldSpec) { s.File = "spaces/base/index.hyperbricks.yaml" }},
		{"nested-unloaded", func(s *scaffoldSpec) { s.File = "partials/new.hyperbricks.yaml" }},
		{"missing-parent", func(s *scaffoldSpec) { s.ImportInto = "missing.hyperbricks.yaml" }},
		{"self-import", func(s *scaffoldSpec) { s.ImportInto = s.File }},
		{"template-mode", func(s *scaffoldSpec) { s.Type = "template"; s.Template = "new.html" }},
		{"missing-template", func(s *scaffoldSpec) { s.Type = "template"; s.Template = "missing.html"; s.TemplateMode = "reuse" }},
		{"children-leaf", func(s *scaffoldSpec) { s.Children = `[{"child":[{"type":"text"},{"value":"Hello"}]}]` }},
		{"children-type", func(s *scaffoldSpec) { s.Type = "tree"; s.Children = `[{"child":[{"value":"Hello"}]}]` }},
		{"properties", func(s *scaffoldSpec) { s.Properties = `{"unknown":"value"}` }},
		{"invalid-markdown", func(s *scaffoldSpec) { s.Type = "markdown"; s.Properties = `{"content":"text","file":"file.md"}` }},
		{"missing-markdown", func(s *scaffoldSpec) { s.Type = "markdown"; s.Properties = `{"file":"missing.md"}` }},
		{"api-endpoint", func(s *scaffoldSpec) { s.Type = "api_render" }},
		{"route-leaf", func(s *scaffoldSpec) { s.Route = "hello" }},
		{"reserved-route", func(s *scaffoldSpec) { s.Type = "hypermedia"; s.Route = "__hyperbricks/spaces" }},
		{"escape-route", func(s *scaffoldSpec) { s.Type = "fragment"; s.Route = "../other" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffoldFixtureModule(t)
			before := scaffoldState(t, root)
			s := scaffoldSpec{Module: root, File: "new.hyperbricks.yaml", Name: "new_root", Type: "text"}
			tc.edit(&s)
			if _, err := prepareScaffold(s); err == nil {
				t.Fatal("accepted invalid plan")
			}
			if !reflect.DeepEqual(before, scaffoldState(t, root)) {
				t.Fatal("invalid plan changed files")
			}
		})
	}
}

func TestScaffoldSourceConflictsAndContainment(t *testing.T) {
	for _, change := range []string{"owner", "package", "new-top", "new-template", "symlink"} {
		t.Run(change, func(t *testing.T) {
			root := scaffoldFixtureModule(t)
			s := scaffoldSpec{Module: root, File: "new.hyperbricks.yaml", Name: "new_root", Type: "template", Template: "new.html", TemplateMode: "create"}
			p, err := prepareScaffold(s)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "owner":
				scaffoldWrite(t, filepath.Join(root, "source", "app.hyperbricks.yaml"), scaffoldFixture+"# external\n")
			case "package":
				scaffoldWrite(t, filepath.Join(root, PackageConfigFileName), "hyperbricks: {}\n")
			case "new-top":
				scaffoldWrite(t, filepath.Join(root, "source", "external.hyperbricks.yaml"), "other:\n  - type: text\n  - value: External\n")
			case "new-template":
				scaffoldWrite(t, filepath.Join(root, "views", "new.html"), "external template")
			case "symlink":
				if err := os.Symlink(t.TempDir(), filepath.Join(root, "views")); err != nil {
					t.Fatal(err)
				}
			}
			before := scaffoldState(t, root)
			if err = p.apply(); err == nil {
				t.Fatal("accepted stale/unsafe plan")
			}
			if !reflect.DeepEqual(before, scaffoldState(t, root)) {
				t.Fatal("failed preflight changed files")
			}
		})
	}
}

func TestScaffoldExistingFilePreservesOrderAndRejectsDuplicateRoute(t *testing.T) {
	root := scaffoldFixtureModule(t)
	file := filepath.Join(root, "source", "app.hyperbricks.yaml")
	scaffoldWrite(t, file, scaffoldFixture+"home:\n  - type: hypermedia\n  - route: index\n")
	if _, err := prepareScaffold(scaffoldSpec{Module: root, File: "new.hyperbricks.yaml", Type: "fragment", Name: "other", Route: "index"}); err == nil {
		t.Fatal("duplicate route accepted")
	}
	p, err := prepareScaffold(scaffoldSpec{Module: root, File: "app.hyperbricks.yaml", Type: "text", Name: "footer", Properties: `{"value":"Copyright"}`})
	if err != nil {
		t.Fatal(err)
	}
	if err = p.apply(); err != nil {
		t.Fatal(err)
	}
	doc, err := yamlparser.ParseBytes([]byte(scaffoldRead(t, file)))
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, n := range doc.Roots {
		names = append(names, n.Name)
	}
	if !reflect.DeepEqual(names, []string{"base", "home", "footer"}) {
		t.Fatal(names)
	}
}

func TestScaffoldWizardCategoriesAndNavigation(t *testing.T) {
	root := scaffoldFixtureModule(t)
	initial := scaffoldSpec{Module: root}
	v := map[string]string{"category": "composite", "type": "template_inline", "file-choice": "@new", "file": "new.hyperbricks.yaml", "name": "card"}
	steps := scaffoldWizardSteps(initial, v)
	if steps[0].key != "category" || steps[1].key != "type" {
		t.Fatal("category is not first")
	}
	for _, opt := range steps[1].options {
		templates, _ := scaffoldLibraryTemplates()
		def, _ := scaffoldDefinition(templates[opt.value].Type)
		if scaffoldCategory(def) != "composite" {
			t.Fatal(opt)
		}
	}
	for _, step := range steps {
		switch step.key {
		case "import-into", "properties", "template-mode", "template", "children-choice", "children":
			t.Fatalf("removed wizard step still present: %s", step.key)
		}
	}
	s := wizardScaffoldSpec(initial, v)
	if _, err := prepareScaffoldLibrary(s); err != nil {
		t.Fatal(err)
	}
	m := newAuthoringWizard(func(v map[string]string) []wizardStep { return scaffoldWizardSteps(initial, v) }, func(map[string]string) (interface{}, string, error) { return nil, "", nil })
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !m.canceled {
		t.Fatal("cancel failed")
	}
	m = newAuthoringWizard(func(v map[string]string) []wizardStep { return scaffoldWizardSteps(initial, v) }, func(map[string]string) (interface{}, string, error) { return nil, "", nil })
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.values["category"] != "composite" || m.current().key != "type" {
		t.Fatal("category navigation failed")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.current().key != "category" {
		t.Fatal("back navigation failed")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.canceled || m.current().key != "category" {
		t.Fatal("back navigation at the first step canceled the wizard")
	}
}

func TestScaffoldWizardKeepsValidSelectionsAndDropsStaleOnes(t *testing.T) {
	m := newAuthoringWizard(func(map[string]string) []wizardStep {
		return []wizardStep{{key: "module", kind: "select", options: []wizardOption{{"current", "current", ""}}}}
	}, nil)
	m.values["module"] = "deleted"
	m.enter()
	if _, exists := m.values["module"]; exists || m.value() != "current" {
		t.Fatalf("stale selection was retained: %#v", m.values)
	}
	m.values["module"] = "current"
	m.enter()
	if m.value() != "current" {
		t.Fatal("valid selection was not retained")
	}
}

func TestScaffoldWizardOffersInlineAndFileStarters(t *testing.T) {
	options := []wizardOption{}
	for _, category := range []string{"composite", "component"} {
		steps := scaffoldWizardSteps(scaffoldSpec{Module: scaffoldFixtureModule(t)}, map[string]string{"category": category})
		for _, step := range steps {
			if step.key == "type" {
				options = append(options, step.options...)
			}
		}
	}
	for _, want := range []string{"template (inline)", "template (file)", "markdown (inline)", "markdown (file)"} {
		found := false
		for _, option := range options {
			if option.label == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing type starter %q: %#v", want, options)
		}
	}
	for starter, wantType := range map[string]string{"template_inline": "template", "template_file": "template", "markdown_inline": "markdown", "markdown_file": "markdown"} {
		s := wizardScaffoldSpec(scaffoldSpec{Module: scaffoldFixtureModule(t)}, map[string]string{"type": starter})
		if s.Type != wantType {
			t.Fatalf("starter %s resolves to %q, want %q", starter, s.Type, wantType)
		}
	}
}

func TestScaffoldWizardNamesBundledStartersWithTheirAssetToken(t *testing.T) {
	root := scaffoldFixtureModule(t)
	for starter, want := range map[string]struct{ category, name string }{
		"image":         {"component", "image_192a"},
		"template_file": {"composite", "template_2sa5"},
		"markdown_file": {"component", "markdown_3b7e"},
	} {
		for _, step := range scaffoldWizardSteps(scaffoldSpec{Module: root}, map[string]string{"category": want.category, "type": starter, "file-choice": "app.hyperbricks.yaml"}) {
			if step.key == "name" && step.initial != want.name {
				t.Fatalf("starter %s suggested %q, want %q", starter, step.initial, want.name)
			}
		}
	}
}

func TestSpaceCLIUsesHypermediaInheritance(t *testing.T) {
	root := scaffoldFixtureModule(t)
	o := spaceOptions{Module: root, Source: "base", Name: "first", Title: "First", Route: "first"}
	before := scaffoldState(t, root)
	p, err := prepareLocalSpace(o)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, scaffoldState(t, root)) {
		t.Fatal("space preview wrote files")
	}
	if err = p.apply(); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "source", "spaces", "base", "first.hyperbricks.yaml")
	b := scaffoldRead(t, file)
	if !strings.Contains(b, "inherit: base") || !strings.Contains(b, "heading: Welcome") {
		t.Fatal(b)
	}
	m, _ := loadAuthoringModule(root, "")
	result, err := yamlparser.ProcessFile(filepath.Join(root, "source", "app.hyperbricks.yaml"), m.options())
	if err != nil {
		t.Fatal(err)
	}
	first := result.Materialized["first"].(map[string]interface{})
	if first["@type"] != "<HYPERMEDIA>" || first["route"] != "first" {
		t.Fatal(first)
	}
	if _, err = prepareLocalSpace(o); err == nil {
		t.Fatal("duplicate space accepted")
	}
}

func TestScaffoldFileBackedTypes(t *testing.T) {
	root := scaffoldFixtureModule(t)
	for file, body := range map[string]string{"data.json": `{"title":"Hello"}`, "app.css": "body {}", "app.js": "console.log('hello')", "photo.png": "fixture", "document.md": "# Hello"} {
		scaffoldWrite(t, filepath.Join(root, "resources", file), body)
	}
	path := func(file string) map[string]interface{} {
		return map[string]interface{}{"path": map[string]interface{}{"base": "resources", "path": file}}
	}
	props := map[string]map[string]interface{}{
		"api_render":          {"endpoint": "https://example.com/api"},
		"api_fragment_render": {"endpoint": "https://example.com/api"},
		"json_render":         {"file": path("data.json")},
		"styles":              {"file": path("app.css")},
		"css":                 {"file": path("app.css")},
		"js":                  {"file": path("app.js")},
		"image":               {"src": path("photo.png")},
		"images":              {"directory": path("")},
		"esbuild":             {"entry": path("app.js"), "outfile": map[string]interface{}{"path": map[string]interface{}{"base": "static", "path": "app.js"}}},
		"plugin":              {"plugin": "Example"},
		"menu":                {"section": "main"},
		"markdown":            {"file": "document.md"},
	}
	for kind, values := range props {
		t.Run(kind, func(t *testing.T) {
			b, _ := json.Marshal(values)
			if _, err := prepareScaffold(scaffoldSpec{Module: root, Type: kind, Name: "example", File: "new.hyperbricks.yaml", Properties: string(b)}); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, s := range []scaffoldSpec{
		{Type: "template", Inline: "{{if}}"},
		{Type: "plugin", Properties: `{"plugin":""}`},
		{Type: "json_render", Properties: `{"file":{"path":{"base":"resources","path":"missing.json"}}}`},
		{Type: "image", Properties: `{"src":"/etc/hosts"}`},
		{Type: "images", Properties: `{"directory":{"path":{"base":"resources","path":"missing"}}}`},
	} {
		s.Module, s.Name, s.File = root, "invalid", "new.hyperbricks.yaml"
		if _, err := prepareScaffold(s); err == nil {
			t.Errorf("accepted invalid reference: %+v", s)
		}
	}
}

func TestScaffoldWizardFitsTerminal(t *testing.T) {
	root := scaffoldFixtureModule(t)
	initial := scaffoldSpec{Module: root}
	for _, size := range []tea.WindowSizeMsg{{Width: 80, Height: 24}, {Width: 50, Height: 20}} {
		m := newAuthoringWizard(func(v map[string]string) []wizardStep { return scaffoldWizardSteps(initial, v) }, func(map[string]string) (interface{}, string, error) { return nil, "", nil })
		m.values = map[string]string{"category": "composite", "type": "hypermedia", "file-choice": "app.hyperbricks.yaml", "name": "example"}
		m.Update(size)
		for i, s := range m.steps(m.values) {
			m.index = i
			m.enter()
			view := m.View()
			if lipgloss.Width(view) > size.Width || lipgloss.Height(view) > size.Height {
				t.Errorf("%s: view %dx%d exceeds %dx%d", s.key, lipgloss.Width(view), lipgloss.Height(view), size.Width, size.Height)
			}
		}
	}
}

func TestScaffoldPartialFailureReportsRetainedFiles(t *testing.T) {
	root := scaffoldFixtureModule(t)
	p, err := prepareScaffold(scaffoldSpec{Module: root, Type: "text", Name: "created", File: "new.hyperbricks.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	// A later write target becomes unavailable after preparation. Retain the
	// completed leaf, and never overwrite the conflicting file during recovery.
	blocked := filepath.Join(root, "source", "app.hyperbricks.yaml")
	p.Files = append(p.Files, scaffoldFile{Path: blocked, After: "unexpected"})
	err = p.apply()
	if err == nil || !strings.Contains(err.Error(), "completed files retained") || !strings.Contains(err.Error(), "new.hyperbricks.yaml") {
		t.Fatal(err)
	}
	if !strings.Contains(scaffoldRead(t, filepath.Join(root, "source", "new.hyperbricks.yaml")), "created:") || scaffoldRead(t, blocked) != scaffoldFixture {
		t.Fatal("partial recovery overwrote existing work")
	}
}

func TestScaffoldTemplateFileDefaultsAndHyphenatedChildren(t *testing.T) {
	root := scaffoldFixtureModule(t)
	p, err := prepareScaffold(scaffoldSpec{Module: root, Type: "api_render", Name: "remote", File: "remote.hyperbricks.yaml", Template: "remote.html", TemplateMode: "create", Properties: `{"endpoint":"https://example.com/api"}`})
	if err != nil || !strings.Contains(p.Files[0].After, "{{.Data}}") {
		t.Fatalf("API template lost its context: %v, %+v", err, p)
	}
	p, err = prepareScaffold(scaffoldSpec{Module: root, Type: "template", Name: "card", File: "card.hyperbricks.yaml", Children: `[{"body-text":[{"type":"markdown"},{"content":"Hello"}]}]`})
	if err != nil || !strings.Contains(p.Files[0].After, `index . "body-text"`) {
		t.Fatalf("invalid hyphenated child binding: %v, %+v", err, p)
	}
}

func TestScaffoldWizardCachesChoicesUntilAnswersChange(t *testing.T) {
	count := 0
	m := newAuthoringWizard(func(v map[string]string) []wizardStep {
		count++
		return []wizardStep{{key: "name", title: "Name", kind: "input"}}
	}, nil)
	for i := 0; i < 5; i++ {
		m.View()
		m.current()
	}
	if count != 1 {
		t.Fatalf("reloaded choices on navigation: %d", count)
	}
	m.values["name"] = "changed"
	m.current()
	if count != 2 {
		t.Fatal("changed answers did not refresh choices")
	}
}

func TestScaffoldWizardExistingFileSuggestsUnusedRoot(t *testing.T) {
	root := scaffoldFixtureModule(t)
	file := filepath.Join(root, "source", "components.hyperbricks.yaml")
	existing := "new_hypermedia:\n  - type: hypermedia\n  - title: Existing\n"
	scaffoldWrite(t, file, existing)
	scaffoldWrite(t, filepath.Join(root, "source", "other.hyperbricks.yaml"), "imports: [partials/other.hyperbricks.yaml]\n")
	scaffoldWrite(t, filepath.Join(root, "source", "partials", "other.hyperbricks.yaml"), "new_hypermedia_2:\n  - type: hypermedia\n")
	before := scaffoldState(t, root)
	initial := scaffoldSpec{Module: root}
	m := newAuthoringWizard(func(v map[string]string) []wizardStep { return scaffoldWizardSteps(initial, v) }, nil)
	m.values = map[string]string{"category": "composite", "type": "hypermedia"}
	m.index = 2
	m.enter()
	if m.current().key != "file-choice" {
		t.Fatal("expected configuration file step")
	}
	for i, option := range m.current().options {
		if option.value == "components.hyperbricks.yaml" {
			m.list.Select(i)
		}
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.current().key != "name" || m.input.Value() != "new_hypermedia_3" {
		t.Fatalf("existing file should advance to an unused root name, got %s: %s", m.current().key, m.input.Value())
	}
	m.input.SetValue("new_hypermedia")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.current().key != "name" || !strings.Contains(m.error, "already exists") {
		t.Fatal("duplicate name should be rejected at the name step")
	}
	m.input.SetValue("custom_page")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.current().key != "route" || m.values["name"] != "custom_page" {
		t.Fatal("valid custom name did not advance")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.input.Value() != "custom_page" {
		t.Fatal("back navigation replaced the user's name")
	}
	if !reflect.DeepEqual(before, scaffoldState(t, root)) {
		t.Fatal("wizard validation wrote files")
	}
	s := wizardScaffoldSpec(initial, m.values)
	p, err := prepareScaffoldLibrary(s)
	if err != nil {
		t.Fatal(err)
	}
	if err = p.apply(); err != nil {
		t.Fatal(err)
	}
	if after := scaffoldRead(t, file); !strings.HasPrefix(after, existing) || !strings.Contains(after, "custom_page:") {
		t.Fatal("new root did not preserve the existing definition")
	}
	if _, err := prepareScaffoldLibrary(s); err == nil {
		t.Fatal("explicit root names must still reject duplicates, never auto-rename")
	}
}

func TestScaffoldLibraryCoversFastTypesAndSubstitutesMetadata(t *testing.T) {
	types, err := scaffoldLibraryTypes("")
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(types, "hypermedia") || !containsString(types, "image") || !containsString(types, "json_render") || containsString(types, "plugin") {
		t.Fatalf("unexpected fast template catalog: %v", types)
	}
	node, err := scaffoldLibraryNode("hypermedia", "guides", "Guides")
	if err != nil {
		t.Fatal(err)
	}
	if route := yget(node, "route"); route == nil || route.Value != "guides" {
		t.Fatalf("route marker was not substituted: %#v", route)
	}
	if title := yget(node, "title"); title == nil || title.Value != "Guides" {
		t.Fatalf("title marker was not substituted: %#v", title)
	}
	doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{ymap()}}
	yput(doc.Content[0], "guides_page", node)
	raw, err := yamlBytes(doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := yamlparser.ParseBytes(raw); err != nil {
		t.Fatalf("library output is invalid: %v\n%s", err, raw)
	}
}

func TestScaffoldLibraryPlansReplaceableAssets(t *testing.T) {
	root := scaffoldFixtureModule(t)
	before := scaffoldState(t, root)
	for _, kind := range []string{"esbuild", "image", "images", "json_render", "styles"} {
		t.Run(kind, func(t *testing.T) {
			p, err := prepareScaffoldLibrary(scaffoldSpec{Module: root, Type: kind, Name: kind + "_starter", File: kind + ".hyperbricks.yaml"})
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Files) != 2 || !strings.Contains(p.Files[0].Path, "/resources/") || !strings.HasSuffix(p.Files[1].Path, "/source/"+kind+".hyperbricks.yaml") {
				t.Fatalf("unexpected %s starter plan: %+v", kind, p.Files)
			}
		})
	}
	if !reflect.DeepEqual(before, scaffoldState(t, root)) {
		t.Fatal("asset preview wrote files")
	}
	p, err := prepareScaffoldLibrary(scaffoldSpec{Module: root, Type: "image", Name: "hero_image", File: "image.hyperbricks.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.apply(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "resources", "images", "image_192a.png")); err != nil {
		t.Fatal(err)
	}
}

func TestScaffoldMarkdownFileStarterUsesContentResolver(t *testing.T) {
	node, err := scaffoldLibraryNode("markdown_file", "", "")
	if err != nil {
		t.Fatal(err)
	}
	content := yget(node, "content")
	file := yget(content, "file")
	if file == nil || yget(file, "base").Value != "resources" || yget(file, "path").Value != "content/markdown_3b7e.md" {
		t.Fatalf("unexpected Markdown content resolver: %#v", content)
	}
	if yget(node, "file") != nil {
		t.Fatal("Markdown file starter used the declared file field")
	}
}

func TestScaffoldWizardRootNameChecksUnloadedFileAndFreshConflicts(t *testing.T) {
	root := scaffoldFixtureModule(t)
	scaffoldWrite(t, filepath.Join(root, "source", "partials", "unused.hyperbricks.yaml"), "new_text:\n  - type: text\n  - value: Existing\n")
	v := map[string]string{"category": "component", "type": "text", "file-choice": "partials/unused.hyperbricks.yaml"}
	var nameStep wizardStep
	for _, step := range scaffoldWizardSteps(scaffoldSpec{Module: root}, v) {
		if step.key == "name" {
			nameStep = step
		}
	}
	if nameStep.initial != "new_text_2" {
		t.Fatalf("unimported destination was ignored: %s", nameStep.initial)
	}
	if err := nameStep.validate("new_text"); err == nil {
		t.Fatal("duplicate in unimported destination accepted")
	}
	if err := nameStep.validate(nameStep.initial); err != nil {
		t.Fatal(err)
	}
	scaffoldWrite(t, filepath.Join(root, "source", "appeared.hyperbricks.yaml"), "new_text_2:\n  - type: text\n  - value: Concurrent\n")
	before := scaffoldState(t, root)
	for _, name := range []string{"new_text_2", "imports", "vars"} {
		if err := nameStep.validate(name); err == nil {
			t.Errorf("accepted conflicting/reserved name %s", name)
		}
	}
	if !reflect.DeepEqual(before, scaffoldState(t, root)) {
		t.Fatal("validation changed source files")
	}
}
