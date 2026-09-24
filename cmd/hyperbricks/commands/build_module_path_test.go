package commands

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestBuildModulePathUsesDirectoryBaseAsDeploymentName(t *testing.T) {
	workingDirectory := t.TempDir()
	moduleDirectory := filepath.Join(workingDirectory, "modules", "demo")
	if err := os.MkdirAll(filepath.Join(moduleDirectory, "hyperbricks"), 0o755); err != nil {
		t.Fatalf("create module fixture: %v", err)
	}
	config := "hyperbricks:\n  metadata:\n    moduleversion: \"1.0\"\n"
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
}
