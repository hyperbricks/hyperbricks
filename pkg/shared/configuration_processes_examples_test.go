package shared

import (
	"os"
	"path/filepath"
	"testing"
)

// Public example packages must stay on the same validation path as author and
// doctor. This check does not execute their declared commands.
func TestDevelopmentHooksExampleModules(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"development-hooks-demo", "development-hooks-plugin-demo", "catalog-store"} {
		t.Run(name, func(t *testing.T) {
			module := filepath.Join(root, "modules", name)
			data, err := os.ReadFile(filepath.Join(module, PackageConfigFileName))
			if err != nil {
				t.Fatal(err)
			}
			config, err := ValidatePackageConfigBytes(data, module)
			if err != nil {
				t.Fatal(err)
			}
			if !config.HasDevelopmentProcesses() {
				t.Fatal("example declares no lifecycle commands")
			}
		})
	}
}
