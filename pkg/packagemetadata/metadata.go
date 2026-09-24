// Package packagemetadata owns the lifecycle of metadata in
// package.hyperbricks.yaml files.
//
// Source packages contain stable module identity. Build artifacts add immutable
// build provenance in memory without changing the source package.
package packagemetadata

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	"go.yaml.in/yaml/v4"
)

const (
	// DefaultModuleVersion is assigned to newly initialized modules.
	DefaultModuleVersion = "1.0.0"
	// UnknownCommit is returned when a Git commit cannot be resolved.
	UnknownCommit = "unknown"
)

// Bump identifies a semantic-version increment.
type Bump string

const (
	BumpNone  Bump = ""
	BumpPatch Bump = "patch"
	BumpMinor Bump = "minor"
	BumpMajor Bump = "major"
)

var sourceFieldOrder = []string{
	"module",
	"moduleversion",
	"hyperbricks",
}

var artifactOnlyFields = []string{
	"format",
	"format_version",
	"commit",
	"built_at",
	"source_hash",
}

var artifactFieldOrder = []string{
	"module",
	"moduleversion",
	"format",
	"format_version",
	"commit",
	"built_at",
	"hyperbricks",
}

// SourceMetadata is the stable metadata stored in a source package.
type SourceMetadata struct {
	Module        string
	ModuleVersion string
	HyperBricks   string
}

// ArtifactMetadata is the immutable metadata rendered into an archive.
type ArtifactMetadata struct {
	SourceMetadata
	Format        string
	FormatVersion string
	Commit        string
	BuiltAt       string
}

// ReconcileOptions controls a source metadata update.
type ReconcileOptions struct {
	Module                    string
	HyperBricks               string
	ResetModuleVersion        bool
	CanonicalizeModuleVersion bool
	Bump                      Bump
}

// ArtifactOptions contains the provenance supplied by a build.
type ArtifactOptions struct {
	Module        string
	Format        string
	FormatVersion string
	Commit        string
	BuiltAt       string
	HyperBricks   string
}

// Change describes one deterministic source metadata change. Removed is true
// for artifact-only fields removed from the source package.
type Change struct {
	Field   string
	Before  string
	After   string
	Removed bool
}

// ReconcileResult is returned by both in-memory and file reconciliation.
type ReconcileResult struct {
	Content  []byte
	Before   SourceMetadata
	Metadata SourceMetadata
	Changes  []Change
	Changed  bool
}

// ArtifactResult is the rendered package document and its effective metadata.
type ArtifactResult struct {
	Content  []byte
	Metadata ArtifactMetadata
}

// ReconcileSource updates stable metadata in a package document. It creates
// hyperbricks.metadata when absent, removes artifact-only fields, and leaves
// unrelated YAML nodes intact.
func ReconcileSource(content []byte, opts ReconcileOptions) (ReconcileResult, error) {
	if strings.TrimSpace(opts.Module) == "" {
		return ReconcileResult{}, errors.New("module cannot be empty")
	}
	if strings.TrimSpace(opts.HyperBricks) == "" {
		return ReconcileResult{}, errors.New("HyperBricks version cannot be empty")
	}
	if opts.ResetModuleVersion && opts.Bump != BumpNone {
		return ReconcileResult{}, errors.New("module version cannot be reset and bumped in the same operation")
	}
	if err := validateBump(opts.Bump); err != nil {
		return ReconcileResult{}, err
	}

	root, err := parseDocument(content)
	if err != nil {
		return ReconcileResult{}, err
	}
	body := documentBody(root)
	if body == nil || body.Kind != yaml.MappingNode {
		return ReconcileResult{}, errors.New("package document must be a YAML mapping")
	}

	hyperbricks, err := ensureMapping(body, "hyperbricks", "package")
	if err != nil {
		return ReconcileResult{}, err
	}
	metadata, err := ensureMapping(hyperbricks, "metadata", "hyperbricks")
	if err != nil {
		return ReconcileResult{}, err
	}
	if err := rejectDuplicateFields(metadata, append(append([]string{}, sourceFieldOrder...), artifactOnlyFields...)); err != nil {
		return ReconcileResult{}, fmt.Errorf("hyperbricks.metadata: %w", err)
	}

	before, states, err := inspectSourceMetadata(metadata)
	if err != nil {
		return ReconcileResult{}, err
	}

	moduleVersion, err := reconcileModuleVersion(states["moduleversion"], opts)
	if err != nil {
		return ReconcileResult{}, err
	}
	after := SourceMetadata{
		Module:        strings.TrimSpace(opts.Module),
		ModuleVersion: moduleVersion,
		HyperBricks:   strings.TrimSpace(opts.HyperBricks),
	}

	setMappingString(metadata, "module", after.Module, 0)
	setMappingString(metadata, "moduleversion", after.ModuleVersion, yaml.DoubleQuotedStyle)
	setMappingString(metadata, "hyperbricks", after.HyperBricks, 0)
	for _, field := range artifactOnlyFields {
		removeMappingField(metadata, field)
	}

	changes := sourceChanges(states, after)
	result := ReconcileResult{
		Before:   before,
		Metadata: after,
		Changes:  changes,
		Changed:  len(changes) > 0,
	}
	if !result.Changed {
		result.Content = append([]byte(nil), content...)
		return result, nil
	}

	reorderMappingFields(metadata, sourceFieldOrder)
	result.Content, err = encodeDocument(root)
	if err != nil {
		return ReconcileResult{}, err
	}
	return result, nil
}

// ReconcileSourceFile applies ReconcileSource atomically. A semantic no-op does
// not rewrite the file. Existing file permissions are preserved.
func ReconcileSourceFile(path string, opts ReconcileOptions) (ReconcileResult, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return ReconcileResult{}, fmt.Errorf("read package metadata %s: %w", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return ReconcileResult{}, fmt.Errorf("inspect package metadata %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return ReconcileResult{}, fmt.Errorf("package metadata is not a regular file: %s", path)
	}

	result, err := ReconcileSource(content, opts)
	if err != nil {
		return ReconcileResult{}, fmt.Errorf("reconcile package metadata %s: %w", path, err)
	}
	if !result.Changed {
		return result, nil
	}
	if err := atomicWriteFile(path, result.Content, info.Mode()); err != nil {
		return ReconcileResult{}, fmt.Errorf("write package metadata %s: %w", path, err)
	}
	return result, nil
}

// RenderArtifact overlays build provenance into an in-memory package document.
// It never writes the source package.
func RenderArtifact(content []byte, opts ArtifactOptions) (ArtifactResult, error) {
	required := []struct {
		name  string
		value string
	}{
		{"module", opts.Module},
		{"format", opts.Format},
		{"format version", opts.FormatVersion},
		{"commit", opts.Commit},
		{"build timestamp", opts.BuiltAt},
		{"HyperBricks version", opts.HyperBricks},
	}
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			return ArtifactResult{}, fmt.Errorf("%s cannot be empty", field.name)
		}
	}

	root, err := parseDocument(content)
	if err != nil {
		return ArtifactResult{}, err
	}
	body := documentBody(root)
	if body == nil || body.Kind != yaml.MappingNode {
		return ArtifactResult{}, errors.New("package document must be a YAML mapping")
	}
	hyperbricks, err := requireMapping(body, "hyperbricks", "package")
	if err != nil {
		return ArtifactResult{}, err
	}
	metadata, err := requireMapping(hyperbricks, "metadata", "hyperbricks")
	if err != nil {
		return ArtifactResult{}, err
	}
	if err := rejectDuplicateFields(metadata, append(append([]string{}, artifactFieldOrder...), "source_hash")); err != nil {
		return ArtifactResult{}, fmt.Errorf("hyperbricks.metadata: %w", err)
	}

	moduleVersionNode, found, err := mappingField(metadata, "moduleversion")
	if err != nil {
		return ArtifactResult{}, fmt.Errorf("hyperbricks.metadata: %w", err)
	}
	if !found || moduleVersionNode.Kind != yaml.ScalarNode || strings.TrimSpace(moduleVersionNode.Value) == "" {
		return ArtifactResult{}, errors.New("missing required field: hyperbricks.metadata.moduleversion")
	}
	moduleVersion := strings.TrimSpace(moduleVersionNode.Value)
	if _, err := semver.NewVersion(moduleVersion); err != nil {
		return ArtifactResult{}, fmt.Errorf("invalid hyperbricks.metadata.moduleversion %q: %w", moduleVersion, err)
	}

	effective := ArtifactMetadata{
		SourceMetadata: SourceMetadata{
			Module:        strings.TrimSpace(opts.Module),
			ModuleVersion: moduleVersion,
			HyperBricks:   strings.TrimSpace(opts.HyperBricks),
		},
		Format:        strings.TrimSpace(opts.Format),
		FormatVersion: strings.TrimSpace(opts.FormatVersion),
		Commit:        strings.TrimSpace(opts.Commit),
		BuiltAt:       strings.TrimSpace(opts.BuiltAt),
	}

	setMappingString(metadata, "module", effective.Module, 0)
	setMappingString(metadata, "format", effective.Format, 0)
	setMappingString(metadata, "format_version", effective.FormatVersion, yaml.DoubleQuotedStyle)
	setMappingString(metadata, "commit", effective.Commit, 0)
	setMappingString(metadata, "built_at", effective.BuiltAt, yaml.DoubleQuotedStyle)
	setMappingString(metadata, "hyperbricks", effective.HyperBricks, 0)
	// source_hash is owned by the build index until the archive format gives it
	// an explicit field. Never carry a stale legacy source value into an artifact.
	removeMappingField(metadata, "source_hash")
	reorderMappingFields(metadata, artifactFieldOrder)

	rendered, err := encodeDocument(root)
	if err != nil {
		return ArtifactResult{}, err
	}
	return ArtifactResult{Content: rendered, Metadata: effective}, nil
}

// GitShortCommit returns the seven-character commit for the worktree containing
// root. Git is always invoked with -C so resolution is independent of the
// process working directory.
func GitShortCommit(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return UnknownCommit
	}
	cmd := exec.Command("git", "-C", root, "rev-parse", "--short=7", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return UnknownCommit
	}
	commit := strings.TrimSpace(string(output))
	if commit == "" {
		return UnknownCommit
	}
	return commit
}

type fieldState struct {
	present bool
	kind    yaml.Kind
	tag     string
	value   string
}

func inspectSourceMetadata(metadata *yaml.Node) (SourceMetadata, map[string]fieldState, error) {
	states := make(map[string]fieldState, len(sourceFieldOrder)+len(artifactOnlyFields))
	for _, field := range append(append([]string{}, sourceFieldOrder...), artifactOnlyFields...) {
		node, found, err := mappingField(metadata, field)
		if err != nil {
			return SourceMetadata{}, nil, fmt.Errorf("hyperbricks.metadata: %w", err)
		}
		state := fieldState{present: found}
		if found {
			state.kind = node.Kind
			state.tag = node.Tag
			state.value = node.Value
			if contains(sourceFieldOrder, field) && node.Kind != yaml.ScalarNode {
				return SourceMetadata{}, nil, fmt.Errorf("hyperbricks.metadata.%s must be a scalar", field)
			}
		}
		states[field] = state
	}
	return SourceMetadata{
		Module:        strings.TrimSpace(states["module"].value),
		ModuleVersion: strings.TrimSpace(states["moduleversion"].value),
		HyperBricks:   strings.TrimSpace(states["hyperbricks"].value),
	}, states, nil
}

func reconcileModuleVersion(current fieldState, opts ReconcileOptions) (string, error) {
	if opts.ResetModuleVersion {
		return DefaultModuleVersion, nil
	}
	value := strings.TrimSpace(current.value)
	if !current.present || value == "" {
		return "", errors.New("missing required field: hyperbricks.metadata.moduleversion")
	}
	version, err := semver.NewVersion(value)
	if err != nil {
		return "", fmt.Errorf("invalid hyperbricks.metadata.moduleversion %q: %w", value, err)
	}
	switch opts.Bump {
	case BumpPatch:
		// A requested patch bump always advances the patch component, even
		// when the source is a prerelease. Masterminds' IncPatch promotes a
		// prerelease to its current release without incrementing, which is not
		// the module lifecycle contract here.
		return fmt.Sprintf("%d.%d.%d", version.Major(), version.Minor(), version.Patch()+1), nil
	case BumpMinor:
		return fmt.Sprintf("%d.%d.0", version.Major(), version.Minor()+1), nil
	case BumpMajor:
		return fmt.Sprintf("%d.0.0", version.Major()+1), nil
	}
	if opts.CanonicalizeModuleVersion {
		return version.String(), nil
	}
	return value, nil
}

func validateBump(bump Bump) error {
	switch bump {
	case BumpNone, BumpPatch, BumpMinor, BumpMajor:
		return nil
	default:
		return fmt.Errorf("unsupported module version bump %q; expected patch, minor, or major", bump)
	}
}

func sourceChanges(before map[string]fieldState, after SourceMetadata) []Change {
	desired := map[string]string{
		"module":        after.Module,
		"moduleversion": after.ModuleVersion,
		"hyperbricks":   after.HyperBricks,
	}
	changes := make([]Change, 0, len(sourceFieldOrder)+len(artifactOnlyFields))
	for _, field := range sourceFieldOrder {
		state := before[field]
		if !state.present || state.kind != yaml.ScalarNode || state.tag != "!!str" || state.value != desired[field] {
			changes = append(changes, Change{Field: field, Before: strings.TrimSpace(state.value), After: desired[field]})
		}
	}
	for _, field := range artifactOnlyFields {
		state := before[field]
		if state.present {
			changes = append(changes, Change{Field: field, Before: strings.TrimSpace(state.value), Removed: true})
		}
	}
	return changes
}

func parseDocument(content []byte) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	var root yaml.Node
	if err := decoder.Decode(&root); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("package document is empty")
		}
		return nil, fmt.Errorf("parse package document: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("package document must contain exactly one YAML document")
		}
		return nil, fmt.Errorf("parse package document: %w", err)
	}
	return &root, nil
}

func encodeDocument(root *yaml.Node) ([]byte, error) {
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(root); err != nil {
		return nil, fmt.Errorf("encode package document: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("encode package document: %w", err)
	}
	return out.Bytes(), nil
}

func documentBody(root *yaml.Node) *yaml.Node {
	if root == nil {
		return nil
	}
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		return root.Content[0]
	}
	return root
}

func ensureMapping(parent *yaml.Node, key, parentPath string) (*yaml.Node, error) {
	value, found, err := mappingField(parent, key)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", parentPath, err)
	}
	if !found {
		value = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		parent.Content = append(parent.Content, stringNode(key, 0), value)
		return value, nil
	}
	if value.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s.%s must be a mapping", parentPath, key)
	}
	return value, nil
}

func requireMapping(parent *yaml.Node, key, parentPath string) (*yaml.Node, error) {
	value, found, err := mappingField(parent, key)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", parentPath, err)
	}
	if !found {
		return nil, fmt.Errorf("missing required object: %s.%s", parentPath, key)
	}
	if value.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s.%s must be a mapping", parentPath, key)
	}
	return value, nil
}

func mappingField(mapping *yaml.Node, key string) (*yaml.Node, bool, error) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil, false, errors.New("expected a YAML mapping")
	}
	var found *yaml.Node
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value != key {
			continue
		}
		if found != nil {
			return nil, false, fmt.Errorf("duplicate field %q", key)
		}
		found = mapping.Content[index+1]
	}
	return found, found != nil, nil
}

func rejectDuplicateFields(mapping *yaml.Node, fields []string) error {
	for _, field := range fields {
		if _, _, err := mappingField(mapping, field); err != nil {
			return err
		}
	}
	return nil
}

func setMappingString(mapping *yaml.Node, key, value string, newStyle yaml.Style) {
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value != key {
			continue
		}
		node := mapping.Content[index+1]
		if node.Kind == yaml.ScalarNode {
			node.Tag = "!!str"
			node.Value = value
			return
		}
		mapping.Content[index+1] = stringNode(value, newStyle)
		return
	}
	mapping.Content = append(mapping.Content, stringNode(key, 0), stringNode(value, newStyle))
}

func removeMappingField(mapping *yaml.Node, key string) {
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value != key {
			continue
		}
		mapping.Content = append(mapping.Content[:index], mapping.Content[index+2:]...)
		return
	}
}

func reorderMappingFields(mapping *yaml.Node, ordered []string) {
	pairs := make(map[string][]*yaml.Node, len(ordered))
	remaining := make([]*yaml.Node, 0, len(mapping.Content))
	wanted := make(map[string]struct{}, len(ordered))
	for _, key := range ordered {
		wanted[key] = struct{}{}
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		keyNode, valueNode := mapping.Content[index], mapping.Content[index+1]
		if _, ok := wanted[keyNode.Value]; ok {
			pairs[keyNode.Value] = []*yaml.Node{keyNode, valueNode}
			continue
		}
		remaining = append(remaining, keyNode, valueNode)
	}
	reordered := make([]*yaml.Node, 0, len(mapping.Content))
	for _, key := range ordered {
		reordered = append(reordered, pairs[key]...)
	}
	mapping.Content = append(reordered, remaining...)
}

func stringNode(value string, style yaml.Style) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value, Style: style}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func atomicWriteFile(path string, content []byte, mode os.FileMode) (err error) {
	directory := filepath.Dir(path)
	temp, err := os.CreateTemp(directory, "."+filepath.Base(path)+".metadata-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		if err != nil {
			_ = os.Remove(tempPath)
		}
	}()

	if err = temp.Chmod(mode); err != nil {
		return err
	}
	if _, err = temp.Write(content); err != nil {
		return err
	}
	if err = temp.Sync(); err != nil {
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	if err = os.Rename(tempPath, path); err != nil {
		return err
	}
	return nil
}
