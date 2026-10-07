package language

import (
	"path/filepath"
	"testing"
)

func TestPackageImportsValidatePendingFragmentsInEntryContext(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "package.hyperbricks.yaml")
	fragment := filepath.Join(root, "config", "runtime.yaml")
	writeResourceAnalysisFile(t, entry, "imports: [config/runtime.yaml]\n")
	writeResourceAnalysisFile(t, fragment, "hyperbricks: {mode: development}\n")
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	bad := "hyperbricks: {mode: invalid}\n"
	if got := analyzer.Diagnostics(pathToURI(entry), "imports: [config/runtime.yaml]\n", map[string]string{pathToURI(fragment): bad}); len(got) == 0 {
		t.Fatal("unsaved import was ignored")
	}
	if got := analyzer.Diagnostics(pathToURI(fragment), bad, nil); len(got) == 0 {
		t.Fatal("fragment was not validated through its entry")
	}
	if got := analyzer.Diagnostics(pathToURI(fragment), "hyperbricks: {mode: live}\n", nil); len(got) != 0 {
		t.Fatalf("valid package fragment: %#v", got)
	}
}
