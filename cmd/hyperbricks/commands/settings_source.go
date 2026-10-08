package commands

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
	"go.yaml.in/yaml/v4"
)

type settingsSession struct {
	root, entry string
	snapshot    *yamlparser.ConfigResult
	effective   *yamlparser.ConfigResult
	pending     map[string][]byte
	intents     map[string]yamlparser.ConfigOrigin
}

type settingsSaveError struct {
	Saved []string
	Err   error
}

func (e *settingsSaveError) Error() string {
	return fmt.Sprintf("saved %v; remaining edits retained: %v", e.Saved, e.Err)
}
func (e *settingsSaveError) Unwrap() error { return e.Err }

func openSettingsSession(root, entry string) (*settingsSession, error) {
	var err error
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	entry, err = filepath.Abs(entry)
	if err != nil {
		return nil, err
	}
	s := &settingsSession{root: root, entry: entry, pending: map[string][]byte{}, intents: map[string]yamlparser.ConfigOrigin{}}
	result, err := shared.LoadPackageConfigSource(entry, root, s.read)
	if err != nil {
		return nil, err
	}
	raw, err := s.read(entry)
	if err != nil {
		return nil, err
	}
	if _, err = shared.ValidatePackageConfigBytesAt(raw, entry, root, s.read); err != nil {
		return nil, err
	}
	s.snapshot, s.effective = result, result
	return s, nil
}

func (s *settingsSession) read(path string) ([]byte, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(s.root, resolved)
	if err != nil || rel == ".." || len(rel) > 3 && rel[:3] == ".."+string(filepath.Separator) {
		return nil, fmt.Errorf("settings path escapes module: %s", path)
	}
	if raw, ok := s.pending[resolved]; ok {
		return append([]byte(nil), raw...), nil
	}
	return os.ReadFile(resolved)
}

func (s *settingsSession) checkSnapshot() error {
	for path, source := range s.snapshot.Sources {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || resolved != source.CanonicalPath {
			return fmt.Errorf("configuration moved, missing, or retargeted: %s; reload before saving", path)
		}
		raw, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(raw, source.Bytes) {
			return fmt.Errorf("configuration changed: %s; reload before saving", path)
		}
	}
	return nil
}

func cloneSettingsNode(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	copy := *n
	copy.Content = nil
	for _, child := range n.Content {
		copy.Content = append(copy.Content, cloneSettingsNode(child))
	}
	return &copy
}
func settingsNodeAt(node *yaml.Node, path []string) *yaml.Node {
	if node != nil && node.Kind == yaml.DocumentNode {
		node = node.Content[0]
	}
	for _, part := range path {
		if node == nil {
			return nil
		}
		if node.Kind == yaml.SequenceNode {
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(node.Content) {
				return nil
			}
			node = node.Content[index]
		} else {
			node = yget(node, part)
		}
	}
	return node
}
func editSettingsNode(node *yaml.Node, path []string, value *yaml.Node) error {
	if node.Kind == yaml.DocumentNode {
		node = node.Content[0]
	}
	if len(path) == 0 {
		return fmt.Errorf("cannot replace entire package")
	}
	part := path[0]
	if node.Kind == yaml.SequenceNode {
		index, err := strconv.Atoi(part)
		if err != nil || index < 0 || index >= len(node.Content) {
			return fmt.Errorf("invalid list index %s", part)
		}
		if len(path) > 1 {
			return editSettingsNode(node.Content[index], path[1:], value)
		}
		if value == nil {
			node.Content = append(node.Content[:index], node.Content[index+1:]...)
		} else {
			node.Content[index] = value
		}
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("cannot edit through a scalar or resolver value")
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value != part {
			continue
		}
		if len(path) > 1 {
			return editSettingsNode(node.Content[i+1], path[1:], value)
		}
		if value == nil {
			node.Content = append(node.Content[:i], node.Content[i+2:]...)
		} else {
			old := node.Content[i+1]
			value.HeadComment, value.LineComment, value.FootComment = old.HeadComment, old.LineComment, old.FootComment
			node.Content[i+1] = value
		}
		return nil
	}
	if value == nil {
		return fmt.Errorf("definition does not exist")
	}
	if len(path) == 1 {
		yput(node, part, value)
		return nil
	}
	child := ymap()
	yput(node, part, child)
	return editSettingsNode(child, path[1:], value)
}

// stage edits the winning definition unless an entry override was explicitly
// requested. A nested list override copies its complete inherited list first.
func (s *settingsSession) stage(path []string, diagnosticPath string, value *yaml.Node, override bool) error {
	if err := s.checkSnapshot(); err != nil {
		return err
	}
	origin, exists := s.effective.Origins[diagnosticPath]
	destination := s.entry
	editPath := append([]string(nil), path...)
	if exists && !override {
		destination = origin.File
		editPath = origin.Path
	}
	if !override && value != nil && settingsNodesEqual(settingsNodeAt(s.effective.Unresolved, path), value) {
		return nil
	}
	resolved, err := filepath.EvalSymlinks(destination)
	if err != nil {
		return err
	}
	raw, err := s.read(destination)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err = yaml.Unmarshal(raw, &doc); err != nil {
		return err
	}
	if override {
		for i := 1; i < len(path); i++ {
			ancestor := settingsNodeAt(s.effective.Unresolved, path[:i])
			if ancestor != nil && ancestor.Kind == yaml.SequenceNode {
				list := cloneSettingsNode(ancestor)
				if err := editSettingsNode(list, path[i:], value); err != nil {
					return err
				}
				value, editPath = list, path[:i]
				break
			}
		}
	}
	if err = editSettingsNode(&doc, editPath, cloneSettingsNode(value)); err != nil {
		return err
	}
	changed, err := yamlBytes(&doc)
	if err != nil {
		return err
	}
	previous, had := s.pending[resolved]
	s.pending[resolved] = changed
	candidate, err := s.validate()
	if err == nil && value != nil && value.Kind != yaml.MappingNode && !settingsNodesEqual(settingsNodeAt(candidate.Unresolved, editPath), value) {
		err = fmt.Errorf("edit is masked by another definition: %s", diagnosticPath)
	}
	if err != nil {
		if had {
			s.pending[resolved] = previous
		} else {
			delete(s.pending, resolved)
		}
		return err
	}
	s.effective = candidate
	if s.intents == nil {
		s.intents = map[string]yamlparser.ConfigOrigin{}
	}
	intent := candidate.Origins[diagnosticPath]
	if intent.File == "" {
		intent.Path = append([]string(nil), path...)
	}
	s.intents[diagnosticPath] = intent
	return nil
}

func (s *settingsSession) validate() (*yamlparser.ConfigResult, error) {
	raw, err := s.read(s.entry)
	if err != nil {
		return nil, err
	}
	if _, err = shared.ValidatePackageConfigBytesAt(raw, s.entry, s.root, s.read); err != nil {
		return nil, err
	}
	return shared.LoadPackageConfigSource(s.entry, s.root, s.read)
}

func (s *settingsSession) changes() []scaffoldFile {
	var files []scaffoldFile
	for path, after := range s.pending {
		for _, source := range s.snapshot.Sources {
			if source.CanonicalPath == path {
				if !bytes.Equal(source.Bytes, after) {
					files = append(files, scaffoldFile{Path: path, Action: "update", Before: string(source.Bytes), After: string(after), old: source.Bytes})
				}
				break
			}
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files
}

func (s *settingsSession) save() ([]string, error) { return s.saveWith(writeScaffoldFile) }

func (s *settingsSession) saveWith(write func(string, scaffoldFile) error) ([]string, error) {
	if err := s.checkSnapshot(); err != nil {
		return nil, err
	}
	candidate, err := s.validate()
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(candidate.Materialized, s.effective.Materialized) {
		return nil, fmt.Errorf("resolved configuration changed; reload and review before saving")
	}
	var saved []string
	for _, file := range s.changes() {
		if err := s.checkSnapshot(); err != nil {
			return saved, &settingsSaveError{saved, err}
		}
		if err := write(s.root, file); err != nil {
			return saved, &settingsSaveError{saved, err}
		}
		saved = append(saved, file.Path)
		delete(s.pending, file.Path)
		for path, source := range s.snapshot.Sources {
			if source.CanonicalPath == file.Path {
				source.Bytes = []byte(file.After)
				s.snapshot.Sources[path] = source
			}
		}
	}
	next, err := openSettingsSession(s.root, s.entry)
	if err != nil {
		return saved, &settingsSaveError{saved, err}
	}
	*s = *next
	return saved, nil
}

// Reload never guesses moved destinations. Unchanged source files can retain
// their pending edits; changed source files need an explicit user decision.
func (s *settingsSession) reload() ([]string, error) {
	next, err := openSettingsSession(s.root, s.entry)
	if err != nil {
		return nil, err
	}
	var conflicts []string
	for path, pending := range s.pending {
		var oldRaw, newRaw []byte
		for _, source := range s.snapshot.Sources {
			if source.CanonicalPath == path {
				oldRaw = source.Bytes
			}
		}
		for _, source := range next.snapshot.Sources {
			if source.CanonicalPath == path {
				newRaw = source.Bytes
			}
		}
		if newRaw == nil || !bytes.Equal(oldRaw, newRaw) {
			conflicts = append(conflicts, path)
			continue
		}
		next.pending[path] = pending
	}
	if len(conflicts) > 0 {
		sort.Strings(conflicts)
		return conflicts, fmt.Errorf("pending edits conflict with %v; review or discard them before reloading", conflicts)
	}
	result, err := next.validate()
	if err != nil {
		return nil, err
	}
	for path, origin := range s.intents {
		if origin.File == "" {
			if _, exists := result.Origins[path]; !exists {
				continue
			}
		}
		if !reflect.DeepEqual(result.Origins[path].Path, origin.Path) || result.Origins[path].File != origin.File {
			return []string{path}, fmt.Errorf("setting ownership changed: %s; review pending edits", path)
		}
	}
	next.intents = s.intents
	next.effective = result
	*s = *next
	return nil, nil
}

func settingsNodesEqual(a, b *yaml.Node) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Kind != b.Kind || a.Tag != b.Tag || a.Value != b.Value || len(a.Content) != len(b.Content) {
		return false
	}
	for i := range a.Content {
		if !settingsNodesEqual(a.Content[i], b.Content[i]) {
			return false
		}
	}
	return true
}
