package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestConfirmDeletionNonInteractiveDoesNotPrompt(t *testing.T) {
	for _, tc := range []struct {
		name           string
		nonInteractive bool
		noKeyboard     string
	}{
		{name: "flag", nonInteractive: true},
		{name: "environment", noKeyboard: "1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previousMode, previousStderr := commands.NonInteractive, os.Stderr
			commands.NonInteractive = tc.nonInteractive
			t.Setenv("HB_NO_KEYBOARD", tc.noKeyboard)
			capture, err := os.Create(filepath.Join(t.TempDir(), "stderr"))
			if err != nil {
				t.Fatal(err)
			}
			os.Stderr = capture
			t.Cleanup(func() {
				commands.NonInteractive, os.Stderr = previousMode, previousStderr
				_ = capture.Close()
			})
			if confirmDeletion("rendered") {
				t.Fatal("non-interactive mode approved deletion without confirmation")
			}
			output, err := os.ReadFile(capture.Name())
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(output), "(y/n)") || !strings.Contains(string(output), "Non-interactive mode: deletion canceled.") {
				t.Fatalf("expected cancellation without a keyboard prompt, got %q", output)
			}
		})
	}
}

func TestResolveDevelopmentWatchDirectoriesDefaultsToHyperbricksAndTemplates(t *testing.T) {
	config := &shared.Config{
		Directories: map[string]string{
			"hyperbricks": "modules/demo/hyperbricks",
			"templates":   "modules/demo/templates",
			"resources":   "modules/demo/resources",
		},
	}

	got := resolveDevelopmentWatchDirectories(config)
	want := []string{"modules/demo/hyperbricks", "modules/demo/templates"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("watch directories = %#v, want %#v", got, want)
	}
}

func TestResolveDevelopmentWatchDirectoriesUsesConfiguredWatchDirs(t *testing.T) {
	config := &shared.Config{
		Development: shared.DevelopmentConfig{
			WatchDirs: []string{"hyperbricks", "templates", "resources", "templates", " "},
		},
		Directories: map[string]string{
			"hyperbricks": "modules/demo/hyperbricks",
			"templates":   "modules/demo/templates",
			"resources":   "modules/demo/resources",
		},
	}

	got := resolveDevelopmentWatchDirectories(config)
	want := []string{"modules/demo/hyperbricks", "modules/demo/templates", "modules/demo/resources"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("watch directories = %#v, want %#v", got, want)
	}
}

func TestResolveDevelopmentWatchDirectoriesAllowsRelativeCustomPaths(t *testing.T) {
	config := &shared.Config{
		Development: shared.DevelopmentConfig{
			WatchDirs: []string{"custom"},
		},
		Directories: map[string]string{},
	}

	got := resolveDevelopmentWatchDirectories(config)
	want := []string{"custom"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("watch directories = %#v, want %#v", got, want)
	}
}
