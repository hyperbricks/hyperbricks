package spaces

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func documentService(t *testing.T) (*service, string) {
	t.Helper()
	s := testService(t)
	source := strings.Replace(fixtureSource, "article_markdown: ''", "article_markdown: uploads/documents/shared.md", 1)
	source = strings.Replace(source, "article_markdown:\n                  type: asset", "article_markdown:\n                  type: asset\n                  edit: {type: markdown, max_bytes: 10000}\n                  order: 40\n                  group: Documents", 1)
	writeTestFile(t, filepath.Join(s.dirs["hyperbricks"], "portfolio.hyperbricks.yaml"), source)
	p := filepath.Join(s.dirs["resources"], "uploads/documents/shared.md")
	writeTestFile(t, p, "# Original\n\nShared document.\n")
	createTest(t, s, "dutch")
	createTest(t, s, "english")
	return s, p
}

func documentRequest(t *testing.T, s *service, name string) (*catalog, DocumentMutation, Document) {
	t.Helper()
	c, sp := currentSpace(t, s, name)
	id := fieldID(t, sp, "article_markdown")
	doc, err := s.document(c, name, id)
	if err != nil {
		t.Fatal(err)
	}
	return c, DocumentMutation{Action: "save", Name: name, Field: id, Revision: c.revision, FileRevision: doc.FileRevision, Content: "# Changed\n", Shared: true}, doc
}

func TestDocumentSharedEditCopyAndRelations(t *testing.T) {
	s, p := documentService(t)
	c, m, doc := documentRequest(t, s, "dutch")
	if !doc.Shared || len(doc.Usages) != 3 {
		t.Fatalf("missing source/Space relations: %+v", doc.Usages)
	}
	m.Shared = false
	if _, _, err := s.mutateDocument(c, m); err == nil {
		t.Fatal("shared edit not confirmed")
	}
	m.Shared = true
	before := fileText(t, filepath.Join(s.dirs["hyperbricks"], "spaces/portfolio_page/dutch.hyperbricks.yaml"))
	updated, _, err := s.mutateDocument(c, m)
	if err != nil {
		t.Fatal(err)
	}
	if fileText(t, p) != m.Content || updated.FileRevision == doc.FileRevision {
		t.Fatal("file was not saved")
	}
	if before != fileText(t, filepath.Join(s.dirs["hyperbricks"], "spaces/portfolio_page/dutch.hyperbricks.yaml")) {
		t.Fatal("shared edit rewrote YAML")
	}
	c, m, doc = documentRequest(t, s, "dutch")
	m.Action, m.Content = "copy", "# My own document\n"
	copy, _, err := s.mutateDocument(c, m)
	if err != nil {
		t.Fatal(err)
	}
	if copy.Shared || copy.Reference == doc.Reference || len(copy.Usages) != 1 {
		t.Fatalf("copy not isolated: %+v", copy)
	}
	if fileText(t, p) != "# Changed\n" {
		t.Fatal("copy overwrote shared file")
	}
	if fileText(t, filepath.Join(s.dirs["resources"], copy.Reference)) != m.Content {
		t.Fatal("copy missing")
	}
	c = catalogTest(t, s)
	if err := s.mutate(c, Mutation{Action: "trash", Name: "english", Revision: c.revision}, nil); err != nil {
		t.Fatal(err)
	}
	c = catalogTest(t, s)
	uses, err := s.fileUsages(c, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(uses) != 2 || !uses[0].Trashed {
		t.Fatalf("trash relation not retained: %+v", uses)
	}
}

func TestDocumentRevisionsAndPermissions(t *testing.T) {
	t.Run("file revision independent of YAML", func(t *testing.T) {
		s, p := documentService(t)
		c, m, _ := documentRequest(t, s, "dutch")
		writeTestFile(t, p, "# External edit\n")
		fresh := catalogTest(t, s)
		if fresh.revision != c.revision {
			t.Fatal("expected unchanged YAML revision")
		}
		_, _, err := s.mutateDocument(fresh, m)
		var status *statusError
		if !errors.As(err, &status) || status.status != 409 {
			t.Fatalf("expected conflict: %v", err)
		}
		if fileText(t, p) != "# External edit\n" {
			t.Fatal("external content lost")
		}
	})
	t.Run("new relations invalidate confirmation", func(t *testing.T) {
		s, _ := documentService(t)
		_, m, _ := documentRequest(t, s, "dutch")
		createTest(t, s, "third")
		if _, _, err := s.mutateDocument(catalogTest(t, s), m); err == nil {
			t.Fatal("accepted stale references")
		}
	})
	t.Run("readonly", func(t *testing.T) {
		s, _ := documentService(t)
		c, m, _ := documentRequest(t, s, "dutch")
		s.write = false
		if _, _, err := s.mutateDocument(c, m); err == nil {
			t.Fatal("readonly write accepted")
		}
	})
	t.Run("instance cannot grant edit", func(t *testing.T) {
		s, _ := documentService(t)
		_, m, _ := documentRequest(t, s, "dutch")
		p := filepath.Join(s.dirs["hyperbricks"], "portfolio.hyperbricks.yaml")
		writeTestFile(t, p, strings.Replace(fileText(t, p), "edit: {type: markdown, max_bytes: 10000}", "help: Upload only", 1))
		if _, err := s.document(catalogTest(t, s), m.Name, m.Field); err == nil {
			t.Fatal("edit without source permission accepted")
		}
	})
	t.Run("edit independent from upload", func(t *testing.T) {
		s, _ := documentService(t)
		p := filepath.Join(s.dirs["hyperbricks"], "portfolio.hyperbricks.yaml")
		src := strings.Replace(fileText(t, p), "upload:\n                    accept: [.md, .markdown]\n                    max_bytes: 10000\n                    directory: {base: resources, path: uploads/documents}", "directory: {base: resources, path: uploads/documents}", 1)
		writeTestFile(t, p, src)
		c, m, _ := documentRequest(t, s, "dutch")
		m.Action = "copy"
		if _, _, err := s.mutateDocument(c, m); err != nil {
			t.Fatal(err)
		}
		if _, err := s.prepareUpload(catalogTest(t, s), m.Name, m.Field, "no.md", []byte("hello")); err == nil {
			t.Fatal("edit permission enabled uploads")
		}
	})
	for _, bad := range []string{"../outside.md", "uploads/documents/../../outside.md", "uploads/documents/link.md"} {
		t.Run(bad, func(t *testing.T) {
			s, p := documentService(t)
			if err := os.Symlink(p, filepath.Join(filepath.Dir(p), "link.md")); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(s.dirs["hyperbricks"], "spaces/portfolio_page/dutch.hyperbricks.yaml")
			writeTestFile(t, file, strings.Replace(fileText(t, file), "uploads/documents/shared.md", bad, 1))
			c, sp := currentSpace(t, s, "dutch")
			if _, err := s.document(c, sp.Name, fieldID(t, sp, "article_markdown")); err == nil {
				t.Fatal("unsafe path accepted")
			}
		})
	}
	t.Run("trash", func(t *testing.T) {
		s, _ := documentService(t)
		c, m, _ := documentRequest(t, s, "dutch")
		if err := s.mutate(c, Mutation{Action: "trash", Name: m.Name, Revision: c.revision}, nil); err != nil {
			t.Fatal(err)
		}
		if _, err := s.document(catalogTest(t, s), m.Name, m.Field); err == nil {
			t.Fatal("trashed document editable")
		}
	})
}

func TestDocumentPreviewAndHTTP(t *testing.T) {
	s, _ := documentService(t)
	c, m, _ := documentRequest(t, s, "dutch")
	m.Action = "preview"
	m.Content = "# Preview\n\n<script>alert(1)</script>\n\n[bad](javascript:alert(1))\n\n<img src=x onerror=alert(1)>"
	_, html, err := s.mutateDocument(c, m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "<h1>Preview</h1>") || strings.Contains(html, "<script>") || strings.Contains(html, "onerror=") || strings.Contains(html, `href="javascript:`) {
		t.Fatalf("unsafe preview: %s", html)
	}
	for _, content := range []string{strings.Repeat("a", 10001), "bad\x00data"} {
		m.Content = content
		if _, _, err := s.mutateDocument(c, m); err == nil {
			t.Fatal("invalid content accepted")
		}
	}
	m.Content = "# HTTP save\n"
	m.Action = "save"
	b, _ := json.Marshal(m)
	for _, origin := range []string{"http://localhost", "https://hostile.example"} {
		r := httptest.NewRequest("POST", "http://localhost"+s.route+"/api/document", strings.NewReader(string(b)))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Spaces-Request", "1")
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		want := 200
		if origin != "http://localhost" {
			want = 403
		}
		if w.Code != want {
			t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
		}
	}
}

func TestEditorialOrderGroupAndEditSchema(t *testing.T) {
	s, _ := documentService(t)
	_, sp := currentSpace(t, s, "dutch")
	last := sp.Fields[len(sp.Fields)-1]
	if last.Key != "article_markdown" || last.Group != "Documents" || last.Edit == nil {
		t.Fatalf("order/group/edit: %+v", last)
	}
	for _, schema := range []string{
		"edit: {type: html, max_bytes: 100}", "edit: {type: markdown, max_bytes: 0}",
		"edit: {type: markdown, max_bytes: 1048577}", "edit: {type: markdown, max_bytes: 100, execute: true}",
	} {
		t.Run(schema, func(t *testing.T) {
			s := testService(t)
			p := filepath.Join(s.dirs["hyperbricks"], "portfolio.hyperbricks.yaml")
			writeTestFile(t, p, strings.Replace(fixtureSource, "type: asset\n                  upload:", "type: asset\n                  "+schema+"\n                  upload:", 1))
			if _, err := s.snapshot(catalogTest(t, s)); err == nil {
				t.Fatal("invalid edit schema accepted")
			}
		})
	}
}
