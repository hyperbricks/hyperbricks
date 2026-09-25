package packagemetadata

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v4"
)

func TestReconcileSourceCreatesStableMetadata(t *testing.T) {
	input := []byte("# package comment\ntitle: Demo\n")
	result, err := ReconcileSource(input, ReconcileOptions{
		Module:             "demo",
		HyperBricks:        "v1.2.5-beta",
		ResetModuleVersion: true,
	})
	if err != nil {
		t.Fatalf("ReconcileSource() error = %v", err)
	}
	if !result.Changed {
		t.Fatal("ReconcileSource() Changed = false, want true")
	}
	wantMetadata := SourceMetadata{Module: "demo", ModuleVersion: "1.0.0", HyperBricks: "v1.2.5-beta"}
	if result.Metadata != wantMetadata {
		t.Fatalf("metadata = %#v, want %#v", result.Metadata, wantMetadata)
	}
	wantFields := []string{"module", "moduleversion", "hyperbricks"}
	if got := changeFields(result.Changes); !reflect.DeepEqual(got, wantFields) {
		t.Fatalf("change fields = %#v, want %#v", got, wantFields)
	}
	if !bytes.Contains(result.Content, []byte("# package comment")) || !bytes.Contains(result.Content, []byte("title: Demo")) {
		t.Fatalf("unrelated content or comment was not preserved:\n%s", result.Content)
	}
	metadata := decodedMetadata(t, result.Content)
	assertScalar(t, metadata, "module", "demo")
	assertScalar(t, metadata, "moduleversion", "1.0.0")
	assertScalar(t, metadata, "hyperbricks", "v1.2.5-beta")
}

func TestReconcileSourceCreatesMissingMetadataInsideExistingHyperbricks(t *testing.T) {
	input := []byte("hyperbricks:\n  mode: live\n")
	result, err := ReconcileSource(input, ReconcileOptions{
		Module:             "demo",
		HyperBricks:        "v1.2.5-beta",
		ResetModuleVersion: true,
	})
	if err != nil {
		t.Fatalf("ReconcileSource() error = %v", err)
	}
	metadata := decodedMetadata(t, result.Content)
	assertScalar(t, metadata, "moduleversion", DefaultModuleVersion)
	if !bytes.Contains(result.Content, []byte("mode: live")) {
		t.Fatalf("existing HyperBricks configuration was lost:\n%s", result.Content)
	}
}

func TestReconcileSourceUpdatesAndRemovesArtifactFields(t *testing.T) {
	input := []byte(`# root comment
hyperbricks:
  metadata:
    # module comment
    module: default
    moduleversion: "1.0"
    format: hra
    format_version: "1"
    commit: unknown
    built_at: "1970-01-01T00:00:00Z"
    source_hash: stale
    hyperbricks: v0.0.0
    owner: team
  mode: live
other: keep
`)
	result, err := ReconcileSource(input, ReconcileOptions{
		Module:                    "demo",
		HyperBricks:               "v1.2.5-beta",
		CanonicalizeModuleVersion: true,
	})
	if err != nil {
		t.Fatalf("ReconcileSource() error = %v", err)
	}
	wantFields := []string{"module", "moduleversion", "hyperbricks", "format", "format_version", "commit", "built_at", "source_hash"}
	if got := changeFields(result.Changes); !reflect.DeepEqual(got, wantFields) {
		t.Fatalf("change fields = %#v, want %#v", got, wantFields)
	}
	for _, change := range result.Changes[3:] {
		if !change.Removed {
			t.Fatalf("change %#v should be marked removed", change)
		}
	}
	if !bytes.Contains(result.Content, []byte("# root comment")) || !bytes.Contains(result.Content, []byte("# module comment")) {
		t.Fatalf("comments were not preserved:\n%s", result.Content)
	}
	metadata := decodedMetadata(t, result.Content)
	assertScalar(t, metadata, "module", "demo")
	assertScalar(t, metadata, "moduleversion", "1.0.0")
	assertScalar(t, metadata, "hyperbricks", "v1.2.5-beta")
	assertScalar(t, metadata, "owner", "team")
	for _, field := range artifactOnlyFields {
		if _, found, err := mappingField(metadata, field); err != nil || found {
			t.Fatalf("artifact field %q still exists (found=%v, err=%v)", field, found, err)
		}
	}
	if got := mappingKeys(metadata); !reflect.DeepEqual(got, []string{"module", "moduleversion", "hyperbricks", "owner"}) {
		t.Fatalf("metadata keys = %#v", got)
	}
	if !bytes.Contains(result.Content, []byte("mode: live")) || !bytes.Contains(result.Content, []byte("other: keep")) {
		t.Fatalf("unrelated YAML was not preserved:\n%s", result.Content)
	}
}

func TestReconcileSourceSemanticNoOpReturnsOriginalBytes(t *testing.T) {
	input := []byte("hyperbricks:\n    metadata:\n        module: demo\n        moduleversion: \"1.0.0\"\n        hyperbricks: v1.2.5-beta\n")
	result, err := ReconcileSource(input, ReconcileOptions{
		Module:                    "demo",
		HyperBricks:               "v1.2.5-beta",
		CanonicalizeModuleVersion: true,
	})
	if err != nil {
		t.Fatalf("ReconcileSource() error = %v", err)
	}
	if result.Changed || len(result.Changes) != 0 {
		t.Fatalf("no-op result = %#v", result)
	}
	if !bytes.Equal(result.Content, input) {
		t.Fatalf("no-op changed bytes:\n%s", result.Content)
	}
}

func TestReconcileSourceDetectsWhitespaceNormalization(t *testing.T) {
	input := []byte("hyperbricks:\n  metadata:\n    module: \" demo \"\n    moduleversion: \"1.0.0\"\n    hyperbricks: v1.2.5-beta\n")
	result, err := ReconcileSource(input, ReconcileOptions{Module: "demo", HyperBricks: "v1.2.5-beta"})
	if err != nil {
		t.Fatalf("ReconcileSource() error = %v", err)
	}
	if !result.Changed {
		t.Fatal("whitespace normalization was not detected")
	}
	assertScalar(t, decodedMetadata(t, result.Content), "module", "demo")
}

func TestReconcileSourcePreservesExistingScalarStyle(t *testing.T) {
	input := []byte("hyperbricks:\n  metadata:\n    module: \"old\"\n    moduleversion: '1.0'\n    hyperbricks: \"v0.0.0\"\n")
	result, err := ReconcileSource(input, ReconcileOptions{
		Module:                    "demo",
		HyperBricks:               "v1.2.5-beta",
		CanonicalizeModuleVersion: true,
	})
	if err != nil {
		t.Fatalf("ReconcileSource() error = %v", err)
	}
	output := string(result.Content)
	for _, want := range []string{`module: "demo"`, `moduleversion: '1.0.0'`, `hyperbricks: "v1.2.5-beta"`} {
		if !strings.Contains(output, want) {
			t.Fatalf("output did not preserve scalar style %q:\n%s", want, output)
		}
	}
}

func TestReconcileSourceBumpsVersions(t *testing.T) {
	tests := []struct {
		name    string
		current string
		bump    Bump
		want    string
	}{
		{name: "patch", current: "1.2.3", bump: BumpPatch, want: "1.2.4"},
		{name: "minor", current: "1.2.3", bump: BumpMinor, want: "1.3.0"},
		{name: "major", current: "1.2.3", bump: BumpMajor, want: "2.0.0"},
		{name: "loose patch", current: "1.0", bump: BumpPatch, want: "1.0.1"},
		{name: "prerelease patch", current: "1.2.3-beta.2", bump: BumpPatch, want: "1.2.4"},
		{name: "prerelease minor", current: "1.2.3-beta.2+build.4", bump: BumpMinor, want: "1.3.0"},
		{name: "prerelease major", current: "1.2.3-beta.2+build.4", bump: BumpMajor, want: "2.0.0"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := packageYAML(test.current)
			result, err := ReconcileSource(input, ReconcileOptions{
				Module:      "demo",
				HyperBricks: "v1.2.5-beta",
				Bump:        test.bump,
			})
			if err != nil {
				t.Fatalf("ReconcileSource() error = %v", err)
			}
			if result.Metadata.ModuleVersion != test.want {
				t.Fatalf("version = %q, want %q", result.Metadata.ModuleVersion, test.want)
			}
		})
	}
}

func TestReconcileSourceRejectsInvalidOperationsWithoutContent(t *testing.T) {
	valid := packageYAML("1.2.3")
	tests := []struct {
		name string
		data []byte
		opts ReconcileOptions
	}{
		{
			name: "invalid version",
			data: packageYAML("not-semver"),
			opts: ReconcileOptions{Module: "demo", HyperBricks: "v1", CanonicalizeModuleVersion: true},
		},
		{
			name: "unsupported bump",
			data: valid,
			opts: ReconcileOptions{Module: "demo", HyperBricks: "v1", Bump: Bump("build")},
		},
		{
			name: "reset and bump",
			data: valid,
			opts: ReconcileOptions{Module: "demo", HyperBricks: "v1", ResetModuleVersion: true, Bump: BumpPatch},
		},
		{
			name: "missing version",
			data: []byte("hyperbricks:\n  metadata:\n    module: demo\n"),
			opts: ReconcileOptions{Module: "demo", HyperBricks: "v1"},
		},
		{
			name: "wrong hyperbricks type",
			data: []byte("hyperbricks: invalid\n"),
			opts: ReconcileOptions{Module: "demo", HyperBricks: "v1", ResetModuleVersion: true},
		},
		{
			name: "wrong metadata type",
			data: []byte("hyperbricks:\n  metadata: invalid\n"),
			opts: ReconcileOptions{Module: "demo", HyperBricks: "v1", ResetModuleVersion: true},
		},
		{
			name: "duplicate known field",
			data: []byte("hyperbricks:\n  metadata:\n    module: one\n    module: two\n    moduleversion: \"1.0.0\"\n"),
			opts: ReconcileOptions{Module: "demo", HyperBricks: "v1"},
		},
		{
			name: "multiple documents",
			data: []byte("hyperbricks: {}\n---\nhyperbricks: {}\n"),
			opts: ReconcileOptions{Module: "demo", HyperBricks: "v1", ResetModuleVersion: true},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if result, err := ReconcileSource(test.data, test.opts); err == nil {
				t.Fatalf("ReconcileSource() = %#v, want error", result)
			}
		})
	}
}

func TestReconcileSourceFileIsAtomicPreservesModeAndSkipsNoOp(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "package.hyperbricks.yaml")
	if err := os.WriteFile(path, packageYAML("1.0"), 0o640); err != nil {
		t.Fatal(err)
	}

	result, err := ReconcileSourceFile(path, ReconcileOptions{
		Module:                    "demo",
		HyperBricks:               "v1.2.5-beta",
		CanonicalizeModuleVersion: true,
	})
	if err != nil {
		t.Fatalf("ReconcileSourceFile() error = %v", err)
	}
	if !result.Changed {
		t.Fatal("ReconcileSourceFile() Changed = false, want true")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Fatalf("mode = %v, want 0640", got)
	}

	oldTime := time.Unix(1_600_000_000, 0)
	if err := os.Chtimes(path, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	noOp, err := ReconcileSourceFile(path, ReconcileOptions{
		Module:                    "demo",
		HyperBricks:               "v1.2.5-beta",
		CanonicalizeModuleVersion: true,
	})
	if err != nil {
		t.Fatalf("no-op ReconcileSourceFile() error = %v", err)
	}
	if noOp.Changed {
		t.Fatal("second ReconcileSourceFile() rewrote a semantic no-op")
	}
	info, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(oldTime) {
		t.Fatalf("no-op modtime = %v, want %v", info.ModTime(), oldTime)
	}
	if matches, err := filepath.Glob(filepath.Join(directory, ".package.hyperbricks.yaml.metadata-*")); err != nil || len(matches) != 0 {
		t.Fatalf("temporary files left behind: %#v, err=%v", matches, err)
	}
}

func TestReconcileSourceFileFailureLeavesOriginalUntouched(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "package.hyperbricks.yaml")
	original := []byte("hyperbricks:\n  metadata:\n    moduleversion: invalid\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ReconcileSourceFile(path, ReconcileOptions{
		Module:                    "demo",
		HyperBricks:               "v1.2.5-beta",
		CanonicalizeModuleVersion: true,
	})
	if err == nil {
		t.Fatal("ReconcileSourceFile() error = nil")
	}
	current, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(current, original) {
		t.Fatalf("failed update changed source:\n%s", current)
	}
}

func TestRenderArtifactOverlaysProvenanceWithoutMutatingSource(t *testing.T) {
	source := []byte(`# source
hyperbricks:
  metadata:
    owner: team
    hyperbricks: v0.0.0
    moduleversion: "1.0"
    module: stale
    source_hash: stale
  mode: live
`)
	original := append([]byte(nil), source...)
	result, err := RenderArtifact(source, ArtifactOptions{
		Module:        "demo",
		Format:        "hra",
		FormatVersion: "1",
		Commit:        "abcdef1",
		BuiltAt:       "2026-09-24T12:34:56Z",
		HyperBricks:   "v1.2.5-beta",
	})
	if err != nil {
		t.Fatalf("RenderArtifact() error = %v", err)
	}
	if !bytes.Equal(source, original) {
		t.Fatal("RenderArtifact() mutated source bytes")
	}
	if result.Metadata.ModuleVersion != "1.0" {
		t.Fatalf("module version = %q, want source value 1.0", result.Metadata.ModuleVersion)
	}
	metadata := decodedMetadata(t, result.Content)
	wantKeys := []string{"module", "moduleversion", "format", "format_version", "commit", "built_at", "hyperbricks", "owner"}
	if got := mappingKeys(metadata); !reflect.DeepEqual(got, wantKeys) {
		t.Fatalf("metadata keys = %#v, want %#v", got, wantKeys)
	}
	for field, want := range map[string]string{
		"module":         "demo",
		"moduleversion":  "1.0",
		"format":         "hra",
		"format_version": "1",
		"commit":         "abcdef1",
		"built_at":       "2026-09-24T12:34:56Z",
		"hyperbricks":    "v1.2.5-beta",
		"owner":          "team",
	} {
		assertScalar(t, metadata, field, want)
	}
	if _, found, err := mappingField(metadata, "source_hash"); err != nil || found {
		t.Fatalf("legacy source_hash survived artifact render (found=%v, err=%v)", found, err)
	}
	if !bytes.Contains(result.Content, []byte("# source")) || !bytes.Contains(result.Content, []byte("mode: live")) {
		t.Fatalf("unrelated artifact YAML was not preserved:\n%s", result.Content)
	}
}

func TestRenderArtifactRequiresValidSourceMetadataAndOptions(t *testing.T) {
	validOptions := ArtifactOptions{
		Module: "demo", Format: "hra", FormatVersion: "1", Commit: "unknown",
		BuiltAt: "2026-09-24T12:34:56Z", HyperBricks: "v1.2.5-beta",
	}
	tests := []struct {
		name string
		data []byte
		opts ArtifactOptions
	}{
		{name: "missing hyperbricks", data: []byte("other: true\n"), opts: validOptions},
		{name: "missing version", data: []byte("hyperbricks:\n  metadata: {}\n"), opts: validOptions},
		{name: "invalid version", data: packageYAML("invalid"), opts: validOptions},
		{name: "empty format", data: packageYAML("1.0.0"), opts: func() ArtifactOptions { o := validOptions; o.Format = ""; return o }()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if result, err := RenderArtifact(test.data, test.opts); err == nil {
				t.Fatalf("RenderArtifact() = %#v, want error", result)
			}
		})
	}
}

func TestRenderArtifactTracksDuplicateOriginOnlyInDerivedArchive(t *testing.T) {
	source := []byte("hyperbricks:\n  metadata:\n    module: demo\n    moduleversion: \"1.0.0\"\n    origin_build_id: stale\n")
	options := ArtifactOptions{
		Module: "demo", Format: "hra", FormatVersion: "1", Commit: "source7",
		BuiltAt: "2026-09-24T12:34:56.123Z", HyperBricks: "v1.2.5-beta",
		OriginBuildID: "original-id",
	}
	derived, err := RenderArtifact(source, options)
	if err != nil {
		t.Fatal(err)
	}
	if derived.Metadata.OriginBuildID != "original-id" {
		t.Fatalf("origin metadata = %q", derived.Metadata.OriginBuildID)
	}
	assertScalar(t, decodedMetadata(t, derived.Content), "origin_build_id", "original-id")
	options.OriginBuildID = ""
	ordinary, err := RenderArtifact(source, options)
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := mappingField(decodedMetadata(t, ordinary.Content), "origin_build_id"); err != nil || found {
		t.Fatalf("ordinary build retained stale origin (found=%v, err=%v)", found, err)
	}
}

func TestGitShortCommitIsScopedToRoot(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	repository := t.TempDir()
	runGit(t, repository, "init", "--quiet")
	runGit(t, repository, "config", "user.name", "HyperBricks Test")
	runGit(t, repository, "config", "user.email", "test@hyperbricks.dev")
	if err := os.WriteFile(filepath.Join(repository, "file.txt"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "add", "file.txt")
	runGit(t, repository, "commit", "--quiet", "-m", "test")
	want := strings.TrimSpace(runGit(t, repository, "rev-parse", "--short=7", "HEAD"))
	otherRepository := t.TempDir()
	runGit(t, otherRepository, "init", "--quiet")
	t.Chdir(otherRepository)
	nested := filepath.Join(repository, "module")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := GitShortCommit(nested); got != want {
		t.Fatalf("GitShortCommit(%q) = %q, want %q", nested, got, want)
	}
	if got := GitShortCommit(filepath.Join(repository, "missing")); got != UnknownCommit {
		t.Fatalf("GitShortCommit(missing) = %q, want %q", got, UnknownCommit)
	}
	if got := GitShortCommit(""); got != UnknownCommit {
		t.Fatalf("GitShortCommit(empty) = %q, want %q", got, UnknownCommit)
	}
}

func packageYAML(version string) []byte {
	return []byte("hyperbricks:\n  metadata:\n    module: demo\n    moduleversion: \"" + version + "\"\n    hyperbricks: v1.2.5-beta\n")
}

func decodedMetadata(t *testing.T, content []byte) *yaml.Node {
	t.Helper()
	root, err := parseDocument(content)
	if err != nil {
		t.Fatal(err)
	}
	body := documentBody(root)
	hyperbricks, found, err := mappingField(body, "hyperbricks")
	if err != nil || !found {
		t.Fatalf("hyperbricks mapping: found=%v, err=%v", found, err)
	}
	metadata, found, err := mappingField(hyperbricks, "metadata")
	if err != nil || !found {
		t.Fatalf("metadata mapping: found=%v, err=%v", found, err)
	}
	return metadata
}

func assertScalar(t *testing.T, mapping *yaml.Node, field, want string) {
	t.Helper()
	node, found, err := mappingField(mapping, field)
	if err != nil || !found {
		t.Fatalf("field %q: found=%v, err=%v", field, found, err)
	}
	if node.Kind != yaml.ScalarNode || node.Value != want {
		t.Fatalf("field %q = kind %v value %q, want scalar %q", field, node.Kind, node.Value, want)
	}
}

func changeFields(changes []Change) []string {
	fields := make([]string, 0, len(changes))
	for _, change := range changes {
		fields = append(fields, change.Field)
	}
	return fields
}

func mappingKeys(mapping *yaml.Node) []string {
	keys := make([]string, 0, len(mapping.Content)/2)
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		keys = append(keys, mapping.Content[index].Value)
	}
	return keys
}

func runGit(t *testing.T, directory string, args ...string) string {
	t.Helper()
	commandArgs := append([]string{"-C", directory}, args...)
	output, err := exec.Command("git", commandArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(commandArgs, " "), err, output)
	}
	return string(output)
}
