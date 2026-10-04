package shared

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// Keep copyable public examples on the same validation path as author/doctor.
// Validation must never execute the declared commands or check host tools.
func TestDevelopmentHooksDocumentationExamples(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	document, err := os.ReadFile(filepath.Join(root, "docs", "DEVELOPMENT_HOOKS.md"))
	if err != nil {
		t.Fatal(err)
	}
	examples := regexp.MustCompile("(?s)```yaml\\n(.*?)```").FindAllSubmatch(document, -1)
	if len(examples) == 0 {
		t.Fatal("guide contains no YAML examples")
	}
	for _, example := range examples {
		if _, err := ValidatePackageConfigBytes(example[1], root); err != nil {
			t.Fatalf("documentation example does not validate: %v", err)
		}
	}
}
