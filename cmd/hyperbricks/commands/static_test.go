package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStaticServeAlsoEnablesRendering(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(filepath.Join("modules", "demo"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("modules", "demo", "package.hyperbricks.yaml"), []byte("hyperbricks: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	previousRenderStatic := RenderStatic
	previousServeStatic := ServeStatic
	previousStartModule := StartModule
	RenderStatic = false
	ServeStatic = false
	StartModule = ""
	t.Cleanup(func() {
		RenderStatic = previousRenderStatic
		ServeStatic = previousServeStatic
		StartModule = previousStartModule
	})

	command := NewMakeStaticCommand()
	command.SetArgs([]string{"--module", "demo", "--serve"})
	if err := command.Execute(); err != nil {
		t.Fatalf("execute static command: %v", err)
	}

	if !RenderStatic {
		t.Fatal("RenderStatic = false, want rendering enabled before serving")
	}
	if !ServeStatic {
		t.Fatal("ServeStatic = false, want static server enabled after rendering")
	}
	if StartModule != "demo" {
		t.Fatalf("StartModule = %q, want %q", StartModule, "demo")
	}
}
