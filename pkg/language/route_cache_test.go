package language

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/composite"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

func TestRouteCacheStorageValueCompletions(t *testing.T) {
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	for _, kind := range []string{"hypermedia", "fragment"} {
		for _, test := range []struct {
			name, cache string
			prefixed    bool
		}{
			{"block", "\n      storage: «cursor»\n      expire: 30s\n", false},
			{"flow", "{storage: «cursor», expire: 30s} # keep\n", false},
			{"empty quoted", "{storage: '«cursor»', expire: 30s} # keep\n", false},
			{"prefix", "{storage: di«cursor», expire: 30s} # keep\n", true},
			{"quoted prefix", "{storage: 'd«cursor»isk', expire: 30s} # keep\n", true},
			{"double quoted prefix", "{storage: \"d«cursor»isk\", expire: 30s} # keep\n", true},
		} {
			t.Run(kind+"/"+test.name, func(t *testing.T) {
				source, position := completionTestPosition(t, "page:\n  - type: "+kind+"\n  - cache: "+test.cache)
				items := analyzer.Completions("untitled:page", source, position, nil)
				item, found := completionByLabel(items, "disk")
				if !found || hasCompletion(items, "mem") == test.prefixed {
					t.Fatalf("storage candidates = %#v", items)
				}
				if !test.prefixed {
					for _, resolver := range []string{"env", "var", "config"} {
						if !hasCompletion(items, resolver) {
							t.Errorf("empty storage value lost %s resolver completion", resolver)
						}
					}
				}
				completed := applyReviewCompletion(t, source, item)
				policy := completedRouteCachePolicy(t, completed)
				if policy.Storage != "disk" || policy.Expire != 30*time.Second {
					t.Fatalf("completed policy = %#v\n%s", policy, completed)
				}
				if strings.Contains(source, "# keep") && !strings.Contains(completed, "# keep") {
					t.Fatalf("completion removed comment:\n%s", completed)
				}
			})
		}
	}
}

func TestRouteCacheInheritedStorageCompletions(t *testing.T) {
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	for _, kind := range []string{"hypermedia", "fragment"} {
		source, position := completionTestPosition(t, "base:\n  - type: "+kind+"\n  - cache: {expire: 2m}\npage:\n  - inherit: base\n  - cache: {storage: «cursor»}\n")
		items := analyzer.Completions("untitled:page", source, position, nil)
		item, found := completionByLabel(items, "disk")
		if !found || !hasCompletion(items, "mem") {
			t.Fatalf("inherited %s storage candidates = %#v", kind, items)
		}
		policy := completedRouteCachePolicy(t, applyReviewCompletion(t, source, item))
		if policy.Storage != "disk" || policy.Expire != 2*time.Minute {
			t.Fatalf("inherited completed policy = %#v", policy)
		}
	}
}

func TestRouteCacheSnippetsHaveValidDefaults(t *testing.T) {
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	for _, kind := range []string{"hypermedia", "fragment"} {
		for _, test := range []struct {
			name, body, label, snippet string
			storage                    string
		}{
			{"whole block", "  - «cursor»\n", "cache", "cache:\n      expire: ${1:30s}\n      storage: ${2|mem,disk|}", "mem"},
			{"block storage", "  - cache:\n      expire: 30s\n      «cursor»\n", "storage", "storage: ${1|mem,disk|}", "mem"},
			{"block expire", "  - cache:\n      storage: disk\n      «cursor»\n", "expire", "expire: ${1:30s}", "disk"},
			{"flow storage", "  - cache: {expire: 30s, «cursor»}\n", "storage", "storage: ${1|mem,disk|}", "mem"},
			{"flow expire", "  - cache: {storage: disk, «cursor»}\n", "expire", "expire: ${1:30s}", "disk"},
		} {
			t.Run(kind+"/"+test.name, func(t *testing.T) {
				source, position := completionTestPosition(t, "page:\n  - type: "+kind+"\n"+test.body)
				item, found := completionByLabel(analyzer.Completions("untitled:page", source, position, nil), test.label)
				if !found || item.InsertText != test.snippet || item.TextEdit == nil {
					t.Fatalf("snippet = %#v; want %q", item, test.snippet)
				}
				item.TextEdit.NewText = regexp.MustCompile(`\$\{[0-9]+:([^}]*)\}`).ReplaceAllString(item.TextEdit.NewText, "$1")
				item.TextEdit.NewText = regexp.MustCompile(`\$\{[0-9]+\|([^,|]+)[^}]*\}`).ReplaceAllString(item.TextEdit.NewText, "$1")
				completed := applyReviewCompletion(t, source, item)
				policy := completedRouteCachePolicy(t, completed)
				if policy.Storage != test.storage || policy.Expire != 30*time.Second {
					t.Fatalf("snippet produced invalid default policy: %#v\n%s", policy, completed)
				}
			})
		}
	}
}

func TestRouteCacheHoverAndScalarCompatibility(t *testing.T) {
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	for _, test := range []struct {
		marked string
		want   string
	}{
		{"page:\n  - type: hypermedia\n  - ca«cursor»che: 30s\n", "a duration such as 30s"},
		{"page:\n  - type: fragment\n  - cache: {sto«cursor»rage: disk, expire: 30s}\n", "mem (default) or disk"},
		{"page:\n  - type: fragment\n  - cache:\n      storage: disk\n      exp«cursor»ire: 30s\n", "Example: `30s`"},
	} {
		source, position := completionTestPosition(t, test.marked)
		hover := analyzer.Hover("untitled:page", source, position)
		if hover == nil || !strings.Contains(hover.Contents.Value, test.want) {
			t.Fatalf("hover = %#v; want %q", hover, test.want)
		}
		if diagnostics := analyzer.Diagnostics("untitled:page", source, nil); len(diagnostics) != 0 {
			t.Fatalf("valid scalar/mapping cache rejected: %#v", diagnostics)
		}
	}
}

func TestRouteCacheCompletionsRespectOtherComponentOwnership(t *testing.T) {
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	for _, kind := range []string{"api_fragment_render", "esbuild"} {
		for _, value := range []string{"«cursor»", "{«cursor»}", "{storage: «cursor»}"} {
			source, position := completionTestPosition(t, fmt.Sprintf("page:\n  - type: %s\n  - cache: %s\n", kind, value))
			items := analyzer.Completions("untitled:page", source, position, nil)
			for _, unsupported := range []string{"mem", "disk", "storage", "expire"} {
				if hasCompletion(items, unsupported) {
					t.Fatalf("%s received route-only completion %s", kind, unsupported)
				}
			}
			if kind == "esbuild" && value == "«cursor»" && (!hasCompletion(items, "true") || !hasCompletion(items, "false")) {
				t.Fatalf("esbuild lost Boolean cache suggestions: %#v", items)
			}
		}
	}
	// The duration example is narrowly owned by cache.expire. Other string
	// fields keep their existing placeholders even when their schema has examples.
	source, position := completionTestPosition(t, "page:\n  - type: hypermedia\n  - «cursor»\n")
	item, found := completionByLabel(analyzer.Completions("untitled:page", source, position, nil), "title")
	if !found || item.InsertText != "title: ${1:value}" {
		t.Fatalf("unrelated example changed title snippet: %#v", item)
	}
}

func TestSchemaAllowedValuesAlsoCompleteExistingMenuEnums(t *testing.T) {
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	for _, test := range []struct {
		field, snippet string
		values         []string
	}{
		{"order", "order: ${1|asc,desc|}", []string{"asc", "desc"}},
		{"sort", "sort: ${1|title,route,index|}", []string{"title", "route", "index"}},
	} {
		source, position := completionTestPosition(t, "menu:\n  - type: menu\n  - "+test.field+": «cursor»\n")
		items := analyzer.Completions("untitled:menu", source, position, nil)
		for _, value := range test.values {
			if !hasCompletion(items, value) {
				t.Errorf("menu.%s missing %s: %#v", test.field, value, items)
			}
		}
		source, position = completionTestPosition(t, "menu:\n  - type: menu\n  - «cursor»\n")
		item, found := completionByLabel(analyzer.Completions("untitled:menu", source, position, nil), test.field)
		if !found || item.InsertText != test.snippet {
			t.Errorf("menu.%s snippet = %#v", test.field, item)
		}
	}
}

func completedRouteCachePolicy(t *testing.T, source string) composite.RouteCachePolicy {
	t.Helper()
	result, err := yamlparser.ProcessBytes([]byte(source), yamlparser.Options{})
	if err != nil {
		t.Fatalf("completion produced invalid YAML: %v\n%s", err, source)
	}
	route, ok := result.Materialized["page"].(map[string]interface{})
	if !ok {
		t.Fatalf("completion lost page route: %#v", result.Materialized)
	}
	policy, err := composite.ResolveRouteCache(route, time.Minute)
	if err != nil {
		t.Fatalf("completion produced invalid cache policy: %v\n%s", err, source)
	}
	return policy
}
