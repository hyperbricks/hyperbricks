package spaces

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
)

const nativeMarkdownSource = `portfolio_page:
  - type: hypermedia
  - route: index
  - title: Portfolio
  - content:
      - type: markdown
      - file: uploads/documents/shared.md
      - max_bytes: 10000
      - editable:
          file:
            type: asset
            label: Document
            required: true
            edit: {type: markdown, max_bytes: 10000}
            upload:
              accept: [.md, .markdown]
              max_bytes: 10000
              directory: {base: resources, path: uploads/documents}
`

func TestNativeMarkdownNestedReferences(t *testing.T) {
	s := testService(t)
	writeTestFile(t, filepath.Join(s.dirs["hyperbricks"], "portfolio.hyperbricks.yaml"), `
native_document:
  - type: markdown
  - file: uploads/documents/perspective.md
  - max_bytes: 10000
  - editable:
      file:
        type: asset
        label: Document
        required: true
        edit: {type: markdown, max_bytes: 10000}
        upload:
          accept: [.md, .markdown]
          max_bytes: 10000
          directory: {base: resources, path: uploads/documents}

portfolio_page:
  - type: hypermedia
  - route: index
  - title: Portfolio
  - content:
      - type: template
      - inline: '<main>{{.body}}</main>'
      - values:
          body:
            - type: template
            - inline: '<article>{{.article_markdown}}</article>'
            - values:
                article_markdown:
                  - inherit: native_document

perspective_document:
  - type: fragment
  - route: perspective/document
  - document:
      - inherit: native_document
`)
	writeTestFile(t, filepath.Join(s.dirs["resources"], "uploads/documents/perspective.md"), "# Perspective\n")
	createTest(t, s, "first")
	createTest(t, s, "second")
	c := catalogTest(t, s)
	snapshot, err := s.snapshot(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Sources) == 0 || len(snapshot.Spaces) == 0 {
		t.Fatal("fixture sources and spaces must load")
	}
	for _, sp := range snapshot.Spaces {
		if sp.Trashed {
			continue
		}
		id := fieldID(t, sp, "file")
		if id != "/content/values/body/values/article_markdown/file" {
			t.Fatalf("%s: unexpected document path %s", sp.Name, id)
		}
		doc, err := s.document(c, sp.Name, id)
		if err != nil || doc.Reference == "" {
			t.Fatalf("%s: document relation %+v: %v", sp.Name, doc, err)
		}
		out, errs := component.NewMarkdownRenderer(s.dirs["resources"]).Render(component.MarkdownConfig{File: doc.Reference}, nil)
		if len(errs) != 0 || out == "" {
			t.Fatalf("%s: native render %q: %v", sp.Name, out, errs)
		}
	}
	fragment, ok := c.defs["perspective_document"]
	if !ok {
		t.Fatal("missing explicit document fragment")
	}
	document, ok := fragment.effective["document"].(map[string]interface{})
	if !ok || document["@type"] != "<MARKDOWN>" || document["file"] != "uploads/documents/perspective.md" {
		t.Fatalf("fragment did not inherit the native document: %+v", document)
	}
}

func TestNativeMarkdownSpacesWorkflow(t *testing.T) {
	s := testService(t)
	writeTestFile(t, filepath.Join(s.dirs["hyperbricks"], "portfolio.hyperbricks.yaml"), nativeMarkdownSource)
	p := filepath.Join(s.dirs["resources"], "uploads/documents/shared.md")
	writeTestFile(t, p, "# Original\n")
	createTest(t, s, "first")
	createTest(t, s, "second")
	c, sp := currentSpace(t, s, "first")
	id := fieldID(t, sp, "file")
	if id != "/content/file" {
		t.Fatalf("field did not target component file: %s", id)
	}
	doc, err := s.document(c, sp.Name, id)
	if err != nil || !doc.Shared || len(doc.Usages) != 3 {
		t.Fatalf("relations: %+v %v", doc, err)
	}
	m := DocumentMutation{Action: "copy", Name: sp.Name, Field: id, Revision: c.revision, FileRevision: doc.FileRevision, Content: "# Own document\n"}
	copy, _, err := s.mutateDocument(c, m)
	if err != nil || copy.Shared || copy.Reference == doc.Reference {
		t.Fatalf("copy: %+v %v", copy, err)
	}
	if fileText(t, p) != "# Original\n" {
		t.Fatal("copy overwrote source document")
	}
	factory := typefactory.NewTypeFactory()
	factory.RegisterType(component.MarkdownConfigGetName(), reflect.TypeOf(component.MarkdownConfig{}))
	checkRendered := func(want string) {
		t.Helper()
		c := catalogTest(t, s)
		raw := c.defs["first"].effective["content"].(map[string]interface{})
		instance, err := factory.CreateInstance(typefactory.TypeRequest{TypeName: component.MarkdownConfigGetName(), Data: raw})
		if err != nil {
			t.Fatal(err)
		}
		out, errs := component.NewMarkdownRenderer(s.dirs["resources"]).Render(instance.Instance, nil)
		if len(errs) != 0 || !strings.Contains(out, want) {
			t.Fatalf("saved reference not rendered: %s %v", out, errs)
		}
	}
	checkRendered("Own document")
	c, sp = currentSpace(t, s, "first")
	upload, err := s.prepareUpload(c, sp.Name, id, "new.md", []byte("# Uploaded\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.mutate(c, Mutation{Action: "save", Name: sp.Name, Title: sp.Title, Route: sp.Route, Revision: c.revision}, upload); err != nil {
		t.Fatal(err)
	}
	checkRendered("Uploaded")
	c, sp = currentSpace(t, s, "first")
	doc, err = s.document(c, sp.Name, id)
	if err != nil {
		t.Fatal(err)
	}
	m = DocumentMutation{Action: "save", Name: sp.Name, Field: id, Revision: c.revision, FileRevision: doc.FileRevision, Content: "# Edited\n"}
	if _, _, err := s.mutateDocument(c, m); err != nil {
		t.Fatal(err)
	}
	checkRendered("Edited")
	c, sp = currentSpace(t, s, "first")
	for _, bad := range []string{"", "../outside.md", "uploads/documents/wrong.png"} {
		if err := s.mutate(c, Mutation{Action: "save", Name: sp.Name, Title: sp.Title, Route: sp.Route, Revision: c.revision, Values: map[string]string{id: bad}}, nil); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	if err := s.mutate(c, Mutation{Action: "trash", Name: sp.Name, Revision: c.revision}, nil); err != nil {
		t.Fatal(err)
	}
	c = catalogTest(t, s)
	if _, active := c.defs[sp.Name]; active {
		t.Fatal("trashed space is still active")
	}
	if _, err := s.document(c, sp.Name, id); err == nil {
		t.Fatal("trashed document remained editable")
	}
	if err := s.mutate(c, Mutation{Action: "restore", Name: sp.Name, Revision: c.revision}, nil); err != nil {
		t.Fatal(err)
	}
	checkRendered("Edited")
}

func TestNativeMarkdownEditableRestrictions(t *testing.T) {
	for _, source := range []string{
		strings.Replace(nativeMarkdownSource, "type: markdown\n      - file:", "type: html\n      - file:", 1),
		strings.Replace(nativeMarkdownSource, "file:\n            type: asset", "class:\n            type: asset", 1),
		strings.Replace(nativeMarkdownSource, "type: asset", "type: text", 1),
		strings.Replace(nativeMarkdownSource, "base: resources", "base: static", 1),
		strings.Replace(nativeMarkdownSource, "[.md, .markdown]", "[.png]", 1),
		strings.Replace(nativeMarkdownSource, "max_bytes: 10000\n      - editable:", "max_bytes: 100\n      - editable:", 1),
		strings.Replace(nativeMarkdownSource, "- max_bytes: 10000", "- content: ambiguous\n      - max_bytes: 10000", 1),
	} {
		s := testService(t)
		writeTestFile(t, filepath.Join(s.dirs["hyperbricks"], "portfolio.hyperbricks.yaml"), source)
		if _, err := s.snapshot(catalogTest(t, s)); err == nil {
			t.Fatalf("accepted schema:\n%s", source)
		}
	}
	s := testService(t)
	writeTestFile(t, filepath.Join(s.dirs["hyperbricks"], "portfolio.hyperbricks.yaml"), `portfolio_page:
  - type: hypermedia
  - route: index
  - content:
      - type: markdown
      - content: '# Inline'
      - max_bytes: 40
      - editable:
          content: {type: textarea, label: Markdown, rows: 8}
`)
	createTest(t, s, "inline")
	c, sp := currentSpace(t, s, "inline")
	id := fieldID(t, sp, "content")
	if id != "/content/content" {
		t.Fatal(id)
	}
	m := Mutation{Action: "save", Name: sp.Name, Title: sp.Title, Route: sp.Route, Revision: c.revision, Values: map[string]string{id: "# Edited\n\n**Content**"}}
	if err := s.mutate(c, m, nil); err != nil {
		t.Fatal(err)
	}
	c, sp = currentSpace(t, s, "inline")
	m.Revision, m.Values[id] = c.revision, strings.Repeat("x", 41)
	if err := s.mutate(c, m, nil); err == nil {
		t.Fatal("content exceeded renderer limit")
	}
}
