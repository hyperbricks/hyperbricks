package language

import (
	"strings"
	"testing"
)

func TestAnalyzerReportsInvalidRouteCacheValues(t *testing.T) {
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	for _, typ := range []string{"hypermedia", "fragment"} {
		for _, tc := range []struct {
			name, cache, field string
			line, character    int
		}{
			{"scalar", "  - cache: eventually\n", "cache", 2, 11},
			{"storage", "  - cache:\n      storage: banana\n", "cache.storage", 3, 15},
			{"expiry", "  - cache:\n      expire: nonsense\n", "cache.expire", 3, 14},
		} {
			t.Run(typ+"/"+tc.name, func(t *testing.T) {
				source := "page:\n  - type: " + typ + "\n" + tc.cache
				diagnostics := analyzer.Diagnostics("untitled:cache", source, nil)
				if len(diagnostics) != 1 {
					t.Fatalf("diagnostics = %+v; want one invalid cache value", diagnostics)
				}
				diagnostic := diagnostics[0]
				if diagnostic.Severity != DiagnosticSeverityError || !strings.Contains(diagnostic.Message, tc.field) ||
					diagnostic.Range.Start != (Position{Line: tc.line, Character: tc.character}) {
					t.Fatalf("diagnostic = %+v; want error for %s at %d:%d", diagnostic, tc.field, tc.line, tc.character)
				}
			})
		}
	}
}
