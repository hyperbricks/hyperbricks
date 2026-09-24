package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

const deployPackageFixture = `hyperbricks:
  mode: development
  metadata:
    module: demo
    moduleversion: "1.0.0"
    format: hra
    format_version: "1"
    commit: fixture
    built_at: "2026-09-24T12:00:00Z"
    hyperbricks: v1.2.5-beta
  live:
    cache: 10m
`

func writeDeployHRAFixture(t *testing.T, path string, config string) []byte {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create archive directory: %v", err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("package.hyperbricks.yaml")
	if err != nil {
		t.Fatalf("create config entry: %v", err)
	}
	if _, err := io.WriteString(entry, config); err != nil {
		t.Fatalf("write config entry: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close archive writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	return data
}

func decodeResponseMap(t *testing.T, recorder *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
	return payload
}

func TestLocalPackageConfigEditorAndArchiveDownload(t *testing.T) {
	root := t.TempDir()
	modulesDir := filepath.Join(root, "modules")
	buildRoot := filepath.Join(root, "deploy")
	moduleDir := filepath.Join(modulesDir, "demo")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "package.hyperbricks.yaml"), []byte(deployPackageFixture), 0o640); err != nil {
		t.Fatal(err)
	}

	archivePath := filepath.Join(buildRoot, "demo", "demo-1.0.0-build-1.hra")
	originalArchive := writeDeployHRAFixture(t, archivePath, deployPackageFixture)
	zipPath := filepath.Join(buildRoot, "demo", "demo-1.0.0-build-zip.zip")
	writeDeployHRAFixture(t, zipPath, deployPackageFixture)
	index := localBuildIndex{Current: "build-1", Versions: []localBuildRow{
		{BuildID: "build-1", Format: "hra", File: archivePath, RuntimeMode: "development"},
		{BuildID: "build-zip", Format: "zip", File: zipPath, RuntimeMode: "development"},
	}}
	api := &deployLocalServer{modulesDir: modulesDir, buildRoot: buildRoot}
	if err := saveLocalBuildIndex(api.indexPath("demo"), index); err != nil {
		t.Fatal(err)
	}

	get := httptest.NewRecorder()
	api.handleBuildPackageConfig(get, httptest.NewRequest(http.MethodGet, "/local/modules/demo/builds/build-1/package-config", nil), "demo", "build-1")
	if get.Code != http.StatusOK {
		t.Fatalf("GET package config status=%d body=%s", get.Code, get.Body.String())
	}
	opened := decodeResponseMap(t, get)
	if opened["scope"] != "runtime" || opened["content"] != deployPackageFixture {
		t.Fatalf("opened package config = %#v", opened)
	}

	updated := strings.Replace(deployPackageFixture, "cache: 10m", "cache: 0s", 1)
	requestBody, _ := json.Marshal(packageConfigUpdateRequest{Content: updated, ExpectedSHA256: opened["sha256"].(string)})
	put := httptest.NewRecorder()
	api.handleBuildPackageConfig(put, httptest.NewRequest(http.MethodPut, "/local/modules/demo/builds/build-1/package-config", bytes.NewReader(requestBody)), "demo", "build-1")
	if put.Code != http.StatusOK {
		t.Fatalf("PUT package config status=%d body=%s", put.Code, put.Body.String())
	}
	runtimeConfig := filepath.Join(buildRoot, "demo", "runtime", "build-1", "package.hyperbricks.yaml")
	gotConfig, err := os.ReadFile(runtimeConfig)
	if err != nil || string(gotConfig) != updated {
		t.Fatalf("runtime config=%q err=%v", gotConfig, err)
	}
	archiveAfter, err := os.ReadFile(archivePath)
	if err != nil || !bytes.Equal(archiveAfter, originalArchive) {
		t.Fatal("editing runtime configuration changed the immutable HRA")
	}

	staleBody, _ := json.Marshal(packageConfigUpdateRequest{Content: deployPackageFixture, ExpectedSHA256: opened["sha256"].(string)})
	stale := httptest.NewRecorder()
	api.handleBuildPackageConfig(stale, httptest.NewRequest(http.MethodPut, "/local/modules/demo/builds/build-1/package-config", bytes.NewReader(staleBody)), "demo", "build-1")
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale PUT status=%d body=%s", stale.Code, stale.Body.String())
	}

	invalidBody, _ := json.Marshal(packageConfigUpdateRequest{Content: "hyperbricks: [\n", ExpectedSHA256: packageConfigSHA256(gotConfig)})
	invalid := httptest.NewRecorder()
	api.handleBuildPackageConfig(invalid, httptest.NewRequest(http.MethodPut, "/local/modules/demo/builds/build-1/package-config", bytes.NewReader(invalidBody)), "demo", "build-1")
	if invalid.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid PUT status=%d body=%s", invalid.Code, invalid.Body.String())
	}
	afterInvalid, _ := os.ReadFile(runtimeConfig)
	if !bytes.Equal(afterInvalid, gotConfig) {
		t.Fatal("invalid package edit changed the runtime configuration")
	}

	download := httptest.NewRecorder()
	api.handleBuildArchive(download, httptest.NewRequest(http.MethodGet, "/local/modules/demo/builds/build-1/archive", nil), "demo", "build-1")
	if download.Code != http.StatusOK || !bytes.Equal(download.Body.Bytes(), originalArchive) {
		t.Fatalf("archive download status=%d bytes=%d", download.Code, download.Body.Len())
	}
	if got := download.Header().Get("Content-Type"); got != "application/vnd.hyperbricks.hra" {
		t.Fatalf("archive content type=%q", got)
	}
	if got := download.Header().Get("Content-Disposition"); !strings.Contains(got, filepath.Base(archivePath)) {
		t.Fatalf("archive disposition=%q", got)
	}

	dev := httptest.NewRecorder()
	api.handleBuildPackageConfig(dev, httptest.NewRequest(http.MethodGet, "/local/modules/demo/builds/dev/package-config", nil), "demo", "dev")
	devOpened := decodeResponseMap(t, dev)
	if dev.Code != http.StatusOK || devOpened["scope"] != "source" {
		t.Fatalf("source package response status=%d body=%s", dev.Code, dev.Body.String())
	}
	devUpdated := strings.Replace(deployPackageFixture, "mode: development", "mode: live", 1)
	devBody, _ := json.Marshal(packageConfigUpdateRequest{Content: devUpdated, ExpectedSHA256: devOpened["sha256"].(string)})
	devPut := httptest.NewRecorder()
	api.handleBuildPackageConfig(devPut, httptest.NewRequest(http.MethodPut, "/local/modules/demo/builds/dev/package-config", bytes.NewReader(devBody)), "demo", "dev")
	if devPut.Code != http.StatusOK {
		t.Fatalf("source package PUT status=%d body=%s", devPut.Code, devPut.Body.String())
	}
	devSaved, err := os.ReadFile(filepath.Join(moduleDir, "package.hyperbricks.yaml"))
	if err != nil || string(devSaved) != devUpdated {
		t.Fatalf("source package config=%q err=%v", devSaved, err)
	}
	devInfo, err := os.Stat(filepath.Join(moduleDir, "package.hyperbricks.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if devInfo.Mode().Perm() != 0o640 {
		t.Fatalf("source package permissions=%v, want 0640", devInfo.Mode().Perm())
	}
	devArchive := httptest.NewRecorder()
	api.handleBuildArchive(devArchive, httptest.NewRequest(http.MethodGet, "/local/modules/demo/builds/dev/archive", nil), "demo", "dev")
	if devArchive.Code != http.StatusBadRequest {
		t.Fatalf("dev archive status=%d", devArchive.Code)
	}

	zipConfig := httptest.NewRecorder()
	api.handleBuildPackageConfig(zipConfig, httptest.NewRequest(http.MethodGet, "/local/modules/demo/builds/build-zip/package-config", nil), "demo", "build-zip")
	if zipConfig.Code != http.StatusOK || decodeResponseMap(t, zipConfig)["scope"] != "runtime" {
		t.Fatalf("ZIP package config status=%d body=%s", zipConfig.Code, zipConfig.Body.String())
	}
	zipArchive := httptest.NewRecorder()
	api.handleBuildArchive(zipArchive, httptest.NewRequest(http.MethodGet, "/local/modules/demo/builds/build-zip/archive", nil), "demo", "build-zip")
	if zipArchive.Code != http.StatusBadRequest {
		t.Fatalf("ZIP archive download status=%d", zipArchive.Code)
	}

	oversizedBody, _ := json.Marshal(packageConfigUpdateRequest{
		Content:        strings.Repeat("x", maxPackageConfigBytes+1),
		ExpectedSHA256: packageConfigSHA256(gotConfig),
	})
	oversized := httptest.NewRecorder()
	api.handleBuildPackageConfig(oversized, httptest.NewRequest(http.MethodPut, "/local/modules/demo/builds/build-1/package-config", bytes.NewReader(oversizedBody)), "demo", "build-1")
	if oversized.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized package PUT status=%d body=%s", oversized.Code, oversized.Body.String())
	}
}

func TestPackageConfigSavePreservesRawText(t *testing.T) {
	moduleRoot := t.TempDir()
	configPath := filepath.Join(moduleRoot, "package.hyperbricks.yaml")
	if err := os.WriteFile(configPath, []byte(deployPackageFixture), 0o640); err != nil {
		t.Fatal(err)
	}

	raw := "# keep this editor comment\r\n" +
		"free_variables:\r\n" +
		"  version_like_value: \"001\"\r\n" +
		"\r\n" +
		strings.ReplaceAll(deployPackageFixture, "\n", "\r\n")
	location := deployPackageConfigLocation{moduleRoot: moduleRoot, path: configPath, scope: "runtime"}
	saved, status, err := savePackageConfig(location, packageConfigUpdateRequest{
		Content:        raw,
		ExpectedSHA256: packageConfigSHA256([]byte(deployPackageFixture)),
	})
	if err != nil || status != http.StatusOK {
		t.Fatalf("save raw package config status=%d err=%v", status, err)
	}
	if !bytes.Equal(saved, []byte(raw)) {
		t.Fatalf("save response changed editor text:\n%q", saved)
	}
	onDisk, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(onDisk, []byte(raw)) {
		t.Fatalf("on-disk config changed editor text:\n%q", onDisk)
	}
	if got, want := packageConfigSHA256(onDisk), packageConfigSHA256([]byte(raw)); got != want {
		t.Fatalf("saved SHA-256 = %s, want %s", got, want)
	}
}

func TestRemoteArchiveAndPackageConfigAreConfined(t *testing.T) {
	root := t.TempDir()
	archives := filepath.Join(root, "demo", "archives")
	archivePath := filepath.Join(archives, "demo-1.0.0-build-1.hra")
	wantArchive := writeDeployHRAFixture(t, archivePath, deployPackageFixture)
	api := &deployAPI{root: root}
	index := deployIndex{Current: "build-1", Versions: []deployIndexRow{{
		BuildID: "build-1", Format: "hra", File: archivePath, RuntimeMode: "live", Production: true,
	}}}
	if err := saveDeployIndex(api.indexPath("demo"), index); err != nil {
		t.Fatal(err)
	}

	download := httptest.NewRecorder()
	api.handleDeploy(download, httptest.NewRequest(http.MethodGet, "/deploy/modules/demo/builds/build-1/archive", nil))
	if download.Code != http.StatusOK || !bytes.Equal(download.Body.Bytes(), wantArchive) {
		t.Fatalf("remote archive status=%d body=%q", download.Code, download.Body.String())
	}

	config := httptest.NewRecorder()
	api.handleDeploy(config, httptest.NewRequest(http.MethodGet, "/deploy/modules/demo/builds/build-1/package-config", nil))
	if config.Code != http.StatusOK || decodeResponseMap(t, config)["scope"] != "runtime" {
		t.Fatalf("remote package status=%d body=%s", config.Code, config.Body.String())
	}

	outside := filepath.Join(root, "outside.hra")
	writeDeployHRAFixture(t, outside, deployPackageFixture)
	index.Versions[0].File = outside
	if err := saveDeployIndex(api.indexPath("demo"), index); err != nil {
		t.Fatal(err)
	}
	escape := httptest.NewRecorder()
	api.handleBuildArchive(escape, httptest.NewRequest(http.MethodGet, "/deploy/modules/demo/builds/build-1/archive", nil), "demo", "build-1")
	if escape.Code != http.StatusBadRequest {
		t.Fatalf("outside archive status=%d body=%s", escape.Code, escape.Body.String())
	}

	if runtime.GOOS != "windows" {
		link := filepath.Join(archives, "linked.hra")
		if err := os.Symlink(outside, link); err != nil {
			t.Fatal(err)
		}
		index.Versions[0].File = link
		if err := saveDeployIndex(api.indexPath("demo"), index); err != nil {
			t.Fatal(err)
		}
		symlink := httptest.NewRecorder()
		api.handleBuildArchive(symlink, httptest.NewRequest(http.MethodGet, "/deploy/modules/demo/builds/build-1/archive", nil), "demo", "build-1")
		if symlink.Code != http.StatusBadRequest {
			t.Fatalf("symlink archive status=%d body=%s", symlink.Code, symlink.Body.String())
		}

		symlinkRoot := filepath.Join(root, "symlink-root")
		outsideModule := filepath.Join(root, "outside-module")
		outsideArchives := filepath.Join(outsideModule, "archives")
		outsideArchive := filepath.Join(outsideArchives, "demo-1.0.0-parent-link.hra")
		writeDeployHRAFixture(t, outsideArchive, deployPackageFixture)
		if err := os.MkdirAll(symlinkRoot, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outsideModule, filepath.Join(symlinkRoot, "demo")); err != nil {
			t.Fatal(err)
		}
		symlinkAPI := &deployAPI{root: symlinkRoot}
		symlinkIndex := deployIndex{Current: "parent-link", Versions: []deployIndexRow{{
			BuildID: "parent-link", Format: "hra", File: outsideArchive, RuntimeMode: "live", Production: true,
		}}}
		if err := saveDeployIndex(symlinkAPI.indexPath("demo"), symlinkIndex); err != nil {
			t.Fatal(err)
		}
		parentLink := httptest.NewRecorder()
		symlinkAPI.handleBuildArchive(parentLink, httptest.NewRequest(http.MethodGet, "/deploy/modules/demo/builds/parent-link/archive", nil), "demo", "parent-link")
		if parentLink.Code != http.StatusBadRequest {
			t.Fatalf("parent symlink archive status=%d body=%s", parentLink.Code, parentLink.Body.String())
		}
	}
}

func TestRemoteArchiveAndPackageConfigSupportModuleRootLayout(t *testing.T) {
	root := t.TempDir()
	moduleRoot := filepath.Join(root, "demo")
	archivePath := filepath.Join(moduleRoot, "demo-1.0.0-build-root.hra")
	wantArchive := writeDeployHRAFixture(t, archivePath, deployPackageFixture)
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relativeArchivePath, err := filepath.Rel(workingDirectory, archivePath)
	if err != nil {
		t.Fatal(err)
	}
	api := &deployAPI{root: root}
	index := deployIndex{Current: "build-root", Versions: []deployIndexRow{{
		BuildID: "build-root", Format: "hra", File: filepath.ToSlash(relativeArchivePath), RuntimeMode: "live", Production: true,
	}}}
	if err := saveDeployIndex(api.indexPath("demo"), index); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(moduleRoot, "archives")); !os.IsNotExist(err) {
		t.Fatalf("archives directory unexpectedly exists: %v", err)
	}

	download := httptest.NewRecorder()
	api.handleDeploy(download, httptest.NewRequest(http.MethodGet, "/deploy/modules/demo/builds/build-root/archive", nil))
	if download.Code != http.StatusOK || !bytes.Equal(download.Body.Bytes(), wantArchive) {
		t.Fatalf("module-root archive status=%d body=%q", download.Code, download.Body.String())
	}

	config := httptest.NewRecorder()
	api.handleDeploy(config, httptest.NewRequest(http.MethodGet, "/deploy/modules/demo/builds/build-root/package-config", nil))
	if config.Code != http.StatusOK {
		t.Fatalf("module-root package status=%d body=%s", config.Code, config.Body.String())
	}
	opened := decodeResponseMap(t, config)
	if opened["scope"] != "runtime" || opened["content"] != deployPackageFixture {
		t.Fatalf("module-root package config = %#v", opened)
	}

	updated := strings.Replace(deployPackageFixture, "cache: 10m", "cache: 0s", 1)
	requestBody, _ := json.Marshal(packageConfigUpdateRequest{
		Content:        updated,
		ExpectedSHA256: opened["sha256"].(string),
	})
	put := httptest.NewRecorder()
	api.handleDeploy(put, httptest.NewRequest(http.MethodPut, "/deploy/modules/demo/builds/build-root/package-config", bytes.NewReader(requestBody)))
	if put.Code != http.StatusOK {
		t.Fatalf("module-root package PUT status=%d body=%s", put.Code, put.Body.String())
	}
	runtimeConfig := filepath.Join(moduleRoot, "runtime", "build-root", "package.hyperbricks.yaml")
	gotConfig, err := os.ReadFile(runtimeConfig)
	if err != nil || string(gotConfig) != updated {
		t.Fatalf("module-root runtime config=%q err=%v", gotConfig, err)
	}
	archiveAfter, err := os.ReadFile(archivePath)
	if err != nil || !bytes.Equal(archiveAfter, wantArchive) {
		t.Fatal("editing the module-root runtime configuration changed the immutable HRA")
	}

	incomingArchive := filepath.Join(moduleRoot, "incoming", "staged.hra")
	writeDeployHRAFixture(t, incomingArchive, deployPackageFixture)
	index.Versions[0].File = incomingArchive
	if err := saveDeployIndex(api.indexPath("demo"), index); err != nil {
		t.Fatal(err)
	}
	staged := httptest.NewRecorder()
	api.handleDeploy(staged, httptest.NewRequest(http.MethodGet, "/deploy/modules/demo/builds/build-root/archive", nil))
	if staged.Code != http.StatusBadRequest {
		t.Fatalf("incoming archive status=%d body=%s", staged.Code, staged.Body.String())
	}
	if runtime.GOOS != "windows" {
		if err := os.Symlink(filepath.Join(moduleRoot, "incoming"), filepath.Join(moduleRoot, "archives")); err != nil {
			t.Fatal(err)
		}
		aliased := httptest.NewRecorder()
		api.handleDeploy(aliased, httptest.NewRequest(http.MethodGet, "/deploy/modules/demo/builds/build-root/archive", nil))
		if aliased.Code != http.StatusBadRequest {
			t.Fatalf("archives symlink alias status=%d body=%s", aliased.Code, aliased.Body.String())
		}
	}
}

func TestPackageConfigConcurrentWritesRejectStaleWriters(t *testing.T) {
	moduleRoot := t.TempDir()
	configPath := filepath.Join(moduleRoot, "package.hyperbricks.yaml")
	if err := os.WriteFile(configPath, []byte(deployPackageFixture), 0o640); err != nil {
		t.Fatal(err)
	}
	location := deployPackageConfigLocation{moduleRoot: moduleRoot, path: configPath, scope: "runtime"}
	expected := packageConfigSHA256([]byte(deployPackageFixture))

	const writers = 16
	start := make(chan struct{})
	results := make(chan int, writers)
	var wait sync.WaitGroup
	for index := 0; index < writers; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			content := deployPackageFixture + fmt.Sprintf("# writer %d\n", index)
			_, status, err := savePackageConfig(location, packageConfigUpdateRequest{
				Content:        content,
				ExpectedSHA256: expected,
			})
			if err == nil {
				results <- status
				return
			}
			results <- status
		}(index)
	}
	close(start)
	wait.Wait()
	close(results)

	successes := 0
	conflicts := 0
	for status := range results {
		switch status {
		case http.StatusOK:
			successes++
		case http.StatusConflict:
			conflicts++
		default:
			t.Fatalf("unexpected concurrent save status %d", status)
		}
	}
	if successes != 1 || conflicts != writers-1 {
		t.Fatalf("concurrent saves = %d success, %d conflicts; want 1 and %d", successes, conflicts, writers-1)
	}
}

func TestBuildRuntimeModeCompatibilityAndHandlers(t *testing.T) {
	root := t.TempDir()
	localPath := filepath.Join(root, "local", "hyperbricks.versions.json")
	legacy := `{"versions":[{"build_id":"legacy-live","production":true},{"build_id":"legacy-default"}]}`
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(localPath, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadLocalBuildIndex(localPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Versions[0].RuntimeMode != "live" || loaded.Versions[1].RuntimeMode != "development" {
		t.Fatalf("legacy modes = %#v", loaded.Versions)
	}

	api := &deployLocalServer{buildRoot: filepath.Join(root, "deploy")}
	index := localBuildIndex{Versions: []localBuildRow{{BuildID: "build-1", RuntimeMode: "development"}}}
	if err := saveLocalBuildIndex(api.indexPath("demo"), index); err != nil {
		t.Fatal(err)
	}
	body := strings.NewReader(`{"mode":"live"}`)
	response := httptest.NewRecorder()
	api.handleBuildMode(response, httptest.NewRequest(http.MethodPut, "/local/modules/demo/builds/build-1/mode", body), "demo", "build-1")
	if response.Code != http.StatusOK {
		t.Fatalf("mode update status=%d body=%s", response.Code, response.Body.String())
	}
	updated, err := loadLocalBuildIndex(api.indexPath("demo"))
	if err != nil || updated.Versions[0].RuntimeMode != "live" || !updated.Versions[0].Production {
		t.Fatalf("updated mode=%#v err=%v", updated.Versions, err)
	}
	invalid := httptest.NewRecorder()
	api.handleBuildMode(invalid, httptest.NewRequest(http.MethodPut, "/local/modules/demo/builds/build-1/mode", strings.NewReader(`{"mode":"default"}`)), "demo", "build-1")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid mode status=%d", invalid.Code)
	}
}
