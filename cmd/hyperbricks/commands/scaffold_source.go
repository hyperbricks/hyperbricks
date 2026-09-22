package commands

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
	"go.yaml.in/yaml/v4"
)

type authoringModule struct {
	Root, ConfigPath string
	Directories      map[string]string
	Config           map[string]interface{}
	PackageBytes     []byte
}

func loadAuthoringModule(selection, config string) (*authoringModule, error) {
	if selection == "" {
		return nil, fmt.Errorf("--module is required")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root, err := resolveDirectStartModuleRoot(selection, cwd)
	if err != nil {
		return nil, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	if config == "" {
		config = PackageConfigFileName
	}
	configPath, err := authoringPath(root, config)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	parsed, err := shared.LoadPackageConfigMap(configPath, root)
	if err != nil {
		return nil, err
	}
	hb, ok := parsed["hyperbricks"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("package requires a hyperbricks mapping")
	}
	dirs, _ := hb["directories"].(map[string]interface{})
	m := &authoringModule{Root: root, ConfigPath: configPath, Config: parsed, PackageBytes: raw, Directories: map[string]string{}}
	for _, key := range []string{"hyperbricks", "templates", "resources", "static"} {
		dir := filepath.Join(root, key)
		if v, exists := dirs[key]; exists {
			var ok bool
			dir, ok = v.(string)
			if !ok || dir == "" {
				return nil, fmt.Errorf("directories.%s must resolve to a path", key)
			}
			if !filepath.IsAbs(dir) {
				dir, err = filepath.Abs(dir)
				if err != nil {
					return nil, err
				}
			}
		}
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			return nil, err
		}
		m.Directories[key], err = authoringPath(root, filepath.ToSlash(rel))
		if err != nil {
			return nil, fmt.Errorf("directories.%s: %w", key, err)
		}
	}
	return m, nil
}

func (m *authoringModule) options() yamlparser.Options {
	return yamlparser.Options{Config: m.Config, TemplateDir: m.Directories["templates"], Paths: yamlparser.PathMarkers{
		Root: ".", ModuleRoot: filepath.Dir(m.Root), Module: m.Root, HyperBricks: m.Directories["hyperbricks"], Templates: m.Directories["templates"], Resources: m.Directories["resources"], Static: m.Directories["static"],
	}}
}

// Source writes intentionally reject symlinks. os.Root also contains each write
// in case a directory changes after this preflight check.
func authoringPath(root, relative string) (string, error) {
	if relative == "" || strings.Contains(relative, "\\") || filepath.IsAbs(relative) || filepath.ToSlash(filepath.Clean(relative)) != relative || relative == "." || relative == ".." || strings.HasPrefix(relative, "../") {
		return "", fmt.Errorf("expected a clean relative path inside the configured directory: %q", relative)
	}
	if strings.ContainsRune(relative, 0) {
		return "", fmt.Errorf("path contains NUL")
	}
	target := filepath.Join(root, filepath.FromSlash(relative))
	for p := target; ; p = filepath.Dir(p) {
		st, err := os.Lstat(p)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if err == nil && st.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("symlinks are not editable: %s", p)
		}
		if p == root {
			break
		}
	}
	return target, nil
}

func sourceYAML(raw []byte) (*yaml.Node, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{ymap()}}, nil
	}
	d := yaml.NewDecoder(bytes.NewReader(raw))
	var n yaml.Node
	if err := d.Decode(&n); err != nil {
		return nil, err
	}
	var extra yaml.Node
	if err := d.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("exactly one YAML document is required")
	}
	if len(n.Content) != 1 || n.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("configuration root must be a mapping")
	}
	if _, err := yamlparser.ParseBytes(raw); err != nil {
		return nil, err
	}
	return &n, nil
}
func ymap() *yaml.Node         { return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"} }
func yseq() *yaml.Node         { return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"} }
func ystr(s string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s} }
func yget(n *yaml.Node, key string) *yaml.Node {
	if n.Kind == yaml.DocumentNode {
		return yget(n.Content[0], key)
	}
	if n.Kind == yaml.SequenceNode {
		for _, c := range n.Content {
			if v := yget(c, key); v != nil {
				return v
			}
		}
		return nil
	}
	if n.Kind == yaml.MappingNode {
		for i := 0; i < len(n.Content); i += 2 {
			if n.Content[i].Value == key {
				return n.Content[i+1]
			}
		}
	}
	return nil
}
func yput(n *yaml.Node, key string, v *yaml.Node) {
	if n.Kind == yaml.SequenceNode {
		m := ymap()
		m.Content = []*yaml.Node{ystr(key), v}
		n.Content = append(n.Content, m)
		return
	}
	n.Content = append(n.Content, ystr(key), v)
}
func yamlBytes(n *yaml.Node) ([]byte, error) {
	var b bytes.Buffer
	e := yaml.NewEncoder(&b)
	e.SetIndent(2)
	if err := e.Encode(n); err != nil {
		return nil, err
	}
	if err := e.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

type scaffoldFile struct {
	Path   string `json:"path"`
	Action string `json:"action"`
	Before string `json:"before,omitempty"`
	After  string `json:"after"`
	old    []byte
}
type scaffoldPlan struct {
	Name    string         `json:"name"`
	Type    string         `json:"type"`
	Files   []scaffoldFile `json:"files"`
	Command string         `json:"command"`
	module  *authoringModule
	inputs  map[string][]byte
	tops    []string
	staged  bool
}

func newScaffoldPlan(m *authoringModule) (*scaffoldPlan, error) {
	tops, err := filepath.Glob(filepath.Join(m.Directories["hyperbricks"], "*.hyperbricks.yaml"))
	return &scaffoldPlan{module: m, inputs: map[string][]byte{m.ConfigPath: m.PackageBytes}, tops: tops}, err
}
func (p *scaffoldPlan) read(path string) ([]byte, error) {
	if b, ok := p.inputs[path]; ok {
		if b == nil {
			return nil, os.ErrNotExist
		}
		return b, nil
	}
	rel, err := filepath.Rel(p.module.Root, path)
	if err != nil {
		return nil, err
	}
	if _, err := authoringPath(p.module.Root, filepath.ToSlash(rel)); err != nil {
		return nil, err
	}
	st, err := os.Stat(path)
	if err == nil && !st.Mode().IsRegular() {
		return nil, fmt.Errorf("expected regular file: %s", path)
	}
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err == nil || os.IsNotExist(err) {
		p.inputs[path] = b
	}
	return b, err
}
func (p *scaffoldPlan) overlay(path string) ([]byte, error) {
	for _, f := range p.Files {
		if f.Path == path {
			return []byte(f.After), nil
		}
	}
	return p.read(path)
}

// hasPlannedFileIn treats a directory that will be created while applying a
// plan as present for pre-write validation of directory-backed components.
func (p *scaffoldPlan) hasPlannedFileIn(dir string) bool {
	for _, f := range p.Files {
		rel, err := filepath.Rel(dir, f.Path)
		if err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
func (p *scaffoldPlan) add(path string, after []byte) error {
	for i, f := range p.Files {
		if f.Path == path {
			if p.staged {
				p.Files[i].After = string(after)
				return nil
			}
			return fmt.Errorf("multiple writes to %s", path)
		}
	}
	old, err := p.read(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	action := "create"
	if err == nil {
		action = "modify"
	}
	p.Files = append(p.Files, scaffoldFile{Path: path, Action: action, Before: string(old), After: string(after), old: old})
	return nil
}

// Materialize each top-level scope independently, just as the runtime does.
// Repeated shared imports are allowed, but different files cannot own one name.
func (p *scaffoldPlan) graph(pending bool) (map[string]map[string]interface{}, map[string]string, error) {
	read := p.read
	tops := append([]string{}, p.tops...)
	if pending {
		read = p.overlay
		for _, f := range p.Files {
			if filepath.Dir(f.Path) == p.module.Directories["hyperbricks"] && strings.HasSuffix(f.Path, ".hyperbricks.yaml") && !containsString(tops, f.Path) {
				tops = append(tops, f.Path)
			}
		}
	}
	sort.Strings(tops)
	owners := map[string]string{}
	values := map[string]map[string]interface{}{}
	reader := func(path string) ([]byte, error) {
		rel, err := filepath.Rel(p.module.Directories["hyperbricks"], path)
		if err != nil {
			return nil, err
		}
		if _, err := authoringPath(p.module.Directories["hyperbricks"], filepath.ToSlash(rel)); err != nil {
			return nil, err
		}
		b, err := read(path)
		if err != nil {
			return nil, err
		}
		doc, err := yamlparser.ParseBytes(b)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		for _, n := range doc.Roots {
			if previous := owners[n.Name]; previous != "" && previous != path {
				return nil, fmt.Errorf("duplicate root %q in %s and %s", n.Name, previous, path)
			}
			owners[n.Name] = path
		}
		return b, nil
	}
	for _, top := range tops {
		doc, err := yamlparser.LoadFileWithReader(top, p.module.options(), reader)
		if err != nil {
			return nil, nil, err
		}
		// Template references are filenames, not a reason to write pending templates.
		opts := p.module.options()
		opts.TemplateDir = ""
		materialized, _, err := doc.MaterializeWithOptions(opts)
		if err != nil {
			return nil, nil, err
		}
		for name, v := range materialized {
			if obj, ok := v.(map[string]interface{}); ok {
				values[name] = obj
			}
		}
	}
	return values, owners, nil
}
func containsString(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func (p *scaffoldPlan) verify() error {
	tops, err := filepath.Glob(filepath.Join(p.module.Directories["hyperbricks"], "*.hyperbricks.yaml"))
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(tops, p.tops) {
		return fmt.Errorf("source file set changed; preview again")
	}
	for path, before := range p.inputs {
		rel, err := filepath.Rel(p.module.Root, path)
		if err != nil {
			return err
		}
		if _, err := authoringPath(p.module.Root, filepath.ToSlash(rel)); err != nil {
			return err
		}
		now, err := os.ReadFile(path)
		if before == nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("file appeared since preview: %s", path)
			}
		} else if err != nil || !bytes.Equal(before, now) {
			return fmt.Errorf("file changed since preview: %s", path)
		}
	}
	return nil
}
func (p *scaffoldPlan) apply() error {
	if err := p.verify(); err != nil {
		return err
	}
	var written []string
	for _, f := range p.Files {
		if err := writeScaffoldFile(p.module.Root, f); err != nil {
			return fmt.Errorf("write %s: %w; completed files retained: %v", f.Path, err, written)
		}
		written = append(written, f.Path)
	}
	return nil
}
func writeScaffoldFile(module string, f scaffoldFile) error {
	rel, err := filepath.Rel(module, f.Path)
	if err != nil {
		return err
	}
	if _, err := authoringPath(module, filepath.ToSlash(rel)); err != nil {
		return err
	}
	r, err := os.OpenRoot(module)
	if err != nil {
		return err
	}
	defer r.Close()
	current, err := r.ReadFile(rel)
	if f.old == nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("file already exists")
		}
	} else if err != nil || !bytes.Equal(current, f.old) {
		return fmt.Errorf("file changed since preview")
	}
	if err := r.MkdirAll(filepath.Dir(rel), 0755); err != nil {
		return err
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	tmp := filepath.Join(filepath.Dir(rel), ".scaffold-"+hex.EncodeToString(b))
	out, err := r.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer r.Remove(tmp)
	mode := os.FileMode(0644)
	if st, e := r.Stat(rel); e == nil {
		mode = st.Mode().Perm()
	}
	if err = out.Chmod(mode); err == nil {
		_, err = io.WriteString(out, f.After)
	}
	if err == nil {
		err = out.Sync()
	}
	closeErr := out.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if f.old == nil {
		return r.Link(tmp, rel)
	}
	current, err = r.ReadFile(rel)
	if err != nil || !bytes.Equal(current, f.old) {
		return fmt.Errorf("file changed since preview")
	}
	return r.Rename(tmp, rel)
}
