package yamlparser

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	oldparser "github.com/hyperbricks/hyperbricks/pkg/parser"
)

func TestParseMaterializesOrderedChildren(t *testing.T) {
	doc, err := ParseBytes([]byte(`
page:
  - type: hypermedia
  - route: index
  - hero:
      - type: html
      - value: <h1>Hello</h1>
  - intro:
      - type: text
      - value: Welcome
`))
	if err != nil {
		t.Fatalf("ParseBytes() error = %v", err)
	}

	got, err := doc.Materialize()
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	page := got["page"].(map[string]interface{})

	if page["@type"] != "<HYPERMEDIA>" {
		t.Fatalf("page @type = %v", page["@type"])
	}
	if page["route"] != "index" {
		t.Fatalf("page route = %v", page["route"])
	}
	if order := page["@order"]; !reflect.DeepEqual(order, []string{"hero", "intro"}) {
		t.Fatalf("page @order = %#v", order)
	}
}

func TestParseKeepsDataSequencesAsArrays(t *testing.T) {
	doc, err := ParseBytes([]byte(`
page:
  - type: hypermedia
  - body:
      - type: template
      - values:
          cards:
            - title: Study
              body: Focus block
            - title: Workout
              body: Interval block
`))
	if err != nil {
		t.Fatalf("ParseBytes() error = %v", err)
	}

	got, err := doc.Materialize()
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	page := got["page"].(map[string]interface{})
	body := page["body"].(map[string]interface{})
	values := body["values"].(map[string]interface{})
	cards, ok := values["cards"].([]interface{})
	if !ok {
		t.Fatalf("cards type = %T, want []interface{}", values["cards"])
	}
	if len(cards) != 2 {
		t.Fatalf("cards len = %d, want 2", len(cards))
	}
	first := cards[0].(map[string]interface{})
	if first["title"] != "Study" {
		t.Fatalf("first title = %v", first["title"])
	}
}

func TestParseKeepsDirectScalarSequencesAsProperties(t *testing.T) {
	doc, err := ParseBytes([]byte(`
api:
  - type: api_render
  - querykeys:
      - id
      - slug
`))
	if err != nil {
		t.Fatalf("ParseBytes() error = %v", err)
	}

	got, err := doc.Materialize()
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	api := got["api"].(map[string]interface{})
	queryKeys, ok := api["querykeys"].([]interface{})
	if !ok {
		t.Fatalf("querykeys type = %T, want []interface{}", api["querykeys"])
	}
	if !reflect.DeepEqual(queryKeys, []interface{}{"id", "slug"}) {
		t.Fatalf("querykeys = %#v", queryKeys)
	}
}

func TestParseAllowsUntypedNestedExtensions(t *testing.T) {
	doc, err := ParseBytes([]byte(`
page:
  - type: hypermedia
  - head:
      - seo_meta:
          - values:
              title: Home
`))
	if err != nil {
		t.Fatalf("ParseBytes() error = %v", err)
	}

	got, err := doc.Materialize()
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	page := got["page"].(map[string]interface{})
	head := page["head"].(map[string]interface{})
	seoMeta := head["seo_meta"].(map[string]interface{})
	values := seoMeta["values"].(map[string]interface{})
	if values["title"] != "Home" {
		t.Fatalf("title = %v", values["title"])
	}
	if order := head["@order"]; !reflect.DeepEqual(order, []string{"seo_meta"}) {
		t.Fatalf("head @order = %#v", order)
	}
}

func TestParseMaterializesNestedComponentProps(t *testing.T) {
	doc, err := ParseBytes([]byte(`
page:
  - type: hypermedia
  - values:
      content:
        - type: tree
        - hero:
            - type: html
            - value: <h1>Hello</h1>
`))
	if err != nil {
		t.Fatalf("ParseBytes() error = %v", err)
	}

	got, err := doc.Materialize()
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	page := got["page"].(map[string]interface{})
	values := page["values"].(map[string]interface{})
	content := values["content"].(map[string]interface{})
	if content["@type"] != "<TREE>" {
		t.Fatalf("content @type = %v", content["@type"])
	}
	if order := content["@order"]; !reflect.DeepEqual(order, []string{"hero"}) {
		t.Fatalf("content @order = %#v", order)
	}
}

func TestMaterializeResolvesInheritanceWithDeepMerge(t *testing.T) {
	doc, err := ParseBytes([]byte(`
my_component:
  - type: template
  - template: video.html
  - values:
      width: 300
      height: 400
      src: https://example.com/original

page:
  - type: hypermedia
  - video:
      - inherit: my_component
      - values:
          src: https://example.com/override
`))
	if err != nil {
		t.Fatalf("ParseBytes() error = %v", err)
	}

	got, err := doc.Materialize()
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	page := got["page"].(map[string]interface{})
	video := page["video"].(map[string]interface{})
	values := video["values"].(map[string]interface{})

	if video["@type"] != "<TEMPLATE>" {
		t.Fatalf("video @type = %v", video["@type"])
	}
	if values["width"] != "300" {
		t.Fatalf("width = %#v", values["width"])
	}
	if values["src"] != "https://example.com/override" {
		t.Fatalf("src = %#v", values["src"])
	}
}

func TestMaterializeInheritedValueNodeOverridesInheritedBaseValueNode(t *testing.T) {
	doc, err := ParseBytes([]byte(`
base_panel:
  - type: template
  - template: base.html

replacement_panel:
  - type: template
  - template: replacement.html

shell:
  - type: hypermedia
  - template_10:
      - type: template
      - values:
          content:
            - inherit: base_panel

page:
  - inherit: shell
  - template_10:
      - values:
          content:
            - inherit: replacement_panel
`))
	if err != nil {
		t.Fatalf("ParseBytes() error = %v", err)
	}

	got, err := doc.Materialize()
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	page := got["page"].(map[string]interface{})
	body := page["template_10"].(map[string]interface{})
	values := body["values"].(map[string]interface{})
	content := values["content"].(map[string]interface{})

	if content["@type"] != "<TEMPLATE>" {
		t.Fatalf("content @type = %#v", content["@type"])
	}
	if content["template"] != "replacement.html" {
		t.Fatalf("content template = %#v, want replacement.html", content["template"])
	}
}

func TestProcessBytesValueResolversMaterializeRuntimeShape(t *testing.T) {
	assetsDir := t.TempDir()
	heroPath := filepath.Join(assetsDir, "hero.html")
	if err := os.WriteFile(heroPath, []byte("<section>From file</section>\n<p>Second line</p>\n"), 0o644); err != nil {
		t.Fatalf("write file resolver asset: %v", err)
	}

	result, err := ProcessBytes([]byte(`
vars:
  route: pipeline
  cta:
    text: Start now
  asset:
    name: hero.png

page:
  - type: hypermedia
  - route:
      var: route
  - title:
      format: "%s %s"
      args:
        - config: site.title
        - env:
            name: HB_PAGE_SUFFIX
            default: Page
  - hero:
      - type: html
      - value:
          file:
            base: resources
            path: hero.html
  - cta:
      - type: text
      - value:
          var: cta.text
  - details:
      - type: template
      - values:
          width: 800
          enabled: true
          empty:
          resource:
            path:
              base: resources
              parts:
                - var: asset.name
`), Options{
		Config: map[string]interface{}{
			"site": map[string]interface{}{
				"title": "Pipeline Page",
			},
		},
		Paths: PathMarkers{
			Resources: assetsDir,
		},
	})
	if err != nil {
		t.Fatalf("ProcessBytes() error = %v", err)
	}
	if !strings.Contains(result.Preprocessed, "var: route") {
		t.Fatalf("resolver source should not be string-preprocessed:\n%s", result.Preprocessed)
	}

	page := result.Materialized["page"].(map[string]interface{})
	if page["route"] != "pipeline" || page["title"] != "Pipeline Page Page" {
		t.Fatalf("page fields = %#v", page)
	}
	hero := page["hero"].(map[string]interface{})
	if !strings.Contains(hero["value"].(string), "<section>From file</section>") {
		t.Fatalf("file resolver content = %#v", hero["value"])
	}
	details := page["details"].(map[string]interface{})
	values := details["values"].(map[string]interface{})
	if values["width"] != "800" {
		t.Fatalf("width = %#v, want string", values["width"])
	}
	if values["enabled"] != "true" {
		t.Fatalf("enabled = %#v, want string", values["enabled"])
	}
	if values["empty"] != "" {
		t.Fatalf("empty = %#v, want empty string", values["empty"])
	}
	if values["resource"] != filepath.Join(assetsDir, "hero.png") {
		t.Fatalf("resource = %#v", values["resource"])
	}
}

func TestProcessBytesValueResolverDiagnosticsAreRecoverable(t *testing.T) {
	result, err := ProcessBytes([]byte(`
page:
  - type: hypermedia
  - route:
      var: missing.route
  - title:
      env:
        name: HB_MISSING_TITLE
        required: true
`), Options{})
	if err != nil {
		t.Fatalf("ProcessBytes() error = %v", err)
	}
	page := result.Materialized["page"].(map[string]interface{})
	if page["route"] != "" || page["title"] != "" {
		t.Fatalf("page recovered values = %#v", page)
	}
	if len(result.Diagnostics) != 2 {
		t.Fatalf("diagnostics len = %d, want 2: %#v", len(result.Diagnostics), result.Diagnostics)
	}
	diagnosticsByCode := map[string]Diagnostic{}
	for _, diagnostic := range result.Diagnostics {
		diagnosticsByCode[diagnostic.Code] = diagnostic
	}
	if diagnostic := diagnosticsByCode["var_missing"]; diagnostic.Path != "page.route" {
		t.Fatalf("var diagnostic = %#v", diagnostic)
	}
	if diagnostic := diagnosticsByCode["env_missing"]; diagnostic.Level != "error" || diagnostic.Path != "page.title" {
		t.Fatalf("env diagnostic = %#v", diagnostic)
	}
}

func TestParseYAMLCommentsAndQuotedHashValues(t *testing.T) {
	doc, err := ParseBytes([]byte(`
# source comment is ignored by YAML
fragment:
  - type: fragment # inline comment is ignored by YAML
  - route: comments
  - response:
      hx_trigger: fixture-updated
      hx_target: "#status"
      hx_reswap: outerHTML
  - body:
      - type: html
      - value: |
          #status stays literal inside the block scalar
          <div id="status">OK</div>
`))
	if err != nil {
		t.Fatalf("ParseBytes() error = %v", err)
	}
	materialized, err := doc.Materialize()
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	fragment := materialized["fragment"].(map[string]interface{})
	response := fragment["response"].(map[string]interface{})
	if response["hx_target"] != "#status" {
		t.Fatalf("hx_target = %#v", response["hx_target"])
	}
	if response["hx_reswap"] != "outerHTML" {
		t.Fatalf("hx_reswap = %#v", response["hx_reswap"])
	}
	body := fragment["body"].(map[string]interface{})
	if !strings.Contains(body["value"].(string), "#status stays literal") {
		t.Fatalf("block scalar value = %#v", body["value"])
	}
}

func TestProcessBytesTemplateFileResolverStoresTemplateContent(t *testing.T) {
	oldparser.ClearTemplateStore()
	t.Cleanup(oldparser.ClearTemplateStore)

	templateDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(templateDir, "cards"), 0o755); err != nil {
		t.Fatalf("mkdir template dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templateDir, "cards", "card.html"), []byte("<article>{{.title}}</article>"), 0o644); err != nil {
		t.Fatalf("write template file: %v", err)
	}

	result, err := ProcessBytes([]byte(`
card:
  - type: template
  - template:
      file: cards/card.html
  - values:
      title: Stored template
`), Options{TemplateDir: templateDir})
	if err != nil {
		t.Fatalf("ProcessBytes() error = %v", err)
	}
	card := result.Materialized["card"].(map[string]interface{})
	if card["template"] != "cards/card.html" {
		t.Fatalf("template field = %#v", card["template"])
	}
	if content, found := oldparser.GetTemplate("cards/card.html"); !found || content != "<article>{{.title}}</article>" {
		t.Fatalf("stored template = %q, found=%v", content, found)
	}
}

func TestLoadFileResolvesImportsInDocumentOrder(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "partials"), 0o755); err != nil {
		t.Fatalf("mkdir partials: %v", err)
	}
	writeYAMLTestFile(t, filepath.Join(root, "partials", "site.hyperbricks.yaml"), `
site_card:
  - type: template
  - values:
      title: Base title
      body: Base body
`)
	writeYAMLTestFile(t, filepath.Join(root, "partials", "plan.hyperbricks.yaml"), `
plan_card:
  - inherit: site_card
  - values:
      title: Plan title
`)
	mainPath := filepath.Join(root, "page.hyperbricks.yaml")
	writeYAMLTestFile(t, mainPath, `
imports:
  - partials/site.hyperbricks.yaml
  - partials/plan.hyperbricks.yaml

page:
  - type: hypermedia
  - route: index
  - hero:
      - inherit: plan_card
`)

	doc, err := LoadFile(mainPath, Options{})
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	rootNames := make([]string, 0, len(doc.Roots))
	for _, root := range doc.Roots {
		rootNames = append(rootNames, root.Name)
	}
	if !reflect.DeepEqual(rootNames, []string{"site_card", "plan_card", "page"}) {
		t.Fatalf("root order = %#v", rootNames)
	}
	materialized, err := doc.Materialize()
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	page := materialized["page"].(map[string]interface{})
	hero := page["hero"].(map[string]interface{})
	values := hero["values"].(map[string]interface{})
	if values["title"] != "Plan title" || values["body"] != "Base body" {
		t.Fatalf("inherited hero values = %#v", values)
	}
}

func TestLoadFileRejectsDuplicateImportedRoots(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "partials"), 0o755); err != nil {
		t.Fatalf("mkdir partials: %v", err)
	}
	writeYAMLTestFile(t, filepath.Join(root, "partials", "a.hyperbricks.yaml"), `
shared:
  - type: html
  - value: A
`)
	writeYAMLTestFile(t, filepath.Join(root, "partials", "b.hyperbricks.yaml"), `
shared:
  - type: html
  - value: B
`)
	mainPath := filepath.Join(root, "page.hyperbricks.yaml")
	writeYAMLTestFile(t, mainPath, `
imports:
  - partials/a.hyperbricks.yaml
  - partials/b.hyperbricks.yaml
`)

	_, err := LoadFile(mainPath, Options{})
	if err == nil {
		t.Fatal("LoadFile() error = nil, want duplicate root error")
	}
	if !strings.Contains(err.Error(), `duplicate top-level object "shared"`) {
		t.Fatalf("LoadFile() error = %v", err)
	}
}

func TestLoadFileRejectsImportCycles(t *testing.T) {
	root := t.TempDir()
	aPath := filepath.Join(root, "a.hyperbricks.yaml")
	bPath := filepath.Join(root, "b.hyperbricks.yaml")
	writeYAMLTestFile(t, aPath, `
imports:
  - b.hyperbricks.yaml
a:
  - type: html
`)
	writeYAMLTestFile(t, bPath, `
imports:
  - a.hyperbricks.yaml
b:
  - type: html
`)

	_, err := LoadFile(aPath, Options{})
	if err == nil {
		t.Fatal("LoadFile() error = nil, want import cycle error")
	}
	if !strings.Contains(err.Error(), "import cycle detected") {
		t.Fatalf("LoadFile() error = %v", err)
	}
}

func TestPreprocessBytesRejectsUnsupportedMacroSyntax(t *testing.T) {
	_, err := PreprocessBytes([]byte(`
@macro "button"
`), Options{})
	if err == nil {
		t.Fatal("PreprocessBytes() error = nil, want macro syntax error")
	}
	if !strings.Contains(err.Error(), "@macro syntax is not supported") {
		t.Fatalf("PreprocessBytes() error = %v", err)
	}
}

func TestProcessBytesDoesNotResolveInterpolationMarkers(t *testing.T) {
	for _, test := range []struct {
		name string
		key  string
		raw  string
	}{
		{
			name: "var",
			key:  "route",
			raw:  interpolationMarker("VAR", "route"),
		},
		{
			name: "env",
			key:  "title",
			raw:  interpolationMarker("ENV", "TITLE"),
		},
		{
			name: "config",
			key:  "title",
			raw:  interpolationMarker("CONF", "site.title"),
		},
		{
			name: "file",
			key:  "value",
			raw:  interpolationMarker("FILE", "/tmp/example.html"),
		},
		{
			name: "template",
			key:  "template",
			raw:  interpolationMarker("TEMPLATE", "cards/card.html"),
		},
		{
			name: "path",
			key:  "value",
			raw:  pathInterpolationMarker("RESOURCES") + "/hero.html",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := `
page:
  - type: hypermedia
  - ` + test.key + `: "` + test.raw + `"
`
			result, err := ProcessBytes([]byte(source), Options{
				Variables: map[string]string{"route": "resolved-route"},
				Env:       map[string]string{"TITLE": "Resolved title"},
				Config: map[string]interface{}{
					"site": map[string]interface{}{"title": "Resolved config title"},
				},
				TemplateDir: t.TempDir(),
				Paths:       PathMarkers{Resources: "resolved-resources"},
			})
			if err == nil {
				page := result.Materialized["page"].(map[string]interface{})
				if page[test.key] != test.raw {
					t.Fatalf("%s = %#v, want literal %#v", test.key, page[test.key], test.raw)
				}
				return
			}
			t.Fatalf("ProcessBytes() error = %v", err)
		})
	}
}

func interpolationMarker(name string, value string) string {
	return "{{" + name + ":" + value + "}}"
}

func pathInterpolationMarker(name string) string {
	return "{{" + name + "}}"
}

func TestParseImports(t *testing.T) {
	doc, err := ParseBytes([]byte(`
imports:
  - partials/site.hyperbricks.yaml
  - partials/plan.hyperbricks.yaml
`))
	if err != nil {
		t.Fatalf("ParseBytes() error = %v", err)
	}
	want := []string{"partials/site.hyperbricks.yaml", "partials/plan.hyperbricks.yaml"}
	if !reflect.DeepEqual(doc.Imports, want) {
		t.Fatalf("imports = %#v, want %#v", doc.Imports, want)
	}
}

func TestParseRejectsUnknownType(t *testing.T) {
	_, err := ParseBytes([]byte(`
page:
  - type: made_up
`))
	if err == nil {
		t.Fatal("ParseBytes() error = nil, want unknown type error")
	}
	if !strings.Contains(err.Error(), `unknown component type "made_up"`) {
		t.Fatalf("ParseBytes() error = %v", err)
	}
}

func TestParseRejectsDuplicateChildren(t *testing.T) {
	_, err := ParseBytes([]byte(`
page:
  - type: hypermedia
  - hero:
      - type: html
  - hero:
      - type: text
`))
	if err == nil {
		t.Fatal("ParseBytes() error = nil, want duplicate child error")
	}
	if !strings.Contains(err.Error(), `duplicate child "hero"`) {
		t.Fatalf("ParseBytes() error = %v", err)
	}
}

func TestParseRecoverDuplicateChildrenRenamesAndDiagnoses(t *testing.T) {
	doc, err := ParseBytesWithOptions([]byte(`
page:
  - type: hypermedia
  - hero:
      - type: html
      - value: first
  - hero:
      - type: text
      - value: second
  - hero_2:
      - type: html
      - value: explicit
`), ParseOptions{RecoverDuplicateChildren: true})
	if err != nil {
		t.Fatalf("ParseBytesWithOptions() error = %v", err)
	}
	if len(doc.Diagnostics) != 1 {
		t.Fatalf("diagnostics len = %d, want 1: %#v", len(doc.Diagnostics), doc.Diagnostics)
	}
	diagnostic := doc.Diagnostics[0]
	if diagnostic.Code != "duplicate_child_name" || diagnostic.Path != "page" || diagnostic.OriginalName != "hero" || diagnostic.MaterializedName != "hero_3" {
		t.Fatalf("diagnostic = %#v", diagnostic)
	}

	got, err := doc.Materialize()
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	page := got["page"].(map[string]interface{})
	if order := page["@order"]; !reflect.DeepEqual(order, []string{"hero", "hero_3", "hero_2"}) {
		t.Fatalf("page @order = %#v", order)
	}
	if _, ok := page["hero_3"].(map[string]interface{}); !ok {
		t.Fatalf("expected recovered child hero_3 in materialized page: %#v", page)
	}
}

func TestProcessBytesRecoverDuplicateChildrenInDeepNestedCompositeMaps(t *testing.T) {
	result, err := ProcessBytes([]byte(`
page:
  - type: hypermedia
  - route: deep-duplicates
  - main:
      - type: tree
      - article:
          - type: tree
          - body:
              - type: tree
              - copy:
                  - type: text
                  - value: first
              - copy:
                  - type: text
                  - value: second
`), Options{RecoverDuplicateChildren: true})
	if err != nil {
		t.Fatalf("ProcessBytes() error = %v", err)
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("diagnostics len = %d, want 1: %#v", len(result.Diagnostics), result.Diagnostics)
	}
	if result.Diagnostics[0].Path != "page.main.article.body" || result.Diagnostics[0].MaterializedName != "copy_2" {
		t.Fatalf("diagnostic = %#v", result.Diagnostics[0])
	}

	page := result.Materialized["page"].(map[string]interface{})
	main := page["main"].(map[string]interface{})
	article := main["article"].(map[string]interface{})
	body := article["body"].(map[string]interface{})
	if order := body["@order"]; !reflect.DeepEqual(order, []string{"copy", "copy_2"}) {
		t.Fatalf("body @order = %#v", order)
	}
	copy2 := body["copy_2"].(map[string]interface{})
	if copy2["value"] != "second" {
		t.Fatalf("copy_2 value = %#v", copy2["value"])
	}
}

func TestParseRejectsPropertyChildCollision(t *testing.T) {
	_, err := ParseBytes([]byte(`
page:
  - type: hypermedia
  - hero: plain value
  - hero:
      - type: html
`))
	if err == nil {
		t.Fatal("ParseBytes() error = nil, want property/child collision error")
	}
	if !strings.Contains(err.Error(), `collides with a property`) {
		t.Fatalf("ParseBytes() error = %v", err)
	}
}

func TestParseRejectsReservedRuntimeChildName(t *testing.T) {
	_, err := ParseBytes([]byte(`
head:
  - type: head
  - css:
      - type: css
      - inline: |
          body { color: green; }
`))
	if err == nil {
		t.Fatal("ParseBytes() error = nil, want reserved child name error")
	}
	if !strings.Contains(err.Error(), `collides with a reserved <HEAD> field`) {
		t.Fatalf("ParseBytes() error = %v", err)
	}
}

func TestParseAllowsReservedRuntimeFieldAsProperty(t *testing.T) {
	doc, err := ParseBytes([]byte(`
page:
  - type: hypermedia
  - head:
      - type: head
      - css:
          - /assets/app.css
      - inline_styles:
          - type: css
          - inline: |
              body { color: green; }
`))
	if err != nil {
		t.Fatalf("ParseBytes() error = %v", err)
	}

	got, err := doc.Materialize()
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	page := got["page"].(map[string]interface{})
	head := page["head"].(map[string]interface{})
	css, ok := head["css"].([]interface{})
	if !ok {
		t.Fatalf("head.css type = %T, want []interface{}", head["css"])
	}
	if !reflect.DeepEqual(css, []interface{}{"/assets/app.css"}) {
		t.Fatalf("head.css = %#v", css)
	}
	if order := head["@order"]; !reflect.DeepEqual(order, []string{"inline_styles"}) {
		t.Fatalf("head @order = %#v", order)
	}
}

func TestParseCanonicalizesJSONAlias(t *testing.T) {
	doc, err := ParseBytes([]byte(`
data:
  - type: json
  - file: data.json
`))
	if err != nil {
		t.Fatalf("ParseBytes() error = %v", err)
	}
	got, err := doc.Materialize()
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	data := got["data"].(map[string]interface{})
	if data["@type"] != "<JSON_RENDER>" {
		t.Fatalf("data @type = %v", data["@type"])
	}
}

func writeYAMLTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.TrimLeft(content, "\n")), 0o644); err != nil {
		t.Fatalf("write YAML test file %s: %v", path, err)
	}
}
