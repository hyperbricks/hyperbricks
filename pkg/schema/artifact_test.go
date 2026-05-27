package schema

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGeneratedSchemaArtifactIsCurrent(t *testing.T) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(ExtractRegistry(Definitions())); err != nil {
		t.Fatalf("encode schema: %v", err)
	}

	artifactPath := filepath.Join(hyperbricksRepoRoot(t), "assets", "schema", "components.schema.json")
	actual, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read generated schema artifact: %v", err)
	}
	if !bytes.Equal(actual, buf.Bytes()) {
		t.Fatalf("generated schema artifact is out of date; run `go run ./cmd/hyperbricks-schema -out assets/schema/components.schema.json`")
	}
}

func hyperbricksRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
