package commands

import "testing"

func TestStaticServeAlsoEnablesRendering(t *testing.T) {
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
