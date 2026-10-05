package analysis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeSourceRouteCacheLiteralsUseRuntimeValidation(t *testing.T) {
	for _, kind := range []string{"hypermedia", "fragment", "<HYPERMEDIA>", "FRAGMENT"} {
		for _, value := range []string{"eventually", "-1s", "''", "null", "true", "30", "0", "[]", "[30s]"} {
			t.Run(kind+"/"+value, func(t *testing.T) {
				issues := AnalyzeSource([]byte("page:\n  - type: "+kind+"\n  - cache: "+value+"\n"), SourceOptions{})
				cache := issuesByCodeList(issues, "component.invalid_cache")
				if len(cache) != 1 || cache[0].Path != "page.cache" || cache[0].Range.Start.Line != 3 || cache[0].Range.Start.Column != 12 || cache[0].Severity != SeverityError {
					t.Fatalf("literal %s: %#v (all %#v)", value, cache, issues)
				}
			})
		}
	}
}

func TestAnalyzeSourceRouteCacheMappingRanges(t *testing.T) {
	for _, tc := range []struct{ field, value string }{
		{"storage", "file"}, {"storage", "null"}, {"storage", "''"}, {"storage", "false"}, {"storage", "30"}, {"storage", "[]"},
		{"expire", "forever"}, {"expire", "-30s"}, {"expire", "null"}, {"expire", "''"}, {"expire", "false"}, {"expire", "0"}, {"expire", "[]"}, {"expire", "{}"},
	} {
		t.Run(tc.field+"/"+tc.value, func(t *testing.T) {
			input := "page:\n  - type: hypermedia\n  - cache:\n      " + tc.field + ": " + tc.value + "\n"
			issues := AnalyzeSource([]byte(input), SourceOptions{})
			cache := issuesByCodeList(issues, "component.invalid_cache")
			if len(cache) != 1 || cache[0].Path != "page.cache."+tc.field || cache[0].Range.Start.Line != 4 || cache[0].Range.Start.Column != 9+len(tc.field) || !strings.Contains(cache[0].Message, "cache."+tc.field) {
				t.Fatalf("mapping diagnostic = %#v (all %#v)", cache, issues)
			}
		})
	}
	issues := AnalyzeSource([]byte("page:\n  - type: fragment\n  - cache: {storage: file, expire: -1s}\n"), SourceOptions{})
	cache := issuesByCodeList(issues, "component.invalid_cache")
	if len(cache) != 2 || cache[0].Range.Start.Column != 22 || cache[1].Range.Start.Column != 36 {
		t.Fatalf("flow ranges = %#v", cache)
	}
}

func TestAnalyzeSourceRouteCacheAcceptedForms(t *testing.T) {
	for _, value := range []string{"30s", "0s", "{}", "{storage: mem}", "{storage: disk}", "{expire: 30s}", "{storage: disk, expire: 0s}"} {
		issues := AnalyzeSource([]byte("page:\n  - type: hypermedia\n  - cache: "+value+"\n"), SourceOptions{})
		if len(issues) != 0 {
			t.Errorf("valid %s: %#v", value, issues)
		}
	}
}

func TestAnalyzeSourceRouteCacheUnknownFieldsHaveOneOwner(t *testing.T) {
	issues := AnalyzeSource([]byte("page:\n  - type: fragment\n  - cache: {storag: disk, expires: 30s}\n"), SourceOptions{})
	unsupported := issuesByCodeList(issues, "component.unsupported_field")
	if len(unsupported) != 2 || len(issues) != 2 || unsupported[0].Path != "page.cache.storag" || unsupported[1].Path != "page.cache.expires" {
		t.Fatalf("duplicate/missing unknown-field findings: %#v", issues)
	}
	issues = AnalyzeSource([]byte("page:\n  - type: fragment\n  - cache: {storag: disk, expire: -1s}\n"), SourceOptions{})
	if len(issuesByCodeList(issues, "component.unsupported_field")) != 1 || len(issuesByCodeList(issues, "component.invalid_cache")) != 1 {
		t.Fatalf("known invalid field suppressed by unknown sibling: %#v", issues)
	}
}

func TestAnalyzeSourceRouteCacheDefersResolversWithoutHidingLiteralSiblings(t *testing.T) {
	t.Setenv("HB_ANALYSIS_CACHE_RUNTIME_VALUE", "invalid-only-at-runtime")
	for _, value := range []string{
		"{env: HB_ANALYSIS_CACHE_RUNTIME_VALUE}",
		"{config: {path: deployment.cache, default: 30s}}",
		"{storage: {env: HB_ANALYSIS_CACHE_RUNTIME_VALUE}, expire: 30s}",
		"{storage: disk, expire: {env: HB_ANALYSIS_CACHE_RUNTIME_VALUE}}",
		"{storage: disk, expire: {env: HB_ANALYSIS_CACHE_MISSING_847192}}",
	} {
		issues := AnalyzeSource([]byte("page:\n  - type: hypermedia\n  - cache: "+value+"\n"), SourceOptions{})
		if cache := issuesByCodeList(issues, "component.invalid_cache"); len(cache) != 0 {
			t.Errorf("resolver was falsely checked as literal: %s: %#v", value, issues)
		}
	}
	issues := AnalyzeSource([]byte("page:\n  - type: fragment\n  - cache: {storage: unknown, expire: {env: HB_ANALYSIS_CACHE_RUNTIME_VALUE}}\n"), SourceOptions{})
	cache := issuesByCodeList(issues, "component.invalid_cache")
	if len(cache) != 1 || cache[0].Path != "page.cache.storage" {
		t.Fatalf("resolver hid invalid literal sibling: %#v", issues)
	}
}

func TestAnalyzeSourceRouteCacheInheritanceKeepsSourceOwnership(t *testing.T) {
	dir := t.TempDir()
	mainPath, basePath := filepath.Join(dir, "app.hyperbricks.yaml"), filepath.Join(dir, "base.hyperbricks.yaml")
	main := []byte("imports: [base.hyperbricks.yaml]\npage:\n  - inherit: base\nother:\n  - inherit: base\n  - cache: {expire: 30s}\n")
	base := []byte("base:\n  - type: hypermedia\n  - cache: {storage: invalid, expire: 1m}\n")
	issues := AnalyzeSource(main, SourceOptions{Filename: mainPath, ReadFile: func(path string) ([]byte, error) {
		switch path {
		case mainPath:
			return main, nil
		case basePath:
			return base, nil
		default:
			return nil, os.ErrNotExist
		}
	}})
	cache := issuesByCodeList(issues, "component.invalid_cache")
	if len(cache) != 1 || cache[0].File != basePath || cache[0].Path != "base.cache.storage" || cache[0].Range.Start.Line != 3 {
		t.Fatalf("inherited invalid policy was duplicated or misattributed: %#v", issues)
	}
	issues = AnalyzeSource([]byte("base:\n  - type: fragment\n  - cache: {storage: disk, expire: 1m}\nchild:\n  - inherit: base\n  - cache: {expire: -1s}\n"), SourceOptions{})
	cache = issuesByCodeList(issues, "component.invalid_cache")
	if len(cache) != 1 || cache[0].Path != "child.cache.expire" || cache[0].Range.Start.Line != 6 {
		t.Fatalf("inherited effective type not validated: %#v", issues)
	}
}

func TestAnalyzeSourceRouteCacheDoesNotValidateOtherOwners(t *testing.T) {
	for _, source := range []string{
		"asset:\n  - type: esbuild\n  - cache: true\n",
		"api:\n  - type: api_fragment_render\n  - cache: false\n",
		"page:\n  - type: hypermedia\n  - cache: &policy 30s\nother:\n  - type: fragment\n  - cache: *policy\n",
	} {
		issues := AnalyzeSource([]byte(source), SourceOptions{})
		if cache := issuesByCodeList(issues, "component.invalid_cache"); len(cache) != 0 {
			t.Fatalf("other owner or unsupported YAML alias received cache diagnostic: %#v", issues)
		}
	}
}
