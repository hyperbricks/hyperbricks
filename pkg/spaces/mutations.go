package spaces

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
	"go.yaml.in/yaml/v4"
)

type Mutation struct {
	Action    string             `json:"action"`
	Revision  string             `json:"revision"`
	Name      string             `json:"name"`
	Source    string             `json:"source"`
	Title     string             `json:"title"`
	Route     string             `json:"route"`
	Values    map[string]string  `json:"values"`
	Meta      map[string]*string `json:"meta"`
	ResetMeta []string           `json:"reset_meta"`
}
type statusError struct {
	status  int
	message string
}

func (e *statusError) Error() string { return e.message }
func conflict(message string) error  { return &statusError{409, message} }

func (s *service) mutate(c *catalog, m Mutation, upload *pendingUpload) error {
	if !s.write {
		return &statusError{403, "Spaces is read-only"}
	}
	if m.Revision == "" || m.Revision != c.revision {
		return conflict("Source files changed; reload before saving")
	}
	if err := s.verify(c); err != nil {
		return err
	}
	switch m.Action {
	case "create":
		if upload != nil {
			return fmt.Errorf("uploads require an existing Space")
		}
		return s.create(c, m)
	case "save":
		return s.save(c, m, upload)
	case "trash", "restore":
		if upload != nil {
			return fmt.Errorf("uploads require save")
		}
		return s.toggle(c, m)
	default:
		return fmt.Errorf("unknown action")
	}
}

func (s *service) create(c *catalog, m Mutation) error {
	plan := &CreatePlan{service: s, catalog: c}
	if err := s.planCreate(c, m, plan); err != nil {
		return err
	}
	return plan.Apply()
}

func (s *service) planCreate(c *catalog, m Mutation, plan *CreatePlan) error {
	d := c.defs[m.Source]
	if d == nil || d.effective["@type"] != "<HYPERMEDIA>" {
		return fmt.Errorf("select an active hypermedia source")
	}
	if err := checkIdentity(c, m.Name, m.Route, ""); err != nil {
		return err
	}
	if err := validateText(Field{Label: "Title", Type: "text", Required: true, Max: 200}, m.Title); err != nil {
		return err
	}
	if !namePattern.MatchString(m.Source) {
		return fmt.Errorf("source name cannot be used as a Spaces directory")
	}
	instancePath, err := contained(s.dirs["hyperbricks"], "spaces/"+m.Source+"/"+m.Name+".hyperbricks.yaml")
	if err != nil {
		return err
	}
	indexPath, err := contained(s.dirs["hyperbricks"], "spaces/"+m.Source+"/index.hyperbricks.yaml")
	if err != nil {
		return err
	}
	fields, err := schemaFields(d.effective, s.dirs)
	if err != nil {
		return err
	}
	n := sequence()
	put(n, "inherit", scalar(m.Source))
	put(n, "route", scalar(m.Route))
	put(n, "title", scalar(m.Title))
	// Copy declared field defaults in source form, retaining native asset resolvers.
	// Missing inherited containers are added by setAt; other properties stay inherited.
	for _, field := range fields {
		if field.imageSource {
			if _, err := s.assetPath(field, field.Value); err != nil {
				return fmt.Errorf("%s: %w", field.ID, err)
			}
		}
		if err := setAt(n, field.Path, fieldNode(field, field.Value), d.effective); err != nil {
			return err
		}
	}
	root := mapping()
	root.HeadComment = "Generated HyperBricks Space: " + m.Title
	root.Content = []*yaml.Node{scalar(m.Name), n}
	b, err := encode(root)
	if err != nil {
		return err
	}
	if _, err := yamlparser.ParseBytes(b); err != nil {
		return err
	}
	index := c.files[indexPath]
	var indexOld []byte
	indexNew := []byte("imports:\n")
	if index != nil {
		indexOld = index.data
		indexNew = append([]byte{}, index.data...)
	}
	quoted := scalar(filepath.Base(instancePath))
	quoted.Style = yaml.DoubleQuotedStyle
	line, _ := encode(quoted)
	if len(indexNew) > 0 && indexNew[len(indexNew)-1] != '\n' {
		indexNew = append(indexNew, '\n')
	}
	indexNew = append(indexNew, []byte("  - "+strings.TrimSpace(string(line))+"\n")...)
	doc, err := parseYAML(indexNew)
	if err != nil {
		return err
	}
	if _, err := parseIndex(&sourceFile{path: indexPath, data: indexNew, doc: doc}); err != nil {
		return err
	}
	owner := d.file
	ownerNew := owner.data
	if index == nil {
		if managedIndex(s.dirs["hyperbricks"], owner.path) {
			return fmt.Errorf("source cannot be an import index")
		}
		ownerDoc, err := parseYAML(owner.data)
		if err != nil {
			return err
		}
		imports := child(ownerDoc, "imports")
		if imports == nil || imports.Tag == "!!null" {
			imports = sequence()
			put(ownerDoc.Content[0], "imports", imports)
		}
		if imports.Kind == yaml.ScalarNode {
			old := *imports
			*imports = *sequence()
			imports.Content = []*yaml.Node{&old}
		}
		if imports.Kind != yaml.SequenceNode {
			return fmt.Errorf("source imports must be a sequence")
		}
		ref := relative(filepath.Dir(owner.path), indexPath)
		imports.Content = append(imports.Content, scalar(ref))
		ownerNew, err = encode(ownerDoc)
		if err != nil {
			return err
		}
	}
	if err := s.verify(c); err != nil {
		return err
	}
	plan.changes = []CreateFile{{Path: instancePath, After: string(b)}, {Path: indexPath, Before: string(indexOld), After: string(indexNew), old: indexOld}}
	if index == nil {
		plan.changes = append(plan.changes, CreateFile{Path: owner.path, Before: string(owner.data), After: string(ownerNew), old: owner.data})
	}
	return plan.verify()
}

func (s *service) save(c *catalog, m Mutation, upload *pendingUpload) error {
	e, err := c.findEntry(m.Name)
	if err != nil {
		return err
	}
	if e.trashed {
		return fmt.Errorf("restore this Space before editing")
	}
	space, err := s.space(c, e)
	if err != nil {
		return err
	}
	if err := checkIdentity(c, m.Name, m.Route, m.Name); err != nil {
		return err
	}
	if err := validateText(Field{Label: "Title", Type: "text", Required: true, Max: 200}, m.Title); err != nil {
		return err
	}
	f := c.files[e.file]
	doc, err := parseYAML(f.data)
	if err != nil {
		return err
	}
	n := child(doc, m.Name)
	allowed := map[string]Field{}
	for _, field := range space.Fields {
		allowed[field.ID] = field
	}
	for id := range m.Values {
		if _, ok := allowed[id]; !ok {
			return fmt.Errorf("field %q is not editable in the source", id)
		}
	}
	if upload != nil {
		if upload.field == "@meta.og:image" {
			if m.Meta == nil {
				m.Meta = map[string]*string{}
			}
			ref := upload.reference
			m.Meta["og:image"] = &ref
		} else {
			if _, ok := allowed[upload.field]; !ok {
				return fmt.Errorf("upload field is not editable")
			}
			if m.Values == nil {
				m.Values = map[string]string{}
			}
			m.Values[upload.field] = upload.reference
		}
	}
	for id, field := range allowed {
		v, ok := m.Values[id]
		if !ok {
			v = field.Value
		}
		if err := validateText(field, v); err != nil {
			return err
		}
		if field.Type == "asset" && v != "" && !(upload != nil && upload.field == id && upload.reference == v) {
			if _, err := s.assetPath(field, v); err != nil {
				return fmt.Errorf("%s: %w", field.Label, err)
			}
		}
		if ok || (upload != nil && upload.field == id) {
			if err := setAt(n, field.Path, fieldNode(field, v), c.defs[m.Name].effective); err != nil {
				return err
			}
		}
	}
	put(n, "title", scalar(m.Title))
	put(n, "route", scalar(m.Route))
	keys := make([]string, 0, len(m.Meta))
	for k := range m.Meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := m.Meta[k]
		if err := validateMeta(k, v); err != nil {
			return err
		}
		if v != nil && k == "og:image" && strings.HasPrefix(*v, "/static/") {
			return fmt.Errorf("sharing image requires a public URL")
		}
		node := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
		if v != nil {
			node = scalar(*v)
		}
		head := child(n, "head")
		if head == nil {
			head = sequence()
			put(head, "type", scalar("head"))
			put(n, "head", head)
		}
		meta := child(head, "meta")
		if meta == nil {
			meta = mapping()
			put(head, "meta", meta)
		}
		if meta.Kind != yaml.MappingNode {
			return fmt.Errorf("head.meta must be a mapping")
		}
		put(meta, k, node)
	}
	for _, k := range m.ResetMeta {
		if _, duplicate := m.Meta[k]; duplicate {
			return fmt.Errorf("cannot set and reset the same metadata key")
		}
		if err := validateMeta(k, nil); err != nil {
			return err
		}
		remove(at(n, []string{"head", "meta"}), k)
	}
	b, err := encode(doc)
	if err != nil {
		return err
	}
	if _, err := yamlparser.ParseBytes(b); err != nil {
		return err
	}
	if err := s.verify(c); err != nil {
		return err
	}
	if upload != nil {
		if err := s.replaceFile(upload.path, upload.data, nil); err != nil {
			return err
		}
	}
	if err := s.replaceFile(f.path, b, f.data); err != nil {
		// The unique asset can remain unreferenced for manual recovery; old YAML stays intact.
		if upload != nil {
			return fmt.Errorf("asset retained at %s; YAML was not saved: %w", upload.reference, err)
		}
		return err
	}
	return nil
}

func (s *service) toggle(c *catalog, m Mutation) error {
	e, err := c.findEntry(m.Name)
	if err != nil {
		return err
	}
	space, err := s.space(c, e)
	if err != nil {
		return err
	}
	if m.Action == "trash" {
		if e.trashed {
			return conflict("Space is already in Trash")
		}
		if space.TrashBlocked != "" {
			return conflict(space.TrashBlocked)
		}
	} else {
		if !e.trashed {
			return conflict("Space is already active")
		}
		if space.TrashBlocked != "" {
			return conflict(space.TrashBlocked)
		}
		if err := checkIdentity(c, m.Name, space.Route, ""); err != nil {
			return err
		}
	}
	f := c.files[e.index]
	lines := strings.Split(string(f.data), "\n")
	line := lines[e.line-1]
	indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	if m.Action == "trash" {
		lines[e.line-1] = indent + "# " + strings.TrimSpace(line)
	} else {
		lines[e.line-1] = indent + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "#"))
	}
	b := []byte(strings.Join(lines, "\n"))
	doc, err := parseYAML(b)
	if err != nil {
		return err
	}
	if _, err := parseIndex(&sourceFile{path: f.path, data: b, doc: doc}); err != nil {
		return err
	}
	if err := s.verify(c); err != nil {
		return err
	}
	return s.replaceFile(f.path, b, f.data)
}
