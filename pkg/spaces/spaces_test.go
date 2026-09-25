package spaces

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

const fixtureSource = `# Source comment stays
portfolio_page:
  - type: hypermedia
  - route: index
  - title: Portfolio
  - head:
      - type: head
      - meta:
          author: Alex
          description: Original description
          og:title: Portfolio
          og:type: website
          og:url: https://example.com/
  - content:
      - type: template
      - inline: '<main>{{.body}}</main>'
      - values:
          body:
            - type: template
            - inline: '<h1>{{.name}}</h1><p>{{.welcome}}</p><img src="{{.hero_image}}"><a href="{{.article_markdown}}">Document</a>'
            - values:
                name: Alex
                welcome: Welcome
                email: hello@example.com
                hero_image: ''
                article_markdown: ''
                private_value: Not editable
            - editable:
                name: {type: text, label: Name, max: 10, required: true}
                welcome: {type: textarea, rows: 3}
                email: email
                hero_image:
                  type: asset
                  upload:
                    accept: [.png, .jpg, .webp]
                    max_bytes: 10000
                    directory: {base: static, path: uploads/images}
                article_markdown:
                  type: asset
                  upload:
                    accept: [.md, .markdown]
                    max_bytes: 10000
                    directory: {base: resources, path: uploads/documents}
`

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}
func testService(t *testing.T) *service {
	t.Helper()
	module := t.TempDir()
	module, _ = filepath.EvalSymlinks(module)
	for _, dir := range []string{"hyperbricks", "templates", "resources", "static"} {
		if err := os.Mkdir(filepath.Join(module, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, filepath.Join(module, "hyperbricks", "portfolio.hyperbricks.yaml"), fixtureSource)
	s, err := newService(module, "/__hyperbricks/spaces", nil, editorOptions{Write: true, PublicOrigin: "https://example.com", SharingImage: &UploadPolicy{Accept: []string{".png"}, MaxBytes: 10000, Directory: Directory{Base: "static", Path: "uploads/sharing"}}}, true)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func catalogTest(t *testing.T, s *service) *catalog {
	t.Helper()
	c, err := s.catalog()
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func createTest(t *testing.T, s *service, name string) {
	t.Helper()
	c := catalogTest(t, s)
	if err := s.mutate(c, Mutation{Action: "create", Revision: c.revision, Source: "portfolio_page", Name: name, Title: "Dutch Portfolio", Route: "portfolio/" + name}, nil); err != nil {
		t.Fatal(err)
	}
}
func currentSpace(t *testing.T, s *service, name string) (*catalog, Space) {
	t.Helper()
	c := catalogTest(t, s)
	e, err := c.findEntry(name)
	if err != nil {
		t.Fatal(err)
	}
	space, err := s.space(c, e)
	if err != nil {
		t.Fatal(err)
	}
	return c, space
}
func saveMutation(c *catalog, sp Space) Mutation {
	return Mutation{Action: "save", Revision: c.revision, Name: sp.Name, Title: sp.Title, Route: sp.Route, Values: map[string]string{}}
}
func fieldID(t *testing.T, sp Space, key string) string {
	t.Helper()
	for _, f := range sp.Fields {
		if f.Key == key {
			return f.ID
		}
	}
	t.Fatalf("missing %s", key)
	return ""
}
func fileText(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCRUDSourceInheritanceTrashRestart(t *testing.T) {
	s := testService(t)
	createTest(t, s, "dutch")
	createTest(t, s, "english")
	c, sp := currentSpace(t, s, "dutch")
	if len(sp.Fields) != 5 || sp.Fields[0].Type != "asset" {
		t.Fatalf("fields: %+v", sp.Fields)
	}
	if c.defs["dutch"].effective["content"].(map[string]interface{})["inline"] != "<main>{{.body}}</main>" {
		t.Fatal("lost inherited outer template")
	}
	if got := getMap(c.defs["dutch"].effective, []string{"content", "values", "body", "values", "private_value"}); got != "Not editable" {
		t.Fatalf("lost nested value: %#v", got)
	}
	m := saveMutation(c, sp)
	m.Values[fieldID(t, sp, "name")] = "Nederlands"
	m.Meta = map[string]*string{"author": nil}
	m.Title = "Nederlandse versie"
	if err := s.mutate(c, m, nil); err != nil {
		t.Fatal(err)
	}
	c, sp = currentSpace(t, s, "dutch")
	if sp.Meta["author"] != nil || sp.Title != "Nederlandse versie" {
		t.Fatalf("save: %+v", sp)
	}
	_, sibling := currentSpace(t, s, "english")
	if sibling.Meta["author"] != "Alex" {
		t.Fatal("source/sibling metadata changed")
	}
	if err := s.mutate(c, Mutation{Action: "trash", Name: "dutch", Revision: c.revision}, nil); err != nil {
		t.Fatal(err)
	}
	c, sp = currentSpace(t, s, "dutch")
	if !sp.Trashed || c.defs["dutch"] != nil {
		t.Fatal("trash is still loaded")
	}
	index := filepath.Join(s.dirs["hyperbricks"], "spaces", "portfolio_page", "index.hyperbricks.yaml")
	if !strings.Contains(fileText(t, index), `# - "dutch.hyperbricks.yaml"`) {
		t.Fatal("trash is not a commented import")
	}
	if err := s.mutate(c, Mutation{Action: "trash", Name: "english", Revision: c.revision}, nil); err != nil {
		t.Fatal(err)
	}
	c = catalogTest(t, s)
	if len(c.entries) != 2 || c.defs["dutch"] != nil || c.defs["english"] != nil {
		t.Fatal("all-commented index was not retained")
	}
	if _, err := yamlparser.ProcessFile(filepath.Join(s.dirs["hyperbricks"], "portfolio.hyperbricks.yaml"), s.parserOptions()); err != nil {
		t.Fatal(err)
	}
	// New service has no in-memory lifecycle registry.
	restarted, err := newService(s.module, s.route, nil, editorOptions{Write: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	c = catalogTest(t, restarted)
	if err := restarted.mutate(c, Mutation{Action: "restore", Name: "dutch", Revision: c.revision}, nil); err != nil {
		t.Fatal(err)
	}
	_, sp = currentSpace(t, restarted, "dutch")
	if sp.Trashed || sp.Meta["author"] != nil || sp.Title != "Nederlandse versie" {
		t.Fatalf("restore lost content: %+v", sp)
	}
	if !strings.Contains(fileText(t, filepath.Join(s.dirs["hyperbricks"], "portfolio.hyperbricks.yaml")), "# Source comment stays") {
		t.Fatal("lost source comment")
	}
}

func TestMutationSafety(t *testing.T) {
	s := testService(t)
	createTest(t, s, "dutch")
	c, sp := currentSpace(t, s, "dutch")
	before := fileText(t, filepath.Join(s.dirs["hyperbricks"], sp.File))
	for _, tc := range []struct {
		name   string
		change func(*Mutation)
	}{
		{"schema-bypass", func(m *Mutation) { m.Values["/content/values/body/values/private_value"] = "bad" }},
		{"maxlength", func(m *Mutation) { m.Values[fieldID(t, sp, "name")] = "elevenchars" }},
		{"email", func(m *Mutation) { m.Values[fieldID(t, sp, "email")] = "bad" }},
		{"required", func(m *Mutation) { m.Values[fieldID(t, sp, "name")] = "" }},
		{"route-collision", func(m *Mutation) { m.Route = "index" }},
		{"reserved-route", func(m *Mutation) { m.Route = "__hyperbricks/spaces" }},
		{"route-traversal", func(m *Mutation) { m.Route = "foo/../bar" }},
		{"revision", func(m *Mutation) { m.Revision = "old" }},
		{"unsafe-meta-key", func(m *Mutation) { v := "x"; m.Meta = map[string]*string{`a" onclick`: &v} }},
		{"relative-og-url", func(m *Mutation) { v := "/static/image.png"; m.Meta = map[string]*string{"og:image": &v} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := saveMutation(c, sp)
			tc.change(&m)
			if err := s.mutate(c, m, nil); err == nil {
				t.Fatal("accepted unsafe mutation")
			}
			if got := fileText(t, filepath.Join(s.dirs["hyperbricks"], sp.File)); got != before {
				t.Fatal("rejected save modified YAML")
			}
		})
	}
	m := saveMutation(c, sp)
	m.Values[fieldID(t, sp, "name")] = "éééééééééé"
	if err := s.mutate(c, m, nil); err != nil {
		t.Fatal("Unicode count:", err)
	}
	c, sp = currentSpace(t, s, "dutch")
	s.write = false
	if err := s.mutate(c, saveMutation(c, sp), nil); err == nil {
		t.Fatal("read-only write")
	}
	s.write = true
	writeTestFile(t, filepath.Join(s.dirs["hyperbricks"], "portfolio.hyperbricks.yaml"), fixtureSource+"\n# concurrent source edit\n")
	if err := s.mutate(c, saveMutation(c, sp), nil); err == nil {
		t.Fatal("did not detect concurrent source edit")
	}
}

func TestInstanceCannotBroadenSchemaAndMissingOverrides(t *testing.T) {
	s := testService(t)
	createTest(t, s, "dutch")
	c, sp := currentSpace(t, s, "dutch")
	p := filepath.Join(s.dirs["hyperbricks"], sp.File)
	writeTestFile(t, p, `# Keep me
dutch:
  - inherit: portfolio_page
  - title: Dutch
  - route: dutch
  - content:
      - values:
          body:
            - type: template
            - editable: [private_value]
`)
	c, sp = currentSpace(t, s, "dutch")
	if len(sp.Fields) != 5 {
		t.Fatalf("instance broadened schema: %+v", sp.Fields)
	}
	m := saveMutation(c, sp)
	m.Values[fieldID(t, sp, "welcome")] = "Hallo\nwereld"
	if err := s.mutate(c, m, nil); err != nil {
		t.Fatal(err)
	}
	c, sp = currentSpace(t, s, "dutch")
	if getMap(c.defs["dutch"].effective, []string{"content", "values", "body", "values", "welcome"}) != "Hallo\nwereld" {
		t.Fatal("inherited missing override was not saved")
	}
	if !strings.Contains(fileText(t, p), "# Keep me") {
		t.Fatal("comment lost")
	}
}

func TestTrashDependenciesAndRestoreConflicts(t *testing.T) {
	s := testService(t)
	createTest(t, s, "dutch")
	c, sp := currentSpace(t, s, "dutch")
	if err := s.mutate(c, Mutation{Action: "create", Revision: c.revision, Name: "dependent", Source: "dutch", Title: "Dependent", Route: "dependent"}, nil); err != nil {
		t.Fatal(err)
	}
	c, sp = currentSpace(t, s, "dutch")
	if sp.TrashBlocked == "" {
		t.Fatal("dependent Space did not block trash")
	}
	if err := s.mutate(c, Mutation{Action: "trash", Revision: c.revision, Name: "dutch"}, nil); err == nil {
		t.Fatal("trashed dependency")
	}
	s2 := testService(t)
	createTest(t, s2, "dutch")
	c, sp = currentSpace(t, s2, "dutch")
	if err := s2.mutate(c, Mutation{Action: "trash", Revision: c.revision, Name: "dutch"}, nil); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(s2.dirs["hyperbricks"], "conflict.hyperbricks.yaml"), "collision:\n  - type: hypermedia\n  - route: "+sp.Route+"\n")
	c = catalogTest(t, s2)
	if err := s2.mutate(c, Mutation{Action: "restore", Revision: c.revision, Name: "dutch"}, nil); err == nil {
		t.Fatal("restored colliding route")
	}
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 200, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func TestAssetsUploadAndSelection(t *testing.T) {
	s := testService(t)
	createTest(t, s, "dutch")
	c, sp := currentSpace(t, s, "dutch")
	id := fieldID(t, sp, "hero_image")
	u, err := s.prepareUpload(c, "dutch", id, "../../photo.png", pngBytes(t))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(u.reference, "/static/uploads/images/") || strings.Contains(u.path, "photo") {
		t.Fatal("client filename controlled path")
	}
	if err := s.mutate(c, saveMutation(c, sp), u); err != nil {
		t.Fatal(err)
	}
	c, sp = currentSpace(t, s, "dutch")
	assets, err := s.assetList(c, "dutch", id)
	if err != nil || len(assets) != 1 {
		t.Fatalf("assets %v %v", assets, err)
	}
	for _, tc := range []struct {
		name string
		data []byte
	}{{"bad.png", []byte("not png")}, {"fake.jpg", pngBytes(t)}, {"x.svg", []byte("<svg/>")}, {"huge.png", make([]byte, 10001)}} {
		if _, err := s.prepareUpload(c, "dutch", id, tc.name, tc.data); err == nil {
			t.Fatalf("accepted %s", tc.name)
		}
	}
	mdID := fieldID(t, sp, "article_markdown")
	md := []byte("# Hello\n\nMarkdown document.\n")
	upload, err := s.prepareUpload(c, "dutch", mdID, "article.md", md)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.mutate(c, saveMutation(c, sp), upload); err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(upload.reference, "/") || !strings.HasPrefix(upload.reference, "uploads/documents/") {
		t.Fatal("Markdown reference not portable")
	}
	if _, err := s.prepareUpload(c, "dutch", mdID, "bad.md", []byte{0xff, 0, 1}); err == nil {
		t.Fatal("binary Markdown accepted")
	}
	c, sp = currentSpace(t, s, "dutch")
	m := saveMutation(c, sp)
	m.Values[id] = ""
	if err := s.mutate(c, m, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(u.path); err != nil {
		t.Fatal("clearing asset deleted shared file")
	}
	c, sp = currentSpace(t, s, "dutch")
	sharing, err := s.prepareUpload(c, "dutch", "@meta.og:image", "hero.png", pngBytes(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.mutate(c, saveMutation(c, sp), sharing); err != nil {
		t.Fatal(err)
	}
	_, sp = currentSpace(t, s, "dutch")
	if !strings.HasPrefix(str(sp.Meta["og:image"]), "https://example.com/static/") {
		t.Fatal("sharing URL guessed host")
	}
}

func TestSymlinkTraversalAndUnimportedSource(t *testing.T) {
	s := testService(t)
	outside := t.TempDir()
	writeTestFile(t, filepath.Join(outside, "image.png"), string(pngBytes(t)))
	if err := os.Symlink(outside, filepath.Join(s.dirs["static"], "escape")); err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"/static/../resources/x.md", "/static/escape/image.png", "https://evil.example/x.png"} {
		if _, err := s.assetPath(Field{Type: "asset"}, ref); err == nil {
			t.Fatalf("accepted %s", ref)
		}
	}
	writeTestFile(t, filepath.Join(s.dirs["hyperbricks"], "unused", "page.hyperbricks.yaml"), "unused:\n  - type: hypermedia\n")
	c := catalogTest(t, s)
	if c.defs["unused"] != nil {
		t.Fatal("unimported source included")
	}
	if err := os.Symlink(outside, filepath.Join(s.dirs["hyperbricks"], "spaces")); err != nil {
		t.Fatal(err)
	}
	if err := s.mutate(c, Mutation{Action: "create", Revision: c.revision, Name: "escape", Source: "portfolio_page", Title: "Escape", Route: "escape"}, nil); err == nil {
		t.Fatal("wrote through symlink")
	}
}

func TestHTTPPermissionsAndUpload(t *testing.T) {
	s := testService(t)
	createTest(t, s, "dutch")
	c, sp := currentSpace(t, s, "dutch")
	m := saveMutation(c, sp)
	body, _ := json.Marshal(m)
	for _, tc := range []struct {
		name, origin, host, marker string
		write                      bool
		want                       int
	}{{"valid", "http://localhost", "localhost", "1", true, 200}, {"cross-origin", "https://evil.example", "localhost", "1", true, 403}, {"no-marker", "http://localhost", "localhost", "", true, 403}, {"host-rebinding", "http://evil.example", "evil.example", "1", true, 403}, {"readonly", "http://localhost", "localhost", "1", false, 403}} {
		t.Run(tc.name, func(t *testing.T) {
			s.write = tc.write
			req := httptest.NewRequest("POST", s.route+"/api", bytes.NewReader(body))
			req.Host = tc.host
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("X-Spaces-Request", tc.marker)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			s.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
		})
	}
	s.write = true
	c, sp = currentSpace(t, s, "dutch")
	m = saveMutation(c, sp)
	payload, _ := json.Marshal(m)
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)
	writer.WriteField("mutation", string(payload))
	writer.WriteField("field", fieldID(t, sp, "hero_image"))
	f, _ := writer.CreateFormFile("file", "photo.png")
	f.Write(pngBytes(t))
	writer.Close()
	r := httptest.NewRequest("POST", s.route+"/api/upload", &b)
	r.Host = "localhost"
	r.Header.Set("X-Spaces-Request", "1")
	r.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("upload %d %s", w.Code, w.Body.String())
	}
}

func TestTrashKeepsSourceVariableScope(t *testing.T) {
	s := testService(t)
	source := filepath.Join(s.dirs["hyperbricks"], "portfolio.hyperbricks.yaml")
	writeTestFile(t, source, "vars:\n  greeting: Scoped greeting\n"+fixtureSource)
	createTest(t, s, "dutch")
	c, sp := currentSpace(t, s, "dutch")
	file := filepath.Join(s.dirs["hyperbricks"], sp.File)
	writeTestFile(t, file, `dutch:
  - inherit: portfolio_page
  - route: dutch
  - title: Dutch
  - content:
      - values:
          body:
            - type: template
            - values:
                welcome: {var: greeting}
`)
	c, _ = currentSpace(t, s, "dutch")
	if err := s.mutate(c, Mutation{Action: "trash", Name: "dutch", Revision: c.revision}, nil); err != nil {
		t.Fatal(err)
	}
	_, sp = currentSpace(t, s, "dutch")
	for _, f := range sp.Fields {
		if f.Key == "welcome" && f.Value != "Scoped greeting" {
			t.Fatalf("lost variable scope in Trash: %q", f.Value)
		}
	}
}

func TestMetadataResetPreservesSourceAndUnrelatedComments(t *testing.T) {
	s := testService(t)
	createTest(t, s, "dutch")
	c, sp := currentSpace(t, s, "dutch")
	m := saveMutation(c, sp)
	m.Meta = map[string]*string{"author": nil}
	if err := s.mutate(c, m, nil); err != nil {
		t.Fatal(err)
	}
	c, sp = currentSpace(t, s, "dutch")
	m = saveMutation(c, sp)
	m.ResetMeta = []string{"author"}
	if err := s.mutate(c, m, nil); err != nil {
		t.Fatal(err)
	}
	_, sp = currentSpace(t, s, "dutch")
	if sp.Meta["author"] != "Alex" {
		t.Fatal("reset did not restore source")
	}
	index := filepath.Join(s.dirs["hyperbricks"], "spaces", "portfolio_page", "index.hyperbricks.yaml")
	writeTestFile(t, index, "# Index comment\nimports:\n  - 'dutch.hyperbricks.yaml' # keep inline\n")
	c = catalogTest(t, s)
	if err := s.mutate(c, Mutation{Action: "trash", Name: "dutch", Revision: c.revision}, nil); err != nil {
		t.Fatal(err)
	}
	c = catalogTest(t, s)
	if err := s.mutate(c, Mutation{Action: "restore", Name: "dutch", Revision: c.revision}, nil); err != nil {
		t.Fatal(err)
	}
	if fileText(t, index) != "# Index comment\nimports:\n  - 'dutch.hyperbricks.yaml' # keep inline\n" {
		t.Fatal("index formatting changed")
	}
}
