package catalog_test

import (
	"os/exec"
	"testing"
)

func TestCatalogImageManifestRefresh(t *testing.T) {
	command := exec.Command("python3", "test_image_manifest.py")
	command.Env = append(command.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("catalog image manifest: %v\n%s", err, output)
	}
}
