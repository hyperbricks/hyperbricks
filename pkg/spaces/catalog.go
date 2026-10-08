package spaces

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
	"go.yaml.in/yaml/v4"
)

type definition struct {
	file      *sourceFile
	node      *yaml.Node
	effective map[string]interface{}
	scope     *yamlparser.Document
}
type importEntry struct {
	index, file string
	line        int
	trashed     bool
}
type catalog struct {
	files    map[string]*sourceFile
	defs     map[string]*definition
	entries  []importEntry
	refs     map[string]int
	tops     []string
	revision string
}
type Source struct {
	Name   string  `json:"name"`
	Title  string  `json:"title"`
	Fields []Field `json:"fields"`
}
type Space struct {
	Name         string                 `json:"name"`
	Source       string                 `json:"source"`
	Title        string                 `json:"title"`
	Route        string                 `json:"route"`
	File         string                 `json:"file"`
	Trashed      bool                   `json:"trashed"`
	Fields       []Field                `json:"fields"`
	Meta         map[string]interface{} `json:"meta"`
	SourceMeta   map[string]interface{} `json:"source_meta"`
	TrashBlocked string                 `json:"trash_blocked,omitempty"`
}
type Snapshot struct {
	Revision     string        `json:"revision"`
	Sources      []Source      `json:"sources"`
	Spaces       []Space       `json:"spaces"`
	Write        bool          `json:"write"`
	Watch        bool          `json:"watch"`
	PublicOrigin string        `json:"public_origin"`
	SharingImage *UploadPolicy `json:"sharing_image,omitempty"`
}

func (s *service) catalog() (*catalog, error) {
	c := &catalog{files: map[string]*sourceFile{}, defs: map[string]*definition{}, refs: map[string]int{}}
	tops, err := filepath.Glob(filepath.Join(s.dirs["hyperbricks"], "*.hyperbricks.yaml"))
	if err != nil {
		return nil, err
	}
	c.tops = tops
	visiting := map[string]bool{}
	loaded := map[string]bool{}
	var visit func(string) error
	visit = func(path string) error {
		if visiting[path] {
			return fmt.Errorf("import cycle at %s", path)
		}
		if loaded[path] {
			return nil
		}
		visiting[path] = true
		defer delete(visiting, path)
		f, err := readSource(path)
		if err != nil {
			return err
		}
		c.files[path] = f
		loaded[path] = true
		if _, err := yamlparser.ParseBytes(f.data); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		root := f.doc.Content[0]
		for i := 0; i < len(root.Content); i += 2 {
			name, n := root.Content[i].Value, root.Content[i+1]
			if name == "imports" || name == "vars" {
				continue
			}
			if previous := c.defs[name]; previous != nil {
				return fmt.Errorf("duplicate definition %q in %s and %s", name, previous.file.path, path)
			}
			c.defs[name] = &definition{file: f, node: n}
		}
		imports := child(root, "imports")
		var items []*yaml.Node
		if imports != nil {
			if imports.Kind == yaml.SequenceNode {
				items = imports.Content
			} else if imports.Value != "" {
				items = []*yaml.Node{imports}
			}
		}
		for _, item := range items {
			rel, err := filepath.Rel(s.dirs["hyperbricks"], filepath.Join(filepath.Dir(path), item.Value))
			if err != nil {
				return err
			}
			target, err := contained(s.dirs["hyperbricks"], rel)
			if err != nil {
				return err
			}
			c.refs[target]++
			if err := visit(target); err != nil {
				return err
			}
		}
		if managedIndex(s.dirs["hyperbricks"], path) {
			entries, err := parseIndex(f)
			if err != nil {
				return err
			}
			for _, e := range entries {
				if _, err := contained(s.dirs["hyperbricks"], relative(s.dirs["hyperbricks"], e.file)); err != nil {
					return err
				}
				if e.trashed {
					f, err := readSource(e.file)
					if err != nil {
						return err
					}
					c.files[e.file] = f
				}
				c.entries = append(c.entries, e)
			}
		}
		return nil
	}
	for _, top := range tops {
		if _, err := contained(s.dirs["hyperbricks"], filepath.Base(top)); err != nil {
			return nil, err
		}
		c.refs[top]++
		if err := visit(top); err != nil {
			return nil, err
		}
	}
	for _, top := range tops {
		result, err := yamlparser.ProcessFile(top, s.parserOptions())
		if err != nil {
			return nil, err
		}
		for name, v := range result.Materialized {
			if d := c.defs[name]; d != nil {
				d.effective, _ = v.(map[string]interface{})
				d.scope = result.Document
			}
		}
	}
	c.revision = digest(c.files, tops)
	return c, nil
}

func relative(root, p string) string { r, _ := filepath.Rel(root, p); return filepath.ToSlash(r) }
func managedIndex(root, path string) bool {
	parts := strings.Split(relative(root, path), "/")
	return len(parts) == 3 && parts[0] == "spaces" && parts[2] == "index.hyperbricks.yaml"
}

// A managed index contains imports only. Commented scalar entries are durable Trash.
func parseIndex(f *sourceFile) ([]importEntry, error) {
	root := f.doc.Content[0]
	if len(root.Content) != 2 || root.Content[0].Value != "imports" {
		return nil, fmt.Errorf("managed Spaces index must contain only imports: %s", f.path)
	}
	imports := root.Content[1]
	if imports.Kind != yaml.SequenceNode && !(imports.Kind == yaml.ScalarNode && imports.Tag == "!!null") {
		return nil, fmt.Errorf("managed imports must be a block sequence")
	}
	if imports.Style&yaml.FlowStyle != 0 {
		return nil, fmt.Errorf("managed imports must use one entry per line")
	}
	entries := []importEntry{}
	seen := map[string]bool{}
	add := func(n *yaml.Node, line int, trashed bool) error {
		if n.Kind != yaml.ScalarNode || n.Tag != "!!str" || filepath.Base(n.Value) != n.Value || !strings.HasSuffix(n.Value, ".hyperbricks.yaml") || n.Value == "index.hyperbricks.yaml" {
			return fmt.Errorf("managed index entries must name sibling instance files")
		}
		if seen[n.Value] {
			return fmt.Errorf("duplicate managed import %q", n.Value)
		}
		seen[n.Value] = true
		entries = append(entries, importEntry{f.path, filepath.Join(filepath.Dir(f.path), n.Value), line, trashed})
		return nil
	}
	for _, n := range imports.Content {
		if err := add(n, n.Line, false); err != nil {
			return nil, err
		}
	}
	for i, line := range strings.Split(string(f.data), "\n") {
		if i+1 <= root.Content[0].Line {
			continue
		}
		trim := strings.TrimSpace(line)
		if !strings.HasPrefix(trim, "#") {
			continue
		}
		trim = strings.TrimSpace(strings.TrimPrefix(trim, "#"))
		if !strings.HasPrefix(trim, "- ") {
			continue
		}
		var doc yaml.Node
		if err := yaml.Unmarshal([]byte(trim), &doc); err != nil {
			return nil, fmt.Errorf("invalid commented import at %s:%d", f.path, i+1)
		}
		if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.SequenceNode || len(doc.Content[0].Content) != 1 {
			return nil, fmt.Errorf("invalid commented import")
		}
		if err := add(doc.Content[0].Content[0], i+1, true); err != nil {
			return nil, err
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].line < entries[j].line })
	return entries, nil
}

func (s *service) snapshot(c *catalog) (Snapshot, error) {
	snap := Snapshot{Revision: c.revision, Write: s.write, Watch: s.watch, PublicOrigin: s.publicOrigin, SharingImage: s.sharingImage, Sources: []Source{}, Spaces: []Space{}}
	for name, d := range c.defs {
		if d.effective["@type"] != "<HYPERMEDIA>" {
			continue
		}
		fields, err := SourceFields(d.effective, s.dirs)
		if err != nil {
			return snap, fmt.Errorf("source %s: %w", name, err)
		}
		snap.Sources = append(snap.Sources, Source{name, str(d.effective["title"]), fields})
	}
	for _, entry := range c.entries {
		space, err := s.space(c, entry)
		if err != nil {
			return snap, err
		}
		snap.Spaces = append(snap.Spaces, space)
	}
	sort.Slice(snap.Sources, func(i, j int) bool { return snap.Sources[i].Name < snap.Sources[j].Name })
	sort.Slice(snap.Spaces, func(i, j int) bool { return snap.Spaces[i].Name < snap.Spaces[j].Name })
	return snap, nil
}

func (s *service) space(c *catalog, e importEntry) (Space, error) {
	f := c.files[e.file]
	if f == nil {
		return Space{}, fmt.Errorf("missing Space file")
	}
	name, n, err := instanceNode(f)
	if err != nil {
		return Space{}, err
	}
	source := value(n, "inherit")
	d := c.defs[source]
	space := Space{Name: name, Source: source, Title: value(n, "title"), Route: value(n, "route"), File: relative(s.dirs["hyperbricks"], f.path), Trashed: e.trashed}
	if d == nil || d.effective["@type"] != "<HYPERMEDIA>" {
		if e.trashed {
			space.TrashBlocked = "Source is missing or no longer hypermedia"
			return space, nil
		}
		return space, fmt.Errorf("Space %s has no eligible source %q", name, source)
	}
	effective := map[string]interface{}(nil)
	if !e.trashed {
		if def := c.defs[name]; def != nil {
			effective = def.effective
		}
	}
	if effective == nil {
		// Resolve retained content against active definitions without activating its import.
		doc := &yamlparser.Document{Vars: map[string]interface{}{}}
		for _, root := range d.scope.Roots {
			if root.Name != name {
				doc.Roots = append(doc.Roots, root)
			}
		}
		for key, v := range d.scope.Vars {
			doc.Vars[key] = v
		}
		part, err := yamlparser.ParseBytes(f.data)
		if err != nil {
			return space, err
		}
		doc.Roots = append(doc.Roots, part.Roots...)
		for key, v := range part.Vars {
			if _, exists := doc.Vars[key]; !exists {
				doc.Vars[key] = v
			}
		}
		materialized, _, err := doc.MaterializeWithOptions(s.parserOptions())
		if err != nil {
			return space, err
		}
		effective, _ = materialized[name].(map[string]interface{})
	}
	if effective["@type"] != "<HYPERMEDIA>" {
		return space, fmt.Errorf("Space %s is not hypermedia", name)
	}
	space.Title, space.Route = str(effective["title"]), str(effective["route"])
	fields, err := schemaFields(d.effective, s.dirs)
	if err != nil {
		return space, err
	}
	for i := range fields {
		// Validate the effective instance against the source-owned target policy.
		target := fields[i]
		if err := configureFieldTarget(effective, &target); err != nil {
			return space, fmt.Errorf("field %s: %w", target.ID, err)
		}
		if target.imageSource != fields[i].imageSource || target.markdownFile != fields[i].markdownFile || target.markdownMaxBytes != fields[i].markdownMaxBytes {
			return space, fmt.Errorf("field %s changed its source-owned target semantics", target.ID)
		}
		fields[i].Value, err = fieldValue(effective, fields[i], s.dirs)
		if err != nil {
			return space, fmt.Errorf("field %s: %w", fields[i].ID, err)
		}
	}
	space.Fields = fields
	space.Meta = metaMap(effective)
	space.SourceMeta = metaMap(d.effective)
	if !e.trashed {
		space.TrashBlocked = trashBlock(c, e, name)
	} else if c.refs[e.file] > 0 {
		space.TrashBlocked = "This retained file is already loaded through another import"
	} else if imports := child(f.doc, "imports"); imports != nil && (len(imports.Content) > 0 || imports.Value != "") {
		space.TrashBlocked = "Retained Spaces with imports require manual recovery"
	}
	return space, nil
}

func trashBlock(c *catalog, e importEntry, name string) string {
	if c.refs[e.file] != 1 {
		return "This file is loaded through another import or top-level route"
	}
	if imports := child(c.files[e.file].doc, "imports"); imports != nil && (len(imports.Content) > 0 || imports.Value != "") {
		return "This Space carries imports; remove its dependents before moving it to Trash"
	}
	for other, d := range c.defs {
		if other == name {
			continue
		}
		var depends func(*yaml.Node) bool
		depends = func(n *yaml.Node) bool {
			if ref := value(n, "inherit"); ref == name || strings.HasPrefix(ref, name+".") {
				return true
			}
			for _, ch := range n.Content {
				if depends(ch) {
					return true
				}
			}
			return false
		}
		if depends(d.node) {
			return fmt.Sprintf("%s inherits this Space", other)
		}
	}
	return ""
}
func (c *catalog) findEntry(name string) (importEntry, error) {
	var result *importEntry
	for _, e := range c.entries {
		f := c.files[e.file]
		found, _, err := instanceNode(f)
		if err != nil {
			return e, err
		}
		if found == name {
			if result != nil {
				return e, fmt.Errorf("ambiguous Space name %q", name)
			}
			copy := e
			result = &copy
		}
	}
	if result == nil {
		return importEntry{}, fmt.Errorf("Space not found")
	}
	return *result, nil
}

func instanceNode(f *sourceFile) (string, *yaml.Node, error) {
	if f == nil {
		return "", nil, fmt.Errorf("missing Space file")
	}
	root := f.doc.Content[0]
	var name string
	var node *yaml.Node
	for i := 0; i < len(root.Content); i += 2 {
		key := root.Content[i].Value
		if key == "imports" || key == "vars" {
			continue
		}
		if node != nil || root.Content[i+1].Kind != yaml.SequenceNode {
			return "", nil, fmt.Errorf("Space file must contain exactly one definition: %s", f.path)
		}
		name, node = key, root.Content[i+1]
	}
	if node == nil {
		return "", nil, fmt.Errorf("Space file has no definition: %s", f.path)
	}
	return name, node, nil
}
func metaMap(m map[string]interface{}) map[string]interface{} {
	v, _ := getMap(m, []string{"head", "meta"}).(map[string]interface{})
	if v == nil {
		return map[string]interface{}{}
	}
	return v
}
func getMap(m map[string]interface{}, parts []string) interface{} {
	var v interface{} = m
	for _, p := range parts {
		next, _ := v.(map[string]interface{})
		v = next[p]
	}
	return v
}
func str(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func (s *service) verify(c *catalog) error {
	tops, err := filepath.Glob(filepath.Join(s.dirs["hyperbricks"], "*.hyperbricks.yaml"))
	if err != nil {
		return err
	}
	files := map[string]*sourceFile{}
	for p := range c.files {
		if _, err := contained(s.dirs["hyperbricks"], relative(s.dirs["hyperbricks"], p)); err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files[p] = &sourceFile{data: b}
	}
	if digest(files, tops) != c.revision {
		return conflict("Source files changed; reload before saving")
	}
	return nil
}
