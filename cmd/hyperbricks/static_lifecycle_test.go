package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestStaticLifecycleOrderingAndFinish(t *testing.T) {
	if !developmentProcessesSupported() {
		t.Skip("processes unavailable")
	}
	for _, failure := range []string{"", "before_static", "initialize", "render", "asset_cleanup", "copy_assets", "package", "after_static", "serve"} {
		t.Run(failure, func(t *testing.T) {
			root := t.TempDir()
			old := commands.ModuleRoot
			commands.ModuleRoot = root
			t.Cleanup(func() { commands.ModuleRoot = old })
			marker := filepath.Join(root, "prepared")
			task := func(name, script string) shared.DevelopmentTaskConfig {
				return shared.DevelopmentTaskConfig{Name: name, Command: []string{"sh", "-c", script}, Timeout: time.Second}
			}
			config := &shared.Config{Hooks: shared.LifecycleHooksConfig{
				BeforeStatic: []shared.DevelopmentTaskConfig{task("prepare", "touch prepared")},
				AfterStatic:  []shared.DevelopmentTaskConfig{task("publish", "test -f \"$HB_EXPORT_ZIP\"; printf published > published")},
				Finish:       []shared.DevelopmentTaskConfig{task("finish", "printf '%s|%s|%s|%s|%s' \"$HB_OPERATION\" \"$HB_OUTCOME\" \"$HB_FAILED_PHASE\" \"$HB_EXIT_CODE\" \"$HB_SERVER_PORT\" > outcome")},
			}}
			if failure == "before_static" {
				config.Hooks.BeforeStatic[0] = task("prepare", "exit 7")
			}
			if failure == "after_static" {
				config.Hooks.AfterStatic[0] = task("publish", "exit 7")
			}
			zip := filepath.Join(root, "output.zip")
			served := false
			err := runStaticLifecycle(context.Background(), config, staticExportPlan{RenderDir: root}, true, func() error {
				if _, err := os.Stat(marker); err != nil {
					t.Fatal("preparation did not finish first")
				}
				if failure == "initialize" {
					return errors.New("init failed")
				}
				return nil
			}, func() (staticExportResult, error) {
				if failure == "render" || failure == "asset_cleanup" || failure == "copy_assets" || failure == "package" {
					return staticExportResult{RenderDir: root}, phaseError(failure, errors.New("export failed"))
				}
				if err := os.WriteFile(zip, []byte("zip"), 0600); err != nil {
					t.Fatal(err)
				}
				return staticExportResult{RenderDir: root, ZipPath: zip}, nil
			}, func() error {
				served = true
				if failure == "serve" {
					return errors.New("serve failed")
				}
				return nil
			})
			if (err != nil) != (failure != "") {
				t.Fatalf("failure %s result %v", failure, err)
			}
			raw, readErr := os.ReadFile(filepath.Join(root, "outcome"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			expected := "static|success||0|"
			if failure != "" {
				expected = "static|failure|" + failure + "|1|"
			}
			if string(raw) != expected {
				t.Fatalf("outcome %q want %q", raw, expected)
			}
			_, publishErr := os.Stat(filepath.Join(root, "published"))
			if (publishErr == nil) != (failure == "" || failure == "serve") {
				t.Fatalf("unexpected publishing outcome for %s: %v", failure, publishErr)
			}
			if served != (failure == "" || failure == "serve") {
				t.Fatalf("serve=%v failure=%s", served, failure)
			}
		})
	}
}

func TestStaticLifecycleDisabledAndCancelled(t *testing.T) {
	root := t.TempDir()
	old := commands.ModuleRoot
	commands.ModuleRoot = root
	t.Cleanup(func() { commands.ModuleRoot = old })
	task := shared.DevelopmentTaskConfig{Name: "never", Command: []string{"does-not-exist"}, Timeout: time.Second}
	config := &shared.Config{Hooks: shared.LifecycleHooksConfig{BeforeStatic: []shared.DevelopmentTaskConfig{task}, AfterStatic: []shared.DevelopmentTaskConfig{task}, Finish: []shared.DevelopmentTaskConfig{task}}}
	if err := runStaticLifecycle(context.Background(), config, staticExportPlan{}, false, func() error { return nil }, func() (staticExportResult, error) { return staticExportResult{}, nil }, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if !developmentProcessesSupported() {
		return
	}
	config.Hooks.BeforeStatic = nil
	config.Hooks.AfterStatic = nil
	config.Hooks.Finish = []shared.DevelopmentTaskConfig{{Name: "finish", Command: []string{"sh", "-c", "printf '%s:%s' \"$HB_OUTCOME\" \"$HB_EXIT_CODE\" > outcome"}, Timeout: time.Second}}
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(errRuntimeInterrupt)
	err := runStaticLifecycle(ctx, config, staticExportPlan{}, true, func() error { t.Fatal("initialized after cancellation"); return nil }, nil, nil)
	if !errors.Is(err, errRuntimeInterrupt) {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "outcome"))
	if err != nil || strings.TrimSpace(string(raw)) != "cancelled:130" {
		t.Fatalf("fresh finish context: %s %v", raw, err)
	}
}

func TestFinishFailurePreservesOriginalAndStopsLaterTasks(t *testing.T) {
	if !developmentProcessesSupported() {
		t.Skip("processes unavailable")
	}
	root := t.TempDir()
	old := commands.ModuleRoot
	commands.ModuleRoot = root
	t.Cleanup(func() { commands.ModuleRoot = old })
	config := &shared.Config{Hooks: shared.LifecycleHooksConfig{Finish: []shared.DevelopmentTaskConfig{
		{Name: "fail", Command: []string{"sh", "-c", "exit 9"}, Timeout: time.Second},
		{Name: "never", Command: []string{"sh", "-c", "touch unexpected"}, Timeout: time.Second},
	}}}
	original := errors.New("publication failed")
	result := finishOperation(config, true, "static", "after_static", staticExportResult{}, original)
	if !errors.Is(result, original) || !strings.Contains(result.Error(), "finish") {
		t.Fatal(result)
	}
	if _, err := os.Stat(filepath.Join(root, "unexpected")); !os.IsNotExist(err) {
		t.Fatal("later finish task ran")
	}
	if err := finishOperation(config, true, "static", "after_static", staticExportResult{}, nil); err == nil {
		t.Fatal("finish failure reported success")
	}
}
func TestStaticDeclineRunsNoPreparation(t *testing.T) {
	root := t.TempDir()
	previousRoot := commands.ModuleRoot
	commands.ModuleRoot = root
	t.Cleanup(func() { commands.ModuleRoot = previousRoot })
	render := filepath.Join(root, "rendered")
	static := filepath.Join(root, "static")
	if err := os.MkdirAll(render, 0755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(render, "keep")
	if err := os.WriteFile(sentinel, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	force, wizard, noninteractive, enabled := commands.ForceStatic, commands.StaticWizard, commands.NonInteractive, commands.StaticWithProcesses
	commands.ForceStatic = false
	commands.StaticWizard = false
	commands.NonInteractive = true
	commands.StaticWithProcesses = true
	t.Cleanup(func() {
		commands.ForceStatic = force
		commands.StaticWizard = wizard
		commands.NonInteractive = noninteractive
		commands.StaticWithProcesses = enabled
	})
	config := &shared.Config{Directories: map[string]string{"render": render, "static": static}, Hooks: shared.LifecycleHooksConfig{BeforeStatic: []shared.DevelopmentTaskConfig{{Name: "never", Command: []string{"does-not-exist"}}}}}
	if err := runStaticOperation(config); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatal("decline removed output")
	}
	if _, err := os.Stat(static); !os.IsNotExist(err) {
		t.Fatal("decline initialized output")
	}
}
