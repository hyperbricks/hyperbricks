package yamlparser

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v4"
)

// ConfigOrigin identifies the winning source definition, before resolution.
// Path uses source mapping keys (and sequence indices), not resolved values.
type ConfigOrigin struct {
	File   string
	Path   []string
	Line   int
	Column int
}

// ConfigSource retains a source document for targeted edits and concurrency checks.
type ConfigSource struct {
	Path          string
	CanonicalPath string
	Bytes         []byte
	Document      *yaml.Node
}

// ProcessPackageConfigFile explicitly enables package composition. Ordinary
// ProcessConfigFile callers retain their single-document semantics.
func ProcessPackageConfigFile(path, module string, opts Options) (*ConfigResult, error) {
	read := opts.ResourceReadFile
	if read == nil {
		read = os.ReadFile
	}
	raw, err := read(path)
	if err != nil {
		return nil, fmt.Errorf("package %s: %w", path, err)
	}
	return ProcessPackageConfigBytes(raw, path, module, opts)
}

// ProcessPackageConfigBytes composes an unsaved entry with imports read through
// the caller's controlled reader. The entry origin is necessary for profiles.
func ProcessPackageConfigBytes(raw []byte, origin, module string, opts Options) (*ConfigResult, error) {
	root, err := filepath.Abs(module)
	if err != nil {
		return nil, err
	}
	root, err = packageCanonicalPath(root)
	if err != nil {
		return nil, err
	}
	entry, err := filepath.Abs(origin)
	if err != nil {
		return nil, err
	}
	loader := packageLoader{root: root, opts: opts, sources: map[string]ConfigSource{}, origins: map[*yaml.Node]ConfigOrigin{}, active: map[string]bool{}}
	merged, err := loader.load(entry, raw, true, nil)
	if err != nil {
		return nil, err
	}
	effective, err := yaml.Marshal(merged)
	if err != nil {
		return nil, err
	}
	result, err := ProcessConfigBytes(effective, opts)
	if err != nil {
		return nil, err
	}
	result.Sources = loader.sources
	result.Dependencies = loader.order
	result.Unresolved = merged
	result.Origins = map[string]ConfigOrigin{}
	var collect func(*yaml.Node, string)
	collect = func(n *yaml.Node, path string) {
		result.Origins[path] = loader.origins[n]
		switch n.Kind {
		case yaml.MappingNode:
			for i := 0; i < len(n.Content); i += 2 {
				key := n.Content[i].Value
				if path != "" {
					key = path + "." + key
				}
				collect(n.Content[i+1], key)
			}
		case yaml.SequenceNode:
			for i, c := range n.Content {
				collect(c, fmt.Sprintf("%s[%d]", path, i))
			}
		}
	}
	collect(merged, "")
	for i := range result.Diagnostics {
		d := &result.Diagnostics[i]
		if source, ok := result.Origins[d.Path]; ok {
			d.Source, d.Line, d.Column = source.File, source.Line, source.Column
		}
	}
	return result, nil
}

type packageLoader struct {
	root    string
	opts    Options
	sources map[string]ConfigSource
	order   []string
	origins map[*yaml.Node]ConfigOrigin
	active  map[string]bool
}

// Resolve existing ancestors too: an unsaved entry/import can be absent while
// a parent symlink still needs confinement checks.
func packageCanonicalPath(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	parent := filepath.Dir(path)
	if parent == path {
		return "", err
	}
	resolved, err = packageCanonicalPath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved, filepath.Base(path)), nil
}

func (l *packageLoader) load(path string, raw []byte, supplied bool, chain []string) (*yaml.Node, error) {
	path = filepath.Clean(path)
	chain = append(append([]string(nil), chain...), path)
	fail := func(err error) (*yaml.Node, error) {
		return nil, fmt.Errorf("package import chain %s: %w", strings.Join(chain, " -> "), err)
	}
	canonical, err := packageCanonicalPath(path)
	if err != nil {
		return fail(err)
	}
	rel, err := filepath.Rel(l.root, canonical)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fail(fmt.Errorf("path escapes module boundary: %s", path))
	}
	if l.active[canonical] {
		return fail(fmt.Errorf("circular package import: %s", path))
	}
	l.active[canonical] = true
	defer delete(l.active, canonical)
	if !supplied {
		read := l.opts.ResourceReadFile
		if read == nil {
			read = os.ReadFile
		}
		raw, err = read(path)
		if err != nil {
			return fail(err)
		}
	}
	processed, err := PreprocessBytes(raw, l.opts)
	if err != nil {
		return fail(err)
	}
	var doc yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(processed))
	if err = decoder.Decode(&doc); err != nil {
		return fail(err)
	}
	var extra yaml.Node
	if err = decoder.Decode(&extra); err != io.EOF {
		return fail(fmt.Errorf("expected exactly one YAML document"))
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return fail(fmt.Errorf("package root must be a mapping"))
	}
	body := doc.Content[0]
	// Validate every document before merge; overridden duplicate/alias errors
	// must not disappear. vars are checked after composition.
	if _, err = parsePlainGenericMap(body); err != nil {
		return fail(err)
	}
	if _, seen := l.sources[path]; !seen {
		l.order = append(l.order, path)
	}
	l.sources[path] = ConfigSource{Path: path, CanonicalPath: canonical, Bytes: append([]byte(nil), raw...), Document: &doc}
	var mark func(*yaml.Node, []string)
	mark = func(n *yaml.Node, p []string) {
		l.origins[n] = ConfigOrigin{File: path, Path: append([]string(nil), p...), Line: n.Line, Column: n.Column}
		if n.Kind == yaml.MappingNode {
			for i := 0; i < len(n.Content); i += 2 {
				mark(n.Content[i+1], append(append([]string(nil), p...), n.Content[i].Value))
			}
		} else if n.Kind == yaml.SequenceNode {
			for i, c := range n.Content {
				mark(c, append(append([]string(nil), p...), strconv.Itoa(i)))
			}
		}
	}
	mark(body, nil)
	merged := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	own := *body
	own.Content = nil
	for i := 0; i < len(body.Content); i += 2 {
		key, value := body.Content[i], body.Content[i+1]
		if key.Value != "imports" {
			own.Content = append(own.Content, key, value)
			continue
		}
		if value.Kind != yaml.SequenceNode {
			return fail(nodeError(value, "imports must be a list of literal relative filenames"))
		}
		for _, item := range value.Content {
			if item.Kind != yaml.ScalarNode || item.Tag != "!!str" || strings.TrimSpace(item.Value) == "" || filepath.IsAbs(item.Value) || strings.Contains(item.Value, "://") || strings.ContainsAny(item.Value, "*?[]") {
				return fail(nodeError(item, "import must be a literal relative filename"))
			}
			imported, err := l.load(filepath.Join(filepath.Dir(path), item.Value), nil, false, chain)
			if err != nil {
				return nil, err
			}
			merged = l.merge(merged, imported)
		}
	}
	l.origins[&own] = l.origins[body]
	return l.merge(merged, &own), nil
}

func (l *packageLoader) merge(earlier, later *yaml.Node) *yaml.Node {
	if earlier.Kind != yaml.MappingNode || later.Kind != yaml.MappingNode || configResolverNode(earlier) || configResolverNode(later) {
		return later
	}
	merged := *earlier
	merged.Content = append([]*yaml.Node(nil), earlier.Content...)
	l.origins[&merged] = l.origins[later]
	for i := 0; i < len(later.Content); i += 2 {
		key, value := later.Content[i], later.Content[i+1]
		found := false
		for j := 0; j < len(merged.Content); j += 2 {
			if merged.Content[j].Value == key.Value {
				merged.Content[j] = key
				merged.Content[j+1] = l.merge(merged.Content[j+1], value)
				found = true
				break
			}
		}
		if !found {
			merged.Content = append(merged.Content, key, value)
		}
	}
	return &merged
}

func configResolverNode(n *yaml.Node) bool {
	if n.Kind != yaml.MappingNode {
		return false
	}
	values, err := parsePlainGenericMap(n)
	return err == nil && isValueResolver(values)
}
