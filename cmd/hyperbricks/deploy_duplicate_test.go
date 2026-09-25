package main

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/assets"
	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
)

func TestLocalDuplicateBuildSnapshotsRuntimeWithoutActivation(t *testing.T) {
	root := t.TempDir()
	buildRoot := filepath.Join(root, "deploy")
	archivePath := filepath.Join(buildRoot, "demo", "demo-1.0.0-original.hra")
	config := strings.ReplaceAll(deployPackageFixture, "v1.2.5-beta", strings.TrimSpace(assets.VersionMD))
	originalArchive := writeDeployHRAFixture(t, archivePath, config)
	api := &deployLocalServer{buildRoot: buildRoot, modulesDir: filepath.Join(root, "modules")}
	index := localBuildIndex{Current: "original", Versions: []localBuildRow{{
		BuildID: "original", ModuleVersion: "1.0.0", Format: "hra", File: archivePath,
		Commit: "source7", HyperBricks: strings.TrimSpace(assets.VersionMD), RuntimeMode: "live", Production: true,
	}}}
	if err := saveLocalBuildIndex(api.indexPath("demo"), index); err != nil {
		t.Fatal(err)
	}
	location, err := api.localPackageConfig("demo", "original")
	if err != nil {
		t.Fatal(err)
	}
	currentConfig := strings.Replace(strings.Replace(config, "cache: 10m", "cache: 0s", 1), "moduleversion: \"1.0.0\"", "moduleversion: \"1.0.1\"", 1)
	if err := os.WriteFile(location.path, []byte(currentConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	spacePath := filepath.Join(location.moduleRoot, "hyperbricks", "spaces", "page", "nl.hyperbricks.yaml")
	if err := os.MkdirAll(filepath.Dir(spacePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(spacePath, []byte("title: Nieuw\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := "/local/modules/demo/builds/original/duplicate"
	first := httptest.NewRecorder()
	api.handleModuleRoutes(first, httptest.NewRequest(http.MethodPost, path, nil))
	if first.Code != http.StatusCreated {
		t.Fatalf("first duplicate status=%d body=%s", first.Code, first.Body.String())
	}
	firstID := decodeResponseMap(t, first)["build_id"].(string)
	second := httptest.NewRecorder()
	api.handleModuleRoutes(second, httptest.NewRequest(http.MethodPost, path, nil))
	if second.Code != http.StatusCreated {
		t.Fatalf("second duplicate status=%d body=%s", second.Code, second.Body.String())
	}
	secondID := decodeResponseMap(t, second)["build_id"].(string)
	if firstID == secondID || firstID == "original" {
		t.Fatalf("duplicate IDs first=%q second=%q", firstID, secondID)
	}
	updated, err := loadLocalBuildIndex(api.indexPath("demo"))
	if err != nil {
		t.Fatal(err)
	}
	if updated.Current != "original" || len(updated.Versions) != 3 {
		t.Fatalf("duplicate promoted/replaced source: %#v", updated)
	}
	row, ok := findLocalRow(updated, firstID)
	if !ok || row.OriginBuildID != "original" || row.RuntimeMode != "live" || !row.Production ||
		row.ModuleVersion != "1.0.1" || row.Commit != "source7" || row.SourceHash != "" {
		t.Fatalf("new local row = %#v", row)
	}
	entries := readHRAEntries(t, row.File)
	if !strings.Contains(entries["package.hyperbricks.yaml"], "cache: 0s") ||
		!strings.Contains(entries["package.hyperbricks.yaml"], "origin_build_id: original") ||
		!strings.Contains(entries["package.hyperbricks.yaml"], "commit: source7") ||
		entries["hyperbricks/spaces/page/nl.hyperbricks.yaml"] != "title: Nieuw\n" {
		t.Fatalf("duplicate did not capture current runtime: %#v", entries)
	}
	derivedRuntime, err := commands.EnsureRuntimeExtracted(row.File, buildRoot, "demo", firstID)
	if err != nil {
		t.Fatalf("new HRA is not extractable as a runtime: %v", err)
	}
	if derivedConfig, err := os.ReadFile(filepath.Join(derivedRuntime, "package.hyperbricks.yaml")); err != nil ||
		!strings.Contains(string(derivedConfig), "origin_build_id: original") {
		t.Fatalf("extracted duplicate lost its package metadata: %v", err)
	}
	if archived, err := os.ReadFile(archivePath); err != nil || !bytes.Equal(archived, originalArchive) {
		t.Fatal("duplication changed the original archive")
	}
	if saved, err := os.ReadFile(location.path); err != nil || string(saved) != currentConfig {
		t.Fatal("duplication changed the edited runtime YAML")
	}
	if saved, err := os.ReadFile(spacePath); err != nil || string(saved) != "title: Nieuw\n" {
		t.Fatal("duplication changed current Spaces content")
	}
	status := httptest.NewRecorder()
	api.handleModuleRoutes(status, httptest.NewRequest(http.MethodGet, "/local/modules/demo/builds/"+firstID+"/status", nil))
	if status.Code != http.StatusOK || decodeResponseMap(t, status)["origin_build_id"] != "original" {
		t.Fatalf("local duplicate status=%d body=%s", status.Code, status.Body.String())
	}
	if _, ok := api.readProcess("demo"); ok {
		t.Fatal("duplicating an inactive build started a process")
	}
}

func TestRemoteDuplicateBuildSnapshotsRuntimeWithoutActivation(t *testing.T) {
	root := t.TempDir()
	api := &deployAPI{root: root}
	archivePath := filepath.Join(root, "demo", "archives", "demo-1.0.0-original.hra")
	config := strings.ReplaceAll(deployPackageFixture, "v1.2.5-beta", strings.TrimSpace(assets.VersionMD))
	originalArchive := writeDeployHRAFixture(t, archivePath, config)
	index := deployIndex{Current: "original", Versions: []deployIndexRow{{
		BuildID: "original", ModuleVersion: "1.0.0", Format: "hra", File: archivePath,
		Commit: "source7", HyperBricks: strings.TrimSpace(assets.VersionMD), RuntimeMode: "development",
	}}}
	if err := saveDeployIndex(api.indexPath("demo"), index); err != nil {
		t.Fatal(err)
	}
	location, err := api.remotePackageConfig("demo", "original")
	if err != nil {
		t.Fatal(err)
	}
	currentConfig := strings.Replace(config, "cache: 10m", "cache: 0s", 1)
	if err := os.WriteFile(location.path, []byte(currentConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	api.handleDeploy(rec, httptest.NewRequest(http.MethodPost, "/deploy/modules/demo/builds/original/duplicate", nil))
	if rec.Code != http.StatusCreated {
		t.Fatalf("remote duplicate status=%d body=%s", rec.Code, rec.Body.String())
	}
	newID := decodeResponseMap(t, rec)["build_id"].(string)
	updated, err := loadDeployIndex(api.indexPath("demo"))
	if err != nil {
		t.Fatal(err)
	}
	if updated.Current != "original" || len(updated.Versions) != 2 {
		t.Fatalf("remote duplicate promoted/replaced source: %#v", updated)
	}
	row, ok := findDeployRow(updated, newID)
	if !ok || row.OriginBuildID != "original" || row.RuntimeMode != "development" || row.Production || row.SourceHash != "" {
		t.Fatalf("new remote row = %#v", row)
	}
	archive, err := resolveRemoteIndexedArchive(filepath.Join(root, "demo"), row.File, row.Format)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readHRAEntries(t, archive)["package.hyperbricks.yaml"], "cache: 0s") {
		t.Fatal("remote duplicate did not capture edited YAML")
	}
	if archived, err := os.ReadFile(archivePath); err != nil || !bytes.Equal(archived, originalArchive) {
		t.Fatal("remote duplication changed original archive")
	}
	if saved, err := os.ReadFile(location.path); err != nil || string(saved) != currentConfig {
		t.Fatal("remote duplication changed the edited runtime YAML")
	}
	status := httptest.NewRecorder()
	api.handleDeploy(status, httptest.NewRequest(http.MethodGet, "/deploy/modules/demo/builds/"+newID+"/status", nil))
	if status.Code != http.StatusOK || decodeResponseMap(t, status)["origin_build_id"] != "original" {
		t.Fatalf("remote duplicate status=%d body=%s", status.Code, status.Body.String())
	}
	if _, ok := api.readProcess("demo"); ok {
		t.Fatal("duplicating an inactive remote build started a process")
	}
}

func TestDuplicateRejectsVersionMismatchAndNonHRA(t *testing.T) {
	root := t.TempDir()
	api := &deployLocalServer{buildRoot: root}
	archivePath := filepath.Join(root, "demo", "demo-1.0.0-original.hra")
	config := strings.ReplaceAll(deployPackageFixture, "v1.2.5-beta", strings.TrimSpace(assets.VersionMD))
	writeDeployHRAFixture(t, archivePath, config)
	index := localBuildIndex{Current: "original", Versions: []localBuildRow{
		{BuildID: "original", Format: "hra", File: archivePath, HyperBricks: "old-host-version"},
		{BuildID: "zip", Format: "zip", File: archivePath},
	}}
	if err := saveLocalBuildIndex(api.indexPath("demo"), index); err != nil {
		t.Fatal(err)
	}
	mismatch := httptest.NewRecorder()
	api.handleBuildDuplicate(mismatch, httptest.NewRequest(http.MethodPost, "/local/modules/demo/builds/original/duplicate", nil), "demo", "original")
	if mismatch.Code != http.StatusConflict || !strings.Contains(mismatch.Body.String(), "old-host-version") {
		t.Fatalf("version mismatch status=%d body=%s", mismatch.Code, mismatch.Body.String())
	}
	zip := httptest.NewRecorder()
	api.handleBuildDuplicate(zip, httptest.NewRequest(http.MethodPost, "/local/modules/demo/builds/zip/duplicate", nil), "demo", "zip")
	if zip.Code != http.StatusBadRequest {
		t.Fatalf("ZIP duplicate status=%d body=%s", zip.Code, zip.Body.String())
	}
	dev := httptest.NewRecorder()
	api.handleBuildDuplicate(dev, httptest.NewRequest(http.MethodPost, "/local/modules/demo/builds/dev/duplicate", nil), "demo", "dev")
	if dev.Code != http.StatusBadRequest {
		t.Fatalf("source-only duplicate status=%d body=%s", dev.Code, dev.Body.String())
	}
	updated, err := loadLocalBuildIndex(api.indexPath("demo"))
	if err != nil || len(updated.Versions) != len(index.Versions) {
		t.Fatalf("rejected duplicate changed index: %#v, %v", updated, err)
	}
	index.Versions[0].HyperBricks = strings.TrimSpace(assets.VersionMD)
	if err := saveLocalBuildIndex(api.indexPath("demo"), index); err != nil {
		t.Fatal(err)
	}
	location, err := api.localPackageConfig("demo", "original")
	if err != nil {
		t.Fatal(err)
	}
	oversizedConfig := config + "#" + strings.Repeat("x", maxPackageConfigBytes)
	if err := os.WriteFile(location.path, []byte(oversizedConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	oversized := httptest.NewRecorder()
	api.handleBuildDuplicate(oversized, httptest.NewRequest(http.MethodPost, "/local/modules/demo/builds/original/duplicate", nil), "demo", "original")
	if oversized.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized config status=%d body=%s", oversized.Code, oversized.Body.String())
	}
}

func TestDuplicateRegistrationPreservesIndexChangesDuringSnapshot(t *testing.T) {
	t.Run("local", func(t *testing.T) {
		root := t.TempDir()
		api := &deployLocalServer{buildRoot: root}
		original := localBuildRow{BuildID: "original", Format: "hra", File: "original.hra", Commit: "source7", ModuleVersion: "1.0.0", HyperBricks: strings.TrimSpace(assets.VersionMD), RuntimeMode: "development"}
		index := localBuildIndex{Current: "original", Versions: []localBuildRow{original}}
		if err := saveLocalBuildIndex(api.indexPath("demo"), index); err != nil {
			t.Fatal(err)
		}
		// Simulate a build and mode change while snapshotting, before register.
		index.Current = "other"
		index.Versions[0].RuntimeMode = "live"
		index.Versions[0].Production = true
		index.Versions = append(index.Versions, localBuildRow{BuildID: "other", Format: "hra"})
		if err := saveLocalBuildIndex(api.indexPath("demo"), index); err != nil {
			t.Fatal(err)
		}
		snapshot := commands.RuntimeSnapshotResult{BuildID: "derived", ArchivePath: filepath.Join(root, "demo", "derived.hra"), ModuleVersion: "1.0.0", BuiltAt: "2026-09-24T12:00:00Z", Commit: "source7"}
		current, mode, status, err := api.registerDuplicateBuild("demo", "original", original, snapshot)
		if err != nil || status != http.StatusCreated || current != "other" || mode != "live" {
			t.Fatalf("register current=%q mode=%q status=%d error=%v", current, mode, status, err)
		}
		got, err := loadLocalBuildIndex(api.indexPath("demo"))
		if err != nil || got.Current != "other" || len(got.Versions) != 3 {
			t.Fatalf("index change lost: %#v, %v", got, err)
		}
		if row, ok := findLocalRow(got, "derived"); !ok || row.RuntimeMode != "live" || !row.Production {
			t.Fatalf("derived mode not inherited from latest row: %#v", row)
		}
	})
	t.Run("remote", func(t *testing.T) {
		root := t.TempDir()
		api := &deployAPI{root: root}
		original := deployIndexRow{BuildID: "original", Format: "hra", File: "original.hra", Commit: "source7", ModuleVersion: "1.0.0", HyperBricks: strings.TrimSpace(assets.VersionMD), RuntimeMode: "development"}
		index := deployIndex{Current: "original", Versions: []deployIndexRow{original}}
		if err := saveDeployIndex(api.indexPath("demo"), index); err != nil {
			t.Fatal(err)
		}
		index.Current = "other"
		index.Versions = append(index.Versions, deployIndexRow{BuildID: "other", Format: "hra"})
		if err := saveDeployIndex(api.indexPath("demo"), index); err != nil {
			t.Fatal(err)
		}
		snapshot := commands.RuntimeSnapshotResult{BuildID: "derived", ArchivePath: filepath.Join(root, "demo", "archives", "derived.hra"), ModuleVersion: "1.0.0", BuiltAt: "2026-09-24T12:00:00Z", Commit: "source7"}
		current, mode, status, err := api.registerDuplicateBuild("demo", "original", original, snapshot)
		if err != nil || status != http.StatusCreated || current != "other" || mode != "development" {
			t.Fatalf("register current=%q mode=%q status=%d error=%v", current, mode, status, err)
		}
		got, err := loadDeployIndex(api.indexPath("demo"))
		if err != nil || got.Current != "other" || len(got.Versions) != 3 {
			t.Fatalf("index change lost: %#v, %v", got, err)
		}
	})
}

func TestDuplicateMissingIndexedCommitDoesNotTrustEditedYAML(t *testing.T) {
	root := t.TempDir()
	api := &deployLocalServer{buildRoot: root}
	archivePath := filepath.Join(root, "demo", "demo-1.0.0-original.hra")
	config := strings.ReplaceAll(deployPackageFixture, "v1.2.5-beta", strings.TrimSpace(assets.VersionMD))
	writeDeployHRAFixture(t, archivePath, config)
	index := localBuildIndex{Current: "original", Versions: []localBuildRow{{
		BuildID: "original", Format: "hra", File: archivePath, HyperBricks: strings.TrimSpace(assets.VersionMD),
	}}}
	if err := saveLocalBuildIndex(api.indexPath("demo"), index); err != nil {
		t.Fatal(err)
	}
	location, err := api.localPackageConfig("demo", "original")
	if err != nil {
		t.Fatal(err)
	}
	forged := strings.Replace(config, "commit: fixture", "commit: forged", 1)
	if err := os.WriteFile(location.path, []byte(forged), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	api.handleBuildDuplicate(rec, httptest.NewRequest(http.MethodPost, "/local/modules/demo/builds/original/duplicate", nil), "demo", "original")
	if rec.Code != http.StatusCreated {
		t.Fatalf("duplicate status=%d body=%s", rec.Code, rec.Body.String())
	}
	row, _ := findLocalRow(mustLoadLocalIndex(t, api.indexPath("demo")), decodeResponseMap(t, rec)["build_id"].(string))
	if row.Commit != "unknown" || !strings.Contains(readHRAEntries(t, row.File)["package.hyperbricks.yaml"], "commit: unknown") {
		t.Fatalf("forged YAML commit propagated: %#v", row)
	}
}

func mustLoadLocalIndex(t *testing.T, path string) localBuildIndex {
	t.Helper()
	index, err := loadLocalBuildIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	return index
}

func readHRAEntries(t *testing.T, path string) map[string]string {
	t.Helper()
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	entries := make(map[string]string)
	for _, file := range archive.File {
		if file.FileInfo().IsDir() {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		entries[file.Name] = string(content)
	}
	return entries
}
