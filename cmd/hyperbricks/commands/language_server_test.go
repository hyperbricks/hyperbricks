package commands

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/language"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestLanguageServerCommandIdentityAndHelp(t *testing.T) {
	cmd := NewLanguageServerCommand()
	if cmd.Name() != "language-server" || cmd.Short == "" {
		t.Fatalf("command identity = %q, %q", cmd.Name(), cmd.Short)
	}
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if help := output.String(); !strings.Contains(help, "--stdio") || !strings.Contains(help, "Language Server Protocol") {
		t.Fatalf("language-server help = %q", help)
	}
}

func TestLanguageServerCommandUsesStdioAndStopsCLIInitialization(t *testing.T) {
	oldExit, oldExitCode := Exit, ExitCode
	t.Cleanup(func() { Exit, ExitCode = oldExit, oldExitCode })
	Exit, ExitCode = false, 99
	cmd := NewLanguageServerCommand()
	cmd.SetIn(bytes.NewReader(nil))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--stdio"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !Exit || ExitCode != 0 {
		t.Fatalf("Exit=%t ExitCode=%d", Exit, ExitCode)
	}
}

func TestLanguageServerCommandRejectsNonStdioTransport(t *testing.T) {
	oldExit, oldExitCode := Exit, ExitCode
	t.Cleanup(func() { Exit, ExitCode = oldExit, oldExitCode })
	cmd := NewLanguageServerCommand()
	cmd.SetArgs([]string{"--stdio=false"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "requires --stdio") {
		t.Fatalf("non-stdio error = %v", err)
	}
	if !Exit || ExitCode != 1 {
		t.Fatalf("Exit=%t ExitCode=%d", Exit, ExitCode)
	}
}

func TestResolveLanguageRuntimeConnectionUsesSelectedStrictProfileAndLocalCredentials(t *testing.T) {
	root := t.TempDir()
	moduleRoot := filepath.Join(root, "modules", "demo")
	profile := filepath.Join(moduleRoot, "profiles", "editor.hyperbricks.yaml")
	if err := os.MkdirAll(filepath.Dir(profile), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HB_LANGUAGE_SERVER_USER", "editor-developer")
	t.Setenv("HB_LANGUAGE_SERVER_PASSWORD", "resolved secret")
	writeLanguageServerConfig(t, profile, `
hyperbricks:
  mode: development
  server:
    port: 9123
  development:
    dashboard:
      enabled: true
      credentials:
        user:
          env: HB_LANGUAGE_SERVER_USER
        password:
          env: HB_LANGUAGE_SERVER_PASSWORD
`)
	preserveRuntimeOptionsForLanguageServerTest(t)

	connection, err := resolveLanguageRuntimeConnection(context.Background(), language.RuntimeConnectionRequest{
		WorkspaceRoot: root,
		Module:        "demo",
		Config:        "profiles/editor.hyperbricks.yaml",
	})
	if err != nil {
		t.Fatal(err)
	}
	if connection.ModuleRoot != moduleRoot || connection.ConfigPath != profile || connection.RuntimeURL != "http://localhost:9123" {
		t.Fatalf("connection = %#v", connection)
	}
	if connection.ErrorsURL != "http://localhost:9123/__hyperbricks/errors" {
		t.Fatalf("errors URL = %q", connection.ErrorsURL)
	}
	if connection.Auth.Mode != language.RuntimeCredentialsAutomatic || connection.Auth.Username != "editor-developer" || connection.Auth.Password != "resolved secret" {
		t.Fatalf("resolved local auth = %#v", connection.Auth)
	}
}

func TestResolveLanguageRuntimeConnectionNeverForwardsLocalCredentialsToExplicitRemote(t *testing.T) {
	root := t.TempDir()
	moduleRoot := filepath.Join(root, "modules", "demo")
	profile := filepath.Join(moduleRoot, PackageConfigFileName)
	if err := os.MkdirAll(moduleRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeLanguageServerConfig(t, profile, `
hyperbricks:
  mode: development
  development:
    dashboard:
      enabled: true
      credentials:
        user: local-developer
        password: local-secret
`)
	preserveRuntimeOptionsForLanguageServerTest(t)

	connection, err := resolveLanguageRuntimeConnection(context.Background(), language.RuntimeConnectionRequest{
		WorkspaceRoot: root,
		Module:        "demo",
		Config:        PackageConfigFileName,
		RuntimeURL:    "https://runtime.example.test:8443/app?token=must-not-echo#fragment",
	})
	if err != nil {
		t.Fatal(err)
	}
	if connection.Auth.Mode != language.RuntimeCredentialsNone || connection.Auth.Username != "" || connection.Auth.Password != "" {
		t.Fatalf("local credentials forwarded to remote runtime: %#v", connection.Auth)
	}
	if connection.RuntimeURL != "https://runtime.example.test:8443" {
		t.Fatalf("runtime URL = %q", connection.RuntimeURL)
	}
	if connection.ErrorsURL != "https://runtime.example.test:8443/__hyperbricks/errors" {
		t.Fatalf("errors URL = %q", connection.ErrorsURL)
	}
}

func TestResolveLanguageRuntimeConnectionPreservesSymlinkedEditorPaths(t *testing.T) {
	root := t.TempDir()
	realModuleRoot := filepath.Join(root, "real-module")
	linkedModuleRoot := filepath.Join(root, "linked-module")
	realConfigPath := filepath.Join(realModuleRoot, PackageConfigFileName)
	if err := os.MkdirAll(filepath.Join(realModuleRoot, "hyperbricks"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeLanguageServerConfig(t, realConfigPath, "hyperbricks:\n  mode: development\n")
	if err := os.WriteFile(filepath.Join(realModuleRoot, "hyperbricks", "page.hyperbricks.yaml"), []byte("page: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realModuleRoot, linkedModuleRoot); err != nil {
		t.Fatal(err)
	}
	preserveRuntimeOptionsForLanguageServerTest(t)

	connection, err := resolveLanguageRuntimeConnection(context.Background(), language.RuntimeConnectionRequest{
		WorkspaceRoot: root,
		Module:        "." + string(filepath.Separator) + "linked-module",
		Config:        PackageConfigFileName,
	})
	if err != nil {
		t.Fatal(err)
	}
	linkedConfigPath := filepath.Join(linkedModuleRoot, PackageConfigFileName)
	if connection.ModuleRoot != linkedModuleRoot || connection.ConfigPath != linkedConfigPath {
		t.Fatalf("editor-visible connection paths = %#v", connection)
	}

	state := language.NewRuntimeDiagnosticsState(connection.ModuleRoot, connection.ConfigPath)
	update := state.Replace(language.RuntimeDiagnosticsSnapshot{
		Generation: 5,
		Records: []language.RuntimeDiagnosticsRecord{{
			Generation: 5,
			Errors: []language.RuntimeDiagnosticsIssue{
				{Hash: "source", File: "hyperbricks/page.hyperbricks.yaml", Err: "source failure"},
				{Hash: "config", File: "__config", Err: "config failure"},
			},
		}},
	})
	linkedSourcePath := filepath.Join(linkedModuleRoot, "hyperbricks", "page.hyperbricks.yaml")
	if len(update.Diagnostics[linkedSourcePath]) != 1 || len(update.Diagnostics[linkedConfigPath]) != 1 || len(update.Unmapped) != 0 {
		t.Fatalf("editor-visible runtime paths = %#v", update)
	}
	if _, resolved := update.Diagnostics[filepath.Join(realModuleRoot, "hyperbricks", "page.hyperbricks.yaml")]; resolved {
		t.Fatalf("runtime diagnostics bypassed editor URI identity: %#v", update.Diagnostics)
	}
}

func TestResolveLanguageRuntimeConnectionMarksLiveProfileUnavailable(t *testing.T) {
	root := t.TempDir()
	moduleRoot := filepath.Join(root, "modules", "demo")
	if err := os.MkdirAll(moduleRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeLanguageServerConfig(t, filepath.Join(moduleRoot, PackageConfigFileName), `
hyperbricks:
  mode: live
`)
	preserveRuntimeOptionsForLanguageServerTest(t)
	_, err := resolveLanguageRuntimeConnection(context.Background(), language.RuntimeConnectionRequest{
		WorkspaceRoot: root,
		Module:        "demo",
		Config:        PackageConfigFileName,
	})
	if !errors.Is(err, language.ErrRuntimeDiagnosticsUnavailable) {
		t.Fatalf("live profile error = %v", err)
	}
}

func TestResolveLanguageRuntimeConnectionRejectsConfigSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	moduleRoot := filepath.Join(root, "modules", "demo")
	if err := os.MkdirAll(moduleRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(root, "outside.hyperbricks.yaml")
	writeLanguageServerConfig(t, external, "hyperbricks:\n  mode: development\n")
	if err := os.Symlink(external, filepath.Join(moduleRoot, PackageConfigFileName)); err != nil {
		t.Fatal(err)
	}
	preserveRuntimeOptionsForLanguageServerTest(t)
	_, err := resolveLanguageRuntimeConnection(context.Background(), language.RuntimeConnectionRequest{
		WorkspaceRoot: root,
		Module:        "demo",
		Config:        PackageConfigFileName,
	})
	if err == nil || !strings.Contains(err.Error(), "must stay inside") {
		t.Fatalf("symlink escape error = %v", err)
	}
}

func TestResolveLanguageRuntimeConnectionOmitsDisabledDashboardErrorsURL(t *testing.T) {
	root := t.TempDir()
	moduleRoot := filepath.Join(root, "modules", "demo")
	profile := filepath.Join(moduleRoot, PackageConfigFileName)
	if err := os.MkdirAll(moduleRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeLanguageServerConfig(t, profile, `
hyperbricks:
  mode: development
  server:
    port: 8123
  development:
    dashboard:
      enabled: false
`)
	preserveRuntimeOptionsForLanguageServerTest(t)

	connection, err := resolveLanguageRuntimeConnection(context.Background(), language.RuntimeConnectionRequest{
		WorkspaceRoot: root,
		Module:        "demo",
		Config:        PackageConfigFileName,
	})
	if err != nil {
		t.Fatal(err)
	}
	if connection.RuntimeURL != "http://localhost:8123" || connection.ErrorsURL != "" {
		t.Fatalf("disabled dashboard connection = %#v", connection)
	}
}

func writeLanguageServerConfig(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func preserveRuntimeOptionsForLanguageServerTest(t *testing.T) {
	t.Helper()
	previous := shared.GetRuntimeOptions()
	shared.SetRuntimeOptions(shared.RuntimeOptions{})
	t.Cleanup(func() { shared.SetRuntimeOptions(previous) })
}
