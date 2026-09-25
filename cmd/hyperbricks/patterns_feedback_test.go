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

func TestPatternsAPIConflictFeedbackHandlesUnavailableAndMalformedBackend(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "modules", "hyperbricks-patterns-yaml", "templates", "patterns", "route-split-api-conflict-result.html"))
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := template.New("conflict").Funcs(sprig.HtmlFuncMap()).Parse(string(source))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		data   any
		status int
		want   string
	}{
		{"mock conflict", map[string]any{"message": "Asset already exists"}, 200, "Asset already exists"},
		{"upstream conflict", map[string]any{"message": "Asset already exists"}, 409, "Asset already exists"},
		{"unavailable", nil, 502, "Check PATTERNS_API_BASE_URL"},
		{"text", "backend unavailable", 502, "Check PATTERNS_API_BASE_URL"},
		{"array", []any{map[string]any{"message": "wrong shape"}}, 200, "Check PATTERNS_API_BASE_URL"},
		{"missing message", map[string]any{}, 200, "Check PATTERNS_API_BASE_URL"},
		{"empty message", map[string]any{"message": ""}, 200, "Check PATTERNS_API_BASE_URL"},
		{"non-string message", map[string]any{"message": 123}, 200, "Check PATTERNS_API_BASE_URL"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := tmpl.Execute(&out, map[string]any{"Data": tc.data, "Status": tc.status}); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), `data-state="error"`) || !strings.Contains(out.String(), tc.want) {
				t.Fatalf("feedback = %s; want error state and %q", out.String(), tc.want)
			}
		})
	}
}
