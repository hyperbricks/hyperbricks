package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalDevMetadataUsesSelectedWorktreeCommit(t *testing.T) {
	root := t.TempDir()
	modulesDir := filepath.Join(root, "modules")
	moduleDir := filepath.Join(modulesDir, "demo")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("create module directory: %v", err)
	}

	packageConfig := `hyperbricks:
  metadata:
    module: demo
    moduleversion: "1.2.3"
    commit: stale-source-value
  server:
    port: 8123
`
	if err := os.WriteFile(filepath.Join(moduleDir, "package.hyperbricks.yaml"), []byte(packageConfig), 0o644); err != nil {
		t.Fatalf("write package config: %v", err)
	}

	runLocalRuntimeGit(t, moduleDir, "init")
	runLocalRuntimeGit(t, moduleDir, "config", "user.email", "test@hyperbricks.dev")
	runLocalRuntimeGit(t, moduleDir, "config", "user.name", "HyperBricks Test")
	runLocalRuntimeGit(t, moduleDir, "add", "package.hyperbricks.yaml")
	runLocalRuntimeGit(t, moduleDir, "commit", "-m", "fixture")
	expectedCommit := runLocalRuntimeGit(t, moduleDir, "rev-parse", "--short=7", "HEAD")

	api := &deployLocalServer{
		modulesDir: modulesDir,
		buildRoot:  filepath.Join(root, "deploy"),
	}
	row, ok := api.devBuildRow("demo")
	if !ok {
		t.Fatal("development build row was not returned")
	}
	status, err := api.devBuildStatus("demo")
	if err != nil {
		t.Fatalf("read development build status: %v", err)
	}

	if row.Commit != expectedCommit {
		t.Fatalf("row commit = %q, want selected worktree commit %q", row.Commit, expectedCommit)
	}
	if row.Commit == "stale-source-value" {
		t.Fatal("development build row used stale source metadata.commit")
	}
	if got := status["commit"]; got != expectedCommit {
		t.Fatalf("status commit = %v, want selected worktree commit %q", got, expectedCommit)
	}
	if row.Commit != status["commit"] {
		t.Fatalf("row/status commits differ: row=%q status=%v", row.Commit, status["commit"])
	}
	if row.ModuleVersion != "1.2.3" || status["moduleversion"] != "1.2.3" {
		t.Fatalf("source module version was not preserved: row=%q status=%v", row.ModuleVersion, status["moduleversion"])
	}
	if status["port"] != 8123 {
		t.Fatalf("status port = %v, want 8123", status["port"])
	}
}

func TestLocalDevMetadataOmitsCommitOutsideGitWorktree(t *testing.T) {
	root := t.TempDir()
	modulesDir := filepath.Join(root, "modules")
	moduleDir := filepath.Join(modulesDir, "demo")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("create module directory: %v", err)
	}
	packageConfig := `hyperbricks:
  metadata:
    module: demo
    moduleversion: "1.0.0"
    commit: stale-source-value
`
	if err := os.WriteFile(filepath.Join(moduleDir, "package.hyperbricks.yaml"), []byte(packageConfig), 0o644); err != nil {
		t.Fatalf("write package config: %v", err)
	}

	api := &deployLocalServer{
		modulesDir: modulesDir,
		buildRoot:  filepath.Join(root, "deploy"),
	}
	row, ok := api.devBuildRow("demo")
	if !ok {
		t.Fatal("development build row was not returned")
	}
	status, err := api.devBuildStatus("demo")
	if err != nil {
		t.Fatalf("read development build status: %v", err)
	}
	if row.Commit != "" || status["commit"] != "" {
		t.Fatalf("non-Git commit should be empty: row=%q status=%v", row.Commit, status["commit"])
	}
}

func TestLocalBuildIndexPreservesArtifactHyperBricksVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "demo", "hyperbricks.versions.json")
	want := "v1.2.5-beta"
	index := localBuildIndex{
		Current: "build-1",
		Versions: []localBuildRow{{
			BuildID:     "build-1",
			HyperBricks: want,
		}},
	}
	if err := saveLocalBuildIndex(path, index); err != nil {
		t.Fatalf("save local build index: %v", err)
	}
	loaded, err := loadLocalBuildIndex(path)
	if err != nil {
		t.Fatalf("load local build index: %v", err)
	}
	row, ok := findLocalRow(loaded, "build-1")
	if !ok || row.HyperBricks != want {
		t.Fatalf("loaded build row = %#v, want HyperBricks %q", row, want)
	}
}

func runLocalRuntimeGit(t *testing.T, directory string, args ...string) string {
	t.Helper()
	commandArgs := append([]string{"-C", directory}, args...)
	output, err := exec.Command("git", commandArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}
