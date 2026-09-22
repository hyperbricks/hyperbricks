package component

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
)

func markdownText(v string) *string { return &v }

func TestMarkdownContentAndErrors(t *testing.T) {
	r := &MarkdownRenderer{}
	meta := shared.Component{Meta: shared.Meta{HyperBricksFile: "page.yaml", HyperBricksPath: "page.body", HyperBricksKey: "body"}, Enclose: "<article>|</article>"}
	out, errs := r.Render(MarkdownConfig{Component: meta, Content: markdownText("**Hello**"), Class: `x" onclick="alert(1)`}, nil)
	if len(errs) != 0 || !strings.Contains(out, "<strong>Hello</strong>") || !strings.Contains(out, `class="x&#34; onclick=&#34;alert(1)"`) || !strings.HasPrefix(out, "<article>") {
		t.Fatalf("out=%s errors=%v", out, errs)
	}
	out, errs = r.Render(MarkdownConfig{Content: markdownText("")}, nil)
	if out != "" || len(errs) != 0 {
		t.Fatalf("empty content: %q %v", out, errs)
	}
	for _, config := range []MarkdownConfig{
		{}, {Content: markdownText("x"), File: "x.md"},
		{Content: markdownText("abcd"), MaxBytes: 3},
		{Content: markdownText("x"), MaxBytes: -1},
		{Content: markdownText("x"), MaxBytes: MaxMarkdownBytes + 1},
		{Content: markdownText("a\x00b")}, {Content: markdownText(string([]byte{0xff}))},
	} {
		config.Component = meta
		out, errs := r.Render(config, nil)
		if out != "" || len(errs) != 1 {
			t.Fatalf("invalid config accepted: %+v %s %v", config, out, errs)
		}
		e, ok := errs[0].(shared.ComponentError)
		if !ok || !e.Rejected || e.File != "page.yaml" || e.Path != "page.body" || e.Key != "body" || e.Type != "<MARKDOWN>" {
			t.Fatalf("missing source context: %+v", errs)
		}
	}
	if _, errs := r.Render("invalid", nil); len(errs) != 1 {
		t.Fatal("accepted wrong config type")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, errs := r.Render(MarkdownConfig{Content: markdownText("text")}, ctx); len(errs) != 1 {
		t.Fatal("ignored cancellation")
	}
}

func TestMarkdownFilesAndNoRequestSelection(t *testing.T) {
	root := t.TempDir()
	r := NewMarkdownRenderer(root)
	write := func(name string, data []byte) {
		t.Helper()
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("docs/read.md", []byte("# Declared"))
	write("docs/other.md", []byte("# Other"))
	write("docs/big.md", []byte(strings.Repeat("x", int(DefaultMarkdownMaxBytes)+1)))
	write("docs/binary.md", []byte{0xff})
	write("docs/nul.md", []byte("a\x00b"))
	write("docs/empty.markdown", nil)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.md"), []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), shared.Request, httptest.NewRequest("GET", "/?file=docs/other.md", nil))
	for _, want := range []string{"Declared", "Updated"} {
		out, errs := r.Render(MarkdownConfig{File: "docs/read.md"}, ctx)
		if len(errs) != 0 || !strings.Contains(out, want) || strings.Contains(out, "Other") {
			t.Fatalf("out=%s errors=%v", out, errs)
		}
		write("docs/read.md", []byte("# Updated"))
	}
	for _, name := range []string{"../secret.md", filepath.Join(outside, "secret.md"), "escape/secret.md", "docs/../docs/read.md", `docs\read.md`, "https://example.com/a.md", "docs/missing.md", "docs/read.txt", "docs/big.md", "docs/binary.md", "docs/nul.md", "docs"} {
		if out, errs := r.Render(MarkdownConfig{File: name}, nil); out != "" || len(errs) == 0 {
			t.Fatalf("accepted %q: %s", name, out)
		}
	}
	if out, errs := r.Render(MarkdownConfig{File: "docs/empty.markdown"}, nil); out != "" || len(errs) != 0 {
		t.Fatalf("empty file: %s %v", out, errs)
	}
	if _, errs := (&MarkdownRenderer{}).Render(MarkdownConfig{File: "docs/read.md"}, nil); len(errs) != 1 {
		t.Fatal("file read without configured root")
	}
	if _, errs := r.Render(MarkdownConfig{File: "docs/read.md", MaxBytes: 2}, nil); len(errs) != 1 {
		t.Fatal("file limit ignored")
	}
}

func TestMarkdownFactoryValidation(t *testing.T) {
	f := typefactory.NewTypeFactory()
	f.RegisterType(MarkdownConfigGetName(), reflect.TypeOf(MarkdownConfig{}))
	for _, raw := range []map[string]interface{}{
		{"content": ""}, {"content": "**Hello**", "max_bytes": "100"}, {"file": "docs/read.md", "max_bytes": int64(100)},
	} {
		if _, err := f.CreateInstance(typefactory.TypeRequest{TypeName: MarkdownConfigGetName(), Data: raw}); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []map[string]interface{}{
		{}, {"content": "x", "file": ""}, {"content": "x", "file": "docs/read.md"},
		{"content": nil}, {"content": []string{"x"}}, {"file": 42}, {"content": "x", "max_bytes": 1.5},
		{"content": "x", "max_bytes": nil}, {"content": "x", "max_bytes": "0"}, {"content": "x", "max_bytes": "-1"},
	} {
		if _, err := f.CreateInstance(typefactory.TypeRequest{TypeName: MarkdownConfigGetName(), Data: raw}); err == nil {
			t.Fatalf("accepted %+v", raw)
		}
	}
}
