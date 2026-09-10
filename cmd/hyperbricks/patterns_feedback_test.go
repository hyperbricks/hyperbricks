package main

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Masterminds/sprig/v3"
)

func TestPatternsAPIFeedbackHandlesUnavailableAndMalformedBackend(t *testing.T) {
	for _, name := range []string{"api-file-write-result.html", "route-split-api-result.html"} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join("..", "..", "modules", "hyperbricks-patterns-yaml", "templates", "patterns", name))
			if err != nil {
				t.Fatal(err)
			}
			tmpl, err := template.New(name).Funcs(sprig.HtmlFuncMap()).Parse(string(source))
			if err != nil {
				t.Fatal(err)
			}
			for _, data := range []any{nil, "backend unavailable", []any{}} {
				var out bytes.Buffer
				if err := tmpl.Execute(&out, map[string]any{"Data": data, "Status": 502}); err != nil {
					t.Fatalf("data=%#v: %v", data, err)
				}
				if !strings.Contains(out.String(), `data-state="error"`) {
					t.Fatalf("missing error feedback: %s", out.String())
				}
			}
			for _, tc := range []struct {
				data   any
				status int
				state  string
			}{
				{[]any{map[string]any{"message": "saved", "saved_path": "demo.html", "project_slug": "demo", "updated_at": "now", "asset_id": 42, "path": "demo.html"}}, 200, "success"},
				{map[string]any{"message": "conflict", "code": "23505", "hint": "retry", "details": "locked"}, 409, "error"},
			} {
				var out bytes.Buffer
				if err := tmpl.Execute(&out, map[string]any{"Data": tc.data, "Status": tc.status}); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(out.String(), `data-state="`+tc.state+`"`) {
					t.Fatalf("feedback = %s", out.String())
				}
			}
		})
	}
}
