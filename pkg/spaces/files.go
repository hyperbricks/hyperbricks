package spaces

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v4"
)

type sourceFile struct {
	path string
	data []byte
	doc  *yaml.Node
}

func readSource(path string) (*sourceFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 4<<20+1))
	if err != nil {
		return nil, err
	}
	if len(b) > 4<<20 {
		return nil, fmt.Errorf("YAML file exceeds 4 MiB: %s", path)
	}
	doc, err := parseYAML(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &sourceFile{path: path, data: b, doc: doc}, nil
}

func parseYAML(b []byte) (*yaml.Node, error) {
	var doc yaml.Node
	d := yaml.NewDecoder(bytes.NewReader(b))
	if err := d.Decode(&doc); err != nil {
		return nil, err
	}
	var extra yaml.Node
	if err := d.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("expected one YAML document")
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected a YAML root mapping")
	}
	var validate func(*yaml.Node) error
	validate = func(n *yaml.Node) error {
		if n.Kind == yaml.AliasNode || n.Anchor != "" {
			return fmt.Errorf("Spaces editing does not support YAML anchors or aliases")
		}
		if n.Kind == yaml.MappingNode {
			seen := map[string]bool{}
			for i := 0; i < len(n.Content); i += 2 {
				k := n.Content[i]
				if k.Kind != yaml.ScalarNode || seen[k.Value] {
					return fmt.Errorf("duplicate or non-scalar YAML key %q", k.Value)
				}
				seen[k.Value] = true
			}
		}
		for _, child := range n.Content {
			if err := validate(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := validate(&doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func scalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}
func mapping() *yaml.Node  { return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"} }
func sequence() *yaml.Node { return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"} }
func child(n *yaml.Node, key string) *yaml.Node {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.DocumentNode {
		return child(n.Content[0], key)
	}
	if n.Kind == yaml.SequenceNode {
		for _, entry := range n.Content {
			if v := child(entry, key); v != nil {
				return v
			}
		}
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
func value(n *yaml.Node, key string) string {
	if v := child(n, key); v != nil {
		return v.Value
	}
	return ""
}
func put(n *yaml.Node, key string, v *yaml.Node) {
	if old := child(n, key); old != nil {
		head, line, foot := old.HeadComment, old.LineComment, old.FootComment
		*old = *v
		old.HeadComment, old.LineComment, old.FootComment = head, line, foot
		return
	}
	if n.Kind == yaml.SequenceNode {
		m := mapping()
		m.Content = append(m.Content, scalar(key), v)
		n.Content = append(n.Content, m)
	} else {
		n.Content = append(n.Content, scalar(key), v)
	}
}
func remove(n *yaml.Node, key string) {
	if n == nil {
		return
	}
	if n.Kind == yaml.SequenceNode {
		for i, e := range n.Content {
			if child(e, key) != nil {
				n.Content = append(n.Content[:i], n.Content[i+1:]...)
				return
			}
		}
	} else if n.Kind == yaml.MappingNode {
		for i := 0; i < len(n.Content); i += 2 {
			if n.Content[i].Value == key {
				n.Content = append(n.Content[:i], n.Content[i+2:]...)
				return
			}
		}
	}
}
func at(n *yaml.Node, parts []string) *yaml.Node {
	for _, p := range parts {
		n = child(n, p)
	}
	return n
}

// Build only the missing override containers; inherited structure remains owned by the source.
func setAt(n *yaml.Node, parts []string, v *yaml.Node, effective map[string]interface{}) error {
	for i, p := range parts {
		if i == len(parts)-1 {
			put(n, p, v)
			return nil
		}
		next := child(n, p)
		nextEffective, _ := effective[p].(map[string]interface{})
		if next == nil {
			next = mapping()
			if typ, ok := nextEffective["@type"]; ok {
				next = sequence()
				put(next, "type", scalar(strings.ToLower(strings.Trim(fmt.Sprint(typ), "<>"))))
			}
			put(n, p, next)
		}
		if next.Kind != yaml.MappingNode && next.Kind != yaml.SequenceNode {
			return fmt.Errorf("cannot edit through non-container %q", p)
		}
		n, effective = next, nextEffective
	}
	return nil
}
func encode(n *yaml.Node) ([]byte, error) {
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

func contained(root, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) || strings.Contains(relative, "\\") {
		return "", fmt.Errorf("invalid relative path %q", relative)
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target := filepath.Join(root, filepath.FromSlash(relative))
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes configured directory")
	}
	// Reject symlinks even when their current target is inside the module.
	for p := target; ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("symlink is not editable: %s", p)
		}
		if p == root {
			break
		}
	}
	return target, nil
}

func digest(files map[string]*sourceFile, tops []string) string {
	h := sha256.New()
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range tops {
		fmt.Fprintf(h, "top:%s\x00", k)
	}
	for _, k := range keys {
		fmt.Fprintf(h, "%s\x00%d\x00", k, len(files[k].data))
		h.Write(files[k].data)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Write temp files beside their targets, then rename. New leaves precede imports.
func (s *service) replaceFile(path string, b []byte, old []byte) error {
	base := ""
	for _, key := range []string{"hyperbricks", "static", "resources"} {
		if strings.HasPrefix(path, s.dirs[key]+string(filepath.Separator)) && len(s.dirs[key]) > len(base) {
			base = s.dirs[key]
		}
	}
	if base == "" {
		return fmt.Errorf("write is outside configured source and asset directories")
	}
	if _, err := contained(base, relative(base, path)); err != nil {
		return err
	}
	module, err := os.OpenRoot(s.module)
	if err != nil {
		return err
	}
	defer module.Close()
	baseRel := relative(s.module, base)
	if err := module.MkdirAll(baseRel, 0755); err != nil {
		return err
	}
	root, err := module.OpenRoot(baseRel)
	if err != nil {
		return err
	}
	defer root.Close()
	target := relative(base, path)
	current, err := root.ReadFile(target)
	if old == nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("file already exists: %s", path)
		}
	} else if err != nil || !bytes.Equal(current, old) {
		return conflict("file changed; reload before saving")
	}
	if err := root.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return err
	}
	temp := filepath.Join(filepath.Dir(target), ".spaces-"+hex.EncodeToString(random))
	f, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(temp)
	mode := os.FileMode(0644)
	if st, e := root.Stat(target); e == nil {
		mode = st.Mode().Perm()
	}
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if old == nil {
		// Link gives no-overwrite semantics for new instance/asset files.
		return root.Link(temp, target)
	}
	current, err = root.ReadFile(target)
	if err != nil || !bytes.Equal(current, old) {
		return conflict("file changed; reload before saving")
	}
	return root.Rename(temp, target)
}
