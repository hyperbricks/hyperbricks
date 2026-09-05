package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveDirectStartModuleRoot(t *testing.T) {
	workingDirectory := t.TempDir()
	absoluteModule := filepath.Join(t.TempDir(), "demo")

	tests := []struct {
		name   string
		module string
		want   string
	}{
		{name: "bare name", module: "demo", want: filepath.Join("modules", "demo")},
		{name: "relative modules path", module: filepath.Join("modules", "demo"), want: filepath.Join(workingDirectory, "modules", "demo")},
		{name: "explicit relative modules path", module: "." + string(filepath.Separator) + filepath.Join("modules", "demo"), want: filepath.Join(workingDirectory, "modules", "demo")},
		{name: "parent relative path", module: filepath.Join("..", "other", "modules", "demo"), want: filepath.Join(filepath.Dir(workingDirectory), "other", "modules", "demo")},
		{name: "absolute path", module: absoluteModule, want: absoluteModule},
		{name: "current directory", module: ".", want: workingDirectory},
		{name: "trailing separator", module: "demo" + string(filepath.Separator), want: filepath.Join(workingDirectory, "demo")},
		{name: "path containing spaces", module: "." + string(filepath.Separator) + filepath.Join("modules", "my module"), want: filepath.Join(workingDirectory, "modules", "my module")},
		{name: "bare name containing spaces", module: "my module", want: filepath.Join("modules", "my module")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveDirectStartModuleRoot(tt.module, workingDirectory)
			if err != nil {
				t.Fatalf("resolve direct start module root: %v", err)
			}
			if got != tt.want {
				t.Fatalf("resolved root = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveDirectStartModuleRootRejectsEmptySelection(t *testing.T) {
	for _, module := range []string{"", "   "} {
		if _, err := resolveDirectStartModuleRoot(module, t.TempDir()); err == nil {
			t.Fatalf("expected %q to be rejected", module)
		}
	}
}

func TestResolveDirectStartModuleRootClassifiesBeforeCleaning(t *testing.T) {
	workingDirectory := t.TempDir()
	got, err := resolveDirectStartModuleRoot("."+string(filepath.Separator)+"demo", workingDirectory)
	if err != nil {
		t.Fatalf("resolve explicit relative path: %v", err)
	}
	if want := filepath.Join(workingDirectory, "demo"); got != want {
		t.Fatalf("resolved root = %q, want %q", got, want)
	}
}

func TestResolveModuleConfigPath(t *testing.T) {
	got, err := resolveModuleConfigPath(filepath.Join("modules", "demo"), "profiles/raw.hyperbricks.yaml")
	if err != nil {
		t.Fatalf("resolve module config path: %v", err)
	}
	want := filepath.Join("modules", "demo", "profiles", "raw.hyperbricks.yaml")
	if got != want {
		t.Fatalf("resolved path = %q, want %q", got, want)
	}
}

func TestResolveModuleConfigPathRejectsPathsOutsideModule(t *testing.T) {
	for _, configPath := range []string{"../outside.yaml", filepath.Join(string(filepath.Separator), "tmp", "outside.yaml")} {
		if _, err := resolveModuleConfigPath(filepath.Join("modules", "demo"), configPath); err == nil {
			t.Fatalf("expected %q to be rejected", configPath)
		}
	}
}

func TestCompleteStartModuleSuggestsNamesAndKeepsPathCompletion(t *testing.T) {
	workingDirectory := t.TempDir()
	modulesDirectory := filepath.Join(workingDirectory, "modules")
	if err := os.MkdirAll(filepath.Join(modulesDirectory, "demo"), 0755); err != nil {
		t.Fatalf("create module fixture: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(modulesDirectory, "different"), 0755); err != nil {
		t.Fatalf("create module fixture: %v", err)
	}

	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(workingDirectory); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDirectory); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	completions, directive := completeStartModule(nil, nil, "de")
	if directive != 0 {
		t.Fatalf("completion directive = %d, want default", directive)
	}
	if len(completions) != 1 || !strings.HasPrefix(completions[0], "demo\t") {
		t.Fatalf("completions = %#v, want demo module", completions)
	}

	completions, directive = completeStartModule(nil, nil, "."+string(filepath.Separator)+"modules"+string(filepath.Separator))
	if len(completions) != 0 || directive != 0 {
		t.Fatalf("path completion = (%#v, %d), want shell default", completions, directive)
	}
}
