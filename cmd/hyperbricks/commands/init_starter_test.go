package commands

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/hyperbricks/hyperbricks/assets"
)

const testStarterPackageConfig = "hyperbricks:\n  metadata:\n    module: old-starter-name\n    moduleversion: \"9.4\"\n    format: zip\n    format_version: \"0\"\n    commit: stale\n    built_at: \"1970-01-01T00:00:00Z\"\n    hyperbricks: v0.0.0\n    source_hash: stale\n  custom:\n    preserved: true\n"
const testStarterYAMLSource = "page:\n  - type: hypermedia\n  - route: index\n  - body:\n      - type: text\n      - value: HELLO WORLD!\n"

func TestNormalizeStarterRef(t *testing.T) {
	for _, test := range []struct {
		input, want string
		valid       bool
	}{
		{"", "main", true},
		{"latest", "main", true},
		{"v1.2.9-beta", "v1.2.9-beta", true},
		{"release/v1.2.9-beta", "release/v1.2.9-beta", true},
		{"a1b2c3d4a1b2c3d4a1b2c3d4a1b2c3d4a1b2c3d4", "a1b2c3d4a1b2c3d4a1b2c3d4a1b2c3d4a1b2c3d4", true},
		{"../main", "", false},
		{"release//v1", "", false},
		{"release/../v1", "", false},
		{"tag..bad", "", false},
		{"tag.lock", "", false},
		{"@{main}", "", false},
		{"main?raw=1", "", false},
	} {
		got, err := normalizeStarterRef(test.input)
		if (err == nil) != test.valid || got != test.want {
			t.Errorf("normalizeStarterRef(%q) = %q, %v; want %q, valid=%v", test.input, got, err, test.want, test.valid)
		}
	}
}

func TestStarterArchiveRefPathUsesDocumentedGitHubRoutes(t *testing.T) {
	for _, test := range []struct {
		ref, want string
	}{
		{"main", "refs/heads/main"},
		{"v1.2.9-beta", "refs/tags/v1.2.9-beta"},
		{"release/v1.2.9-beta", "refs/tags/release/v1.2.9-beta"},
		{"deadbeef", "refs/tags/deadbeef"},
		{"a1b2c3d4a1b2c3d4a1b2c3d4a1b2c3d4a1b2c3d4", "a1b2c3d4a1b2c3d4a1b2c3d4a1b2c3d4a1b2c3d4"},
		{"ABCDEF0123456789ABCDEF0123456789ABCDEF01", "ABCDEF0123456789ABCDEF0123456789ABCDEF01"},
	} {
		if got := starterArchiveRefPath(test.ref); got != test.want {
			t.Errorf("archive path for %q = %q, want %q", test.ref, got, test.want)
		}
	}
}

func TestParseStarterArg(t *testing.T) {
	for _, raw := range []string{"", "hello-world@", "../hello-world", "hello-world@latest@main"} {
		if _, _, err := parseStarterArg(raw); err == nil {
			t.Errorf("parseStarterArg(%q) accepted invalid argument", raw)
		}
	}
	name, ref, err := parseStarterArg("hello-world@v1.2.9-beta")
	if err != nil || name != "hello-world" || ref != "v1.2.9-beta" {
		t.Fatalf("parsed starter = %q, %q, %v", name, ref, err)
	}
}

func TestDecodeStarterIndexRequiresCurrentModulePaths(t *testing.T) {
	for _, test := range []struct {
		name, index string
	}{
		{"fixture module", "{\"hello-world\":{\"path\":\"modules/api-security-test\"}}"},
		{"old repository path", "{\"hello-world\":{\"path\":\"starters/hello-world/1.0.0\"}}"},
		{"traversal", "{\"hello-world\":{\"path\":\"modules/../api-security-test\"}}"},
		{"old versioned index", "{\"hello-world\":{\"1.0.0\":{\"path\":\"modules/hello-world\"}}}"},
		{"old version field", "{\"hello-world\":{\"version\":\"1.0.0\",\"path\":\"modules/hello-world\"}}"},
		{"invalid compatibility", "{\"hello-world\":{\"path\":\"modules/hello-world\",\"compatible_hyperbricks\":[\">=broken\"]}}"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodeStarterIndex(strings.NewReader(test.index)); err == nil {
				t.Fatal("expected index validation error")
			}
		})
	}
	starters, err := decodeStarterIndex(strings.NewReader("{\"hello-world\":{\"description\":\"Minimal\"}}"))
	if err != nil {
		t.Fatalf("decode defaulted starter: %v", err)
	}
	meta := starters["hello-world"]
	if meta.Path != "modules/hello-world" || meta.Entrypoint != "package.hyperbricks.yaml" {
		t.Fatalf("incorrect defaults: %+v", meta)
	}
}

func TestRepositoryStarterIndexContainsOnlyRunnableCatalogModules(t *testing.T) {
	repositoryRoot := filepath.Join("..", "..", "..")
	indexFile, err := os.Open(filepath.Join(repositoryRoot, "starters.index.json"))
	if err != nil {
		t.Fatalf("open repository starter index: %v", err)
	}
	defer indexFile.Close()
	starters, err := decodeStarterIndex(indexFile)
	if err != nil {
		t.Fatalf("decode repository starter index: %v", err)
	}
	if len(starters) == 0 {
		t.Fatal("repository starter index is empty")
	}
	for name, meta := range starters {
		packagePath := filepath.Join(repositoryRoot, filepath.FromSlash(meta.Path), meta.Entrypoint)
		if info, err := os.Stat(packagePath); err != nil || !info.Mode().IsRegular() {
			t.Errorf("starter %s has no package at %s: %v", name, packagePath, err)
		}
	}
	readme, err := os.ReadFile(filepath.Join(repositoryRoot, "modules", "README.md"))
	if err != nil {
		t.Fatalf("read module index: %v", err)
	}
	starterSection := strings.SplitN(string(readme), "### Starter modules", 2)
	if len(starterSection) != 2 {
		t.Fatal("module index has no Starter modules section")
	}
	starterRows := strings.SplitN(starterSection[1], "\n## ", 2)[0]
	catalogNames := namesFromModuleTable(starterRows)
	for name := range catalogNames {
		if _, found := starters[name]; !found {
			t.Errorf("catalog starter %s is missing from starters.index.json", name)
		}
	}
	for name := range starters {
		if _, found := catalogNames[name]; !found {
			t.Errorf("indexed starter %s is absent from Starter modules table", name)
		}
	}
	fixtureSection := strings.SplitN(string(readme), "### Fixture modules", 2)
	if len(fixtureSection) != 2 {
		t.Fatal("module index has no Fixture modules section")
	}
	fixtureRows := strings.SplitN(fixtureSection[1], "\n## ", 2)[0]
	for name := range namesFromModuleTable(fixtureRows) {
		if _, found := starters[name]; found {
			t.Errorf("test fixture %s is offered by init-starter", name)
		}
	}
}

func namesFromModuleTable(table string) map[string]struct{} {
	names := make(map[string]struct{})
	for _, line := range strings.Split(table, "\n") {
		if !strings.HasPrefix(line, "| [") {
			continue
		}
		fields := strings.Split(line, "\x60")
		if len(fields) < 2 {
			continue
		}
		names[fields[1]] = struct{}{}
	}
	return names
}

func TestRunInitStarterGetRejectsInvalidInputBeforeNetwork(t *testing.T) {
	_, _, err := runInitStarterGet("hello-world", "../outside")
	if err == nil || !strings.Contains(err.Error(), "module must be a name below ./modules") {
		t.Fatalf("invalid module override error = %v", err)
	}
	_, _, err = runInitStarterGet("hello-world@../main", "")
	if err == nil || !strings.Contains(err.Error(), "invalid starter Git ref") {
		t.Fatalf("invalid Git ref error = %v", err)
	}
}

func TestRunInitStarterGetReadsIndexAndModuleFromSameArchive(t *testing.T) {
	useStarterWorkingDirectory(t)
	archive := starterArchiveFixture(t, "hyperbricks-main", starterIndexFixture(), testStarterPackageConfig)
	requests := useStarterServer(t, map[string][]byte{"main": archive}, nil)
	moduleName, meta, err := runInitStarterGet("hello-world", "example-site")
	if err != nil {
		t.Fatalf("runInitStarterGet returned error: %v", err)
	}
	if moduleName != "example-site" || meta.Name != "hello-world" || meta.Path != "modules/hello-world" {
		t.Fatalf("installed starter = %q, %+v", moduleName, meta)
	}
	if got := requests(); !reflect.DeepEqual(got, []string{"/archive/refs/heads/main.zip"}) {
		t.Fatalf("get requests = %v; index should come from the downloaded archive", got)
	}
	for _, path := range []string{
		"modules/example-site/package.hyperbricks.yaml",
		"modules/example-site/hyperbricks/hello-world.hyperbricks.yaml",
		"modules/example-site/templates",
		"modules/example-site/static",
		"modules/example-site/resources",
		"modules/example-site/rendered",
		"modules/example-site/logs",
		"bin/plugins",
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
	}
	data, err := os.ReadFile("modules/example-site/package.hyperbricks.yaml")
	if err != nil {
		t.Fatalf("read installed package: %v", err)
	}
	for _, want := range []string{"module: example-site", "moduleversion: \"1.0.0\"", "hyperbricks: " + strings.TrimSpace(assets.VersionMD), "preserved: true"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("installed package lacks %q:\n%s", want, data)
		}
	}
	for _, unwanted := range []string{"format:", "commit:", "built_at:", "source_hash:"} {
		if strings.Contains(string(data), unwanted) {
			t.Fatalf("installed source package contains artifact metadata %q", unwanted)
		}
	}
	if _, err := os.Stat("modules/example-site/manifest.json"); !os.IsNotExist(err) {
		t.Fatalf("starter manifest should not be installed: %v", err)
	}
}

func TestRunInitStarterGetUsesRequestedTagOrCommit(t *testing.T) {
	for _, ref := range []string{"v1.2.9-beta", "release/v1.2.9-beta", "a1b2c3d4a1b2c3d4a1b2c3d4a1b2c3d4a1b2c3d4"} {
		t.Run(ref, func(t *testing.T) {
			useStarterWorkingDirectory(t)
			archive := starterArchiveFixture(t, "hyperbricks-"+strings.ReplaceAll(ref, "/", "-"), starterIndexFixture(), testStarterPackageConfig)
			requests := useStarterServer(t, map[string][]byte{ref: archive}, nil)
			name, _, err := runInitStarterGet("hello-world@"+ref, "")
			if err != nil || name != "hello-world" {
				t.Fatalf("install from %s = %q, %v", ref, name, err)
			}
			if got := requests(); !reflect.DeepEqual(got, []string{"/archive/" + starterArchiveRefPath(ref) + ".zip"}) {
				t.Fatalf("archive requests = %v", got)
			}
		})
	}
}

func TestFetchStarterIndexUsesSelectedRefForList(t *testing.T) {
	index, err := json.Marshal(starterIndexFixture())
	if err != nil {
		t.Fatalf("encode index: %v", err)
	}
	for _, ref := range []string{
		"main",
		"v1.2.9-beta",
		"release/v1.2.9-beta",
		"a1b2c3d4a1b2c3d4a1b2c3d4a1b2c3d4a1b2c3d4",
	} {
		t.Run(ref, func(t *testing.T) {
			requests := useStarterServer(t, nil, map[string][]byte{ref: index})
			starters, err := fetchStarterIndex(ref)
			if err != nil || starters["hello-world"].Path != "modules/hello-world" {
				t.Fatalf("fetch selected index = %+v, %v", starters, err)
			}
			expected := "/contents/starters.index.json?ref=" + url.QueryEscape(starterArchiveRefPath(ref))
			if got := requests(); !reflect.DeepEqual(got, []string{expected}) {
				t.Fatalf("index requests = %v; want %s", got, expected)
			}
		})
	}
	if InitStarterListCommand().Flags().Lookup("ref") == nil {
		t.Fatal("list command lacks --ref")
	}
}

func TestRunInitStarterGetRejectsIncompatibleStarter(t *testing.T) {
	useStarterWorkingDirectory(t)
	index := starterIndexFixture()
	meta := index["hello-world"]
	meta.CompatibleHyperbricks = []string{">=999.0.0"}
	index["hello-world"] = meta
	useStarterServer(t, map[string][]byte{"main": starterArchiveFixture(t, "hyperbricks-main", index, testStarterPackageConfig)}, nil)
	_, _, err := runInitStarterGet("hello-world", "demo")
	if err == nil || !strings.Contains(err.Error(), "not compatible") {
		t.Fatalf("incompatible starter error = %v", err)
	}
	if _, statErr := os.Stat("modules/demo"); !os.IsNotExist(statErr) {
		t.Fatalf("incompatible starter installed: %v", statErr)
	}
}

func TestRunInitStarterGetKeepsPluginModuleName(t *testing.T) {
	useStarterWorkingDirectory(t)
	index := starterIndexFixture()
	meta := index["hello-world"]
	meta.FixedModuleName = true
	index["hello-world"] = meta
	useStarterServer(t, map[string][]byte{"main": starterArchiveFixture(t, "hyperbricks-main", index, testStarterPackageConfig)}, nil)
	_, _, err := runInitStarterGet("hello-world", "renamed-site")
	if err == nil || !strings.Contains(err.Error(), "requires module name") {
		t.Fatalf("plugin module rename error = %v", err)
	}
	if _, statErr := os.Stat("modules/renamed-site"); !os.IsNotExist(statErr) {
		t.Fatalf("renamed module was extracted: %v", statErr)
	}
	name, _, err := runInitStarterGet("hello-world", "")
	if err != nil || name != "hello-world" {
		t.Fatalf("install under required module name = %q, %v", name, err)
	}
}

func TestRunInitStarterGetRejectsNonEmptyModuleDir(t *testing.T) {
	useStarterWorkingDirectory(t)
	useStarterServer(t, map[string][]byte{"main": starterArchiveFixture(t, "hyperbricks-main", starterIndexFixture(), testStarterPackageConfig)}, nil)
	if err := os.MkdirAll("modules/hello-world", 0755); err != nil {
		t.Fatalf("create module: %v", err)
	}
	if err := os.WriteFile("modules/hello-world/existing.txt", []byte("occupied"), 0644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}
	_, _, err := runInitStarterGet("hello-world", "")
	if err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("non-empty destination error = %v", err)
	}
}

func TestRunInitStarterGetInvalidPackageKeepsDestination(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "empty"}[existing], func(t *testing.T) {
			useStarterWorkingDirectory(t)
			archive := starterArchiveFixture(t, "hyperbricks-main", starterIndexFixture(), "hyperbricks:\n  metadata: [invalid\n")
			useStarterServer(t, map[string][]byte{"main": archive}, nil)
			moduleDir := "modules/broken-site"
			if existing {
				if err := os.MkdirAll(moduleDir, 0711); err != nil {
					t.Fatalf("create empty destination: %v", err)
				}
			}
			_, _, err := runInitStarterGet("hello-world", "broken-site")
			if err == nil || !strings.Contains(err.Error(), "prepare starter package metadata") {
				t.Fatalf("staged metadata error = %v", err)
			}
			info, statErr := os.Stat(moduleDir)
			if existing {
				if statErr != nil || !info.IsDir() || info.Mode().Perm() != 0711 {
					t.Fatalf("empty destination changed: %+v, %v", info, statErr)
				}
				entries, err := os.ReadDir(moduleDir)
				if err != nil || len(entries) != 0 {
					t.Fatalf("empty destination contents = %v, %v", entries, err)
				}
			} else if !os.IsNotExist(statErr) {
				t.Fatalf("missing destination was created: %v", statErr)
			}
			assertNoStarterStagingDirectories(t, "broken-site")
		})
	}
}

func TestRunInitStarterGetRequiresIndexInArchive(t *testing.T) {
	useStarterWorkingDirectory(t)
	archive := makeStarterZip(t, map[string]string{"hyperbricks-main/modules/hello-world/package.hyperbricks.yaml": testStarterPackageConfig})
	useStarterServer(t, map[string][]byte{"main": archive}, nil)
	_, _, err := runInitStarterGet("hello-world", "demo")
	if err == nil || !strings.Contains(err.Error(), "starter index not found") {
		t.Fatalf("missing index error = %v", err)
	}
}

func TestDownloadStarterArchiveRejectsOversizeAndRemovesTempFile(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("TMPDIR", temporary)
	previousLimit := starterMaxArchiveBytes
	starterMaxArchiveBytes = 64
	t.Cleanup(func() { starterMaxArchiveBytes = previousLimit })
	useStarterServer(t, map[string][]byte{"main": bytes.Repeat([]byte("x"), 65)}, nil)
	_, err := downloadStarterArchive("main")
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized archive error = %v", err)
	}
	entries, err := os.ReadDir(temporary)
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary archive was left behind: %v, %v", entries, err)
	}
}

func TestExtractStarterModuleEnforcesUncompressedLimits(t *testing.T) {
	for _, test := range []struct {
		name       string
		files      map[string]string
		byteLimit  int64
		entryLimit int
		want       string
	}{
		{
			name:       "bytes",
			files:      map[string]string{"hyperbricks-main/modules/hello-world/large.txt": "12345"},
			byteLimit:  4,
			entryLimit: 10000,
			want:       "extracted bytes",
		},
		{
			name: "entries",
			files: map[string]string{
				"hyperbricks-main/modules/hello-world/a.txt": "a",
				"hyperbricks-main/modules/hello-world/b.txt": "b",
			},
			byteLimit:  128 << 20,
			entryLimit: 1,
			want:       "archive entries",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			previousBytes, previousEntries := starterMaxModuleBytes, starterMaxModuleEntries
			starterMaxModuleBytes, starterMaxModuleEntries = test.byteLimit, test.entryLimit
			t.Cleanup(func() {
				starterMaxModuleBytes, starterMaxModuleEntries = previousBytes, previousEntries
			})
			archivePath := filepath.Join(t.TempDir(), "source.zip")
			if err := os.WriteFile(archivePath, makeStarterZip(t, test.files), 0644); err != nil {
				t.Fatalf("write starter archive: %v", err)
			}
			err := extractZipSubdirArchive(archivePath, t.TempDir(), "hyperbricks-main/modules/hello-world")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("extraction limit error = %v", err)
			}
		})
	}
}

func starterIndexFixture() map[string]StarterMeta {
	return map[string]StarterMeta{
		"hello-world": {
			Name:                  "hello-world",
			Path:                  "modules/hello-world",
			Entrypoint:            "package.hyperbricks.yaml",
			Description:           "Minimal starter",
			CompatibleHyperbricks: []string{">=0.8.0-alpha"},
		},
	}
}

func starterArchiveFixture(t *testing.T, root string, index map[string]StarterMeta, packageConfig string) []byte {
	t.Helper()
	indexData, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("encode index: %v", err)
	}
	return makeStarterZip(t, map[string]string{
		root + "/starters.index.json":                                          string(indexData),
		root + "/modules/hello-world/package.hyperbricks.yaml":                 packageConfig,
		root + "/modules/hello-world/hyperbricks/hello-world.hyperbricks.yaml": testStarterYAMLSource,
		root + "/modules/hello-world/templates/.gitkeep":                       "",
		root + "/modules/hello-world/static/.gitkeep":                          "",
		root + "/modules/hello-world/resources/.gitkeep":                       "",
		root + "/modules/hello-world/rendered/.gitkeep":                        "",
		root + "/modules/hello-world/logs/.gitkeep":                            "",
		root + "/modules/other-starter/package.hyperbricks.yaml":               "ignored: true\n",
		root + "/README.md": "ignored\n",
	})
}

func useStarterWorkingDirectory(t *testing.T) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("change to test working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
}

func useStarterServer(t *testing.T, archives map[string][]byte, indexes map[string][]byte) func() []string {
	t.Helper()
	var mu sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.URL.RequestURI())
		mu.Unlock()
		for ref, body := range archives {
			if r.URL.Path == "/archive/"+starterArchiveRefPath(ref)+".zip" {
				w.Header().Set("Content-Type", "application/zip")
				_, _ = w.Write(body)
				return
			}
		}
		for ref, body := range indexes {
			if r.URL.Path == "/contents/starters.index.json" && r.URL.Query().Get("ref") == starterArchiveRefPath(ref) {
				if r.Header.Get("Accept") != "application/vnd.github.raw+json" {
					http.Error(w, "missing raw content Accept header", http.StatusNotAcceptable)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(body)
				return
			}
		}
		http.NotFound(w, r)
	}))
	previousIndexURL, previousArchiveURL := starterIndexURL, starterArchiveURL
	starterIndexURL = server.URL + "/contents/starters.index.json"
	starterArchiveURL = server.URL + "/archive/%s.zip"
	t.Cleanup(func() {
		starterIndexURL, starterArchiveURL = previousIndexURL, previousArchiveURL
		server.Close()
	})
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), requests...)
	}
}

func assertNoStarterStagingDirectories(t *testing.T, moduleName string) {
	t.Helper()
	entries, err := os.ReadDir("modules")
	if err != nil {
		t.Fatalf("read modules directory: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "."+moduleName+"-starter-stage-") {
			t.Fatalf("starter staging directory was not removed: %s", entry.Name())
		}
	}
}

func makeStarterZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, contents := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := file.Write([]byte(contents)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return buffer.Bytes()
}
