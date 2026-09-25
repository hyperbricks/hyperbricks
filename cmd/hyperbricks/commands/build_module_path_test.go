package commands

import (
	"archive/zip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/assets"
	"go.yaml.in/yaml/v4"
)

func TestBuildModulePathUsesDirectoryBaseAsDeploymentName(t *testing.T) {
	workingDirectory := t.TempDir()
	moduleDirectory := filepath.Join(workingDirectory, "modules", "demo")
	if err := os.MkdirAll(filepath.Join(moduleDirectory, "hyperbricks"), 0o755); err != nil {
		t.Fatalf("create module fixture: %v", err)
	}
	config := "hyperbricks:\n  metadata:\n    module: stale-name\n    moduleversion: \"1.0\"\n    hyperbricks: v0.0.0\n"
	if err := os.WriteFile(filepath.Join(moduleDirectory, PackageConfigFileName), []byte(config), 0o644); err != nil {
		t.Fatalf("write package config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDirectory, "hyperbricks", "page.hyperbricks.yaml"), []byte("page:\n  - type: hypermedia\n"), 0o644); err != nil {
		t.Fatalf("write module source: %v", err)
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

	outDirectory := filepath.Join(workingDirectory, "artifacts")
	result, err := BuildModuleWithOptions(BuildOptions{
		Module: "./modules/demo",
		OutDir: outDirectory,
		Force:  true,
		Format: "hra",
	})
	if err != nil {
		t.Fatalf("build module by path: %v", err)
	}
	if result.Module != "demo" {
		t.Fatalf("result module = %q, want demo", result.Module)
	}
	if got, want := filepath.Dir(result.ArchivePath), filepath.Join(outDirectory, "demo"); got != want {
		t.Fatalf("archive directory = %q, want %q", got, want)
	}
	if name := filepath.Base(result.ArchivePath); !strings.HasPrefix(name, "demo-1.0-") || !strings.HasSuffix(name, ".hra") {
		t.Fatalf("archive name = %q, want demo-1.0-<build-id>.hra", name)
	}
	if _, err := os.Stat(filepath.Join(outDirectory, "demo", versionIndexFile)); err != nil {
		t.Fatalf("read module build index: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDirectory, "modules")); !os.IsNotExist(err) {
		t.Fatalf("path selection created an unexpected modules output directory: %v", err)
	}
	if sourceConfig, err := os.ReadFile(filepath.Join(moduleDirectory, PackageConfigFileName)); err != nil || string(sourceConfig) != config {
		t.Fatalf("build changed source package config: %q, %v", sourceConfig, err)
	}

	archive, err := zip.OpenReader(result.ArchivePath)
	if err != nil {
		t.Fatalf("open HRA archive: %v", err)
	}
	defer archive.Close()

	var archivedConfig []byte
	for _, file := range archive.File {
		if file.Name != PackageConfigFileName {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatalf("open archived package config: %v", err)
		}
		archivedConfig, err = io.ReadAll(reader)
		closeErr := reader.Close()
		if err != nil {
			t.Fatalf("read archived package config: %v", err)
		}
		if closeErr != nil {
			t.Fatalf("close archived package config: %v", closeErr)
		}
		break
	}
	if len(archivedConfig) == 0 {
		t.Fatal("archive does not contain package.hyperbricks.yaml")
	}
	var decoded struct {
		HyperBricks struct {
			Metadata map[string]string `yaml:"metadata"`
		} `yaml:"hyperbricks"`
	}
	if err := yaml.Unmarshal(archivedConfig, &decoded); err != nil {
		t.Fatalf("decode archived package config: %v", err)
	}
	if got := decoded.HyperBricks.Metadata["module"]; got != "demo" {
		t.Fatalf("archived metadata.module = %q, want demo", got)
	}
	if got := decoded.HyperBricks.Metadata["moduleversion"]; got != "1.0" {
		t.Fatalf("archived metadata.moduleversion = %q, want 1.0", got)
	}
	if got := decoded.HyperBricks.Metadata["format"]; got != "hra" {
		t.Fatalf("archived metadata.format = %q, want hra", got)
	}
	if got := decoded.HyperBricks.Metadata["format_version"]; got != "1" {
		t.Fatalf("archived metadata.format_version = %q, want 1", got)
	}
	if got := decoded.HyperBricks.Metadata["commit"]; got != "unknown" {
		t.Fatalf("archived metadata.commit = %q, want unknown outside a Git worktree", got)
	}
	if _, err := time.Parse(time.RFC3339, decoded.HyperBricks.Metadata["built_at"]); err != nil {
		t.Fatalf("archived metadata.built_at = %q: %v", decoded.HyperBricks.Metadata["built_at"], err)
	}
	if !strings.HasSuffix(decoded.HyperBricks.Metadata["built_at"], "Z") {
		t.Fatalf("archived metadata.built_at = %q, want UTC timestamp", decoded.HyperBricks.Metadata["built_at"])
	}
	if got := decoded.HyperBricks.Metadata["hyperbricks"]; got != strings.TrimSpace(assets.VersionMD) {
		t.Fatalf("archived metadata.hyperbricks = %q, want %q", got, strings.TrimSpace(assets.VersionMD))
	}
	if _, present := decoded.HyperBricks.Metadata["source_hash"]; present {
		t.Fatal("archived metadata unexpectedly contains build-index-only source_hash")
	}

	index, err := loadBuildIndex(filepath.Join(outDirectory, "demo", versionIndexFile))
	if err != nil {
		t.Fatalf("load build index: %v", err)
	}
	current, ok := findBuildIndex(index, index.Current)
	if !ok || current.HyperBricks != strings.TrimSpace(assets.VersionMD) {
		t.Fatalf("current build index row = %#v, want current HyperBricks version", current)
	}

	zipResult, err := BuildModuleWithOptions(BuildOptions{
		Module: "./modules/demo",
		OutDir: outDirectory,
		Format: "zip",
	})
	if err != nil {
		t.Fatalf("build ZIP after unchanged HRA source: %v", err)
	}
	if !zipResult.Built || !strings.HasSuffix(zipResult.ArchivePath, ".zip") {
		t.Fatalf("ZIP request reused the HRA build: %#v", zipResult)
	}
	zipArchive, err := zip.OpenReader(zipResult.ArchivePath)
	if err != nil {
		t.Fatalf("open ZIP archive: %v", err)
	}
	defer zipArchive.Close()
	zipMetadata := archiveMetadata(t, zipArchive.File)
	if got := zipMetadata["format"]; got != "zip" {
		t.Fatalf("ZIP metadata.format = %q, want zip", got)
	}
	if got := zipMetadata["hyperbricks"]; got != strings.TrimSpace(assets.VersionMD) {
		t.Fatalf("ZIP metadata.hyperbricks = %q, want current runtime", got)
	}
	if sourceConfig, err := os.ReadFile(filepath.Join(moduleDirectory, PackageConfigFileName)); err != nil || string(sourceConfig) != config {
		t.Fatalf("HRA/ZIP builds changed source package config: %q, %v", sourceConfig, err)
	}
}

func archiveMetadata(t *testing.T, files []*zip.File) map[string]string {
	t.Helper()
	for _, file := range files {
		if file.Name != PackageConfigFileName {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatalf("open archived package config: %v", err)
		}
		content, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil {
			t.Fatalf("read archived package config: %v", readErr)
		}
		if closeErr != nil {
			t.Fatalf("close archived package config: %v", closeErr)
		}
		var decoded struct {
			HyperBricks struct {
				Metadata map[string]string `yaml:"metadata"`
			} `yaml:"hyperbricks"`
		}
		if err := yaml.Unmarshal(content, &decoded); err != nil {
			t.Fatalf("decode archived package config: %v", err)
		}
		return decoded.HyperBricks.Metadata
	}
	t.Fatal("archive does not contain package.hyperbricks.yaml")
	return nil
}

func TestBuildUsesSelectedModuleWorktreeCommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	repository := t.TempDir()
	moduleDirectory := filepath.Join(repository, "modules", "demo")
	if err := os.MkdirAll(filepath.Join(moduleDirectory, "hyperbricks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moduleDirectory, PackageConfigFileName), []byte("hyperbricks:\n  metadata:\n    moduleversion: \"1.0.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moduleDirectory, "hyperbricks", "page.hyperbricks.yaml"), []byte("page:\n  - type: hypermedia\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runBuildGit(t, repository, "init", "--quiet")
	runBuildGit(t, repository, "config", "user.email", "test@hyperbricks.dev")
	runBuildGit(t, repository, "config", "user.name", "HyperBricks Test")
	runBuildGit(t, repository, "add", ".")
	runBuildGit(t, repository, "commit", "--quiet", "-m", "module fixture")
	wantCommit := runBuildGit(t, repository, "rev-parse", "--short=7", "HEAD")

	otherRepository := t.TempDir()
	runBuildGit(t, otherRepository, "init", "--quiet")
	runBuildGit(t, otherRepository, "config", "user.email", "test@hyperbricks.dev")
	runBuildGit(t, otherRepository, "config", "user.name", "HyperBricks Test")
	if err := os.WriteFile(filepath.Join(otherRepository, "other.txt"), []byte("different repository\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runBuildGit(t, otherRepository, "add", ".")
	runBuildGit(t, otherRepository, "commit", "--quiet", "-m", "other fixture")
	t.Chdir(otherRepository)

	result, err := BuildModuleWithOptions(BuildOptions{
		Module: moduleDirectory,
		OutDir: t.TempDir(),
		Force:  true,
		Format: "hra",
	})
	if err != nil {
		t.Fatalf("build selected module worktree: %v", err)
	}
	archive, err := zip.OpenReader(result.ArchivePath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer archive.Close()
	if got := archiveMetadata(t, archive.File)["commit"]; got != wantCommit {
		t.Fatalf("archive commit = %q, want selected module worktree commit %q", got, wantCommit)
	}
}

func runBuildGit(t *testing.T, directory string, args ...string) string {
	t.Helper()
	commandArgs := append([]string{"-C", directory}, args...)
	output, err := exec.Command("git", commandArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}
