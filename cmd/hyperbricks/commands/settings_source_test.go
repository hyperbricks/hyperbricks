package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

func settingsFixture(t *testing.T) (*settingsSession, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "config"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, PackageConfigFileName), []byte("# entry\nimports: [config/runtime.yaml]\n"), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config/runtime.yaml"), []byte("# keep this comment\nhyperbricks:\n  mode: development\n  server:\n    port: 8080 # listen here\n"), 0640); err != nil {
		t.Fatal(err)
	}
	s, err := openSettingsSession(root, filepath.Join(root, PackageConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	return s, root
}
func TestSettingsEditsOwnerAndPreservesSources(t *testing.T) {
	s, root := settingsFixture(t)
	entry, _ := os.ReadFile(s.entry)
	value := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "9090"}
	if err := s.stage([]string{"hyperbricks", "server", "port"}, "hyperbricks.server.port", value, false); err != nil {
		t.Fatal(err)
	}
	if len(s.changes()) != 1 || !strings.HasSuffix(s.changes()[0].Path, "runtime.yaml") {
		t.Fatal(s.changes())
	}
	if _, err := s.save(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(s.entry)
	if string(got) != string(entry) {
		t.Fatal("entry was flattened")
	}
	fragment := filepath.Join(root, "config/runtime.yaml")
	raw, _ := os.ReadFile(fragment)
	if !strings.Contains(string(raw), "9090 # listen here") || !strings.Contains(string(raw), "keep this comment") {
		t.Fatalf("comments/value lost: %s", raw)
	}
	info, _ := os.Stat(fragment)
	if info.Mode().Perm() != 0640 {
		t.Fatal("permissions changed")
	}
	if err := s.stage([]string{"hyperbricks", "server", "port"}, "hyperbricks.server.port", value, false); err != nil {
		t.Fatal(err)
	}
	if len(s.changes()) != 0 {
		t.Fatal("no-op edit rewrites source")
	}
	if err := s.stage([]string{"hyperbricks", "server", "port"}, "hyperbricks.server.port", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "8081"}, true); err != nil {
		t.Fatal(err)
	}
	if len(s.changes()) != 1 || !strings.HasSuffix(s.changes()[0].Path, PackageConfigFileName) {
		t.Fatal("override did not target entry")
	}
}
func TestSettingsExternalChangesBlockSave(t *testing.T) {
	for _, action := range []string{"edit", "delete", "move"} {
		t.Run(action, func(t *testing.T) {
			s, root := settingsFixture(t)
			if err := s.stage([]string{"hyperbricks", "mode"}, "hyperbricks.mode", ystr("live"), false); err != nil {
				t.Fatal(err)
			}
			fragment := filepath.Join(root, "config/runtime.yaml")
			switch action {
			case "edit":
				if err := os.WriteFile(fragment, []byte("hyperbricks: {mode: debug}"), 0600); err != nil {
					t.Fatal(err)
				}
			case "delete":
				if err := os.Remove(fragment); err != nil {
					t.Fatal(err)
				}
			case "move":
				if err := os.Rename(fragment, fragment+".moved"); err != nil {
					t.Fatal(err)
				}
			}
			saved, err := s.save()
			if err == nil || len(saved) != 0 || len(s.pending) == 0 {
				t.Fatalf("saved=%v err=%v pending=%v", saved, err, s.pending)
			}
			if action != "edit" {
				if _, err := os.Stat(fragment); !os.IsNotExist(err) {
					t.Fatal("missing import recreated")
				}
			}
		})
	}
}
func TestSettingsReloadDetectsChangedOwnership(t *testing.T) {
	s, _ := settingsFixture(t)
	if err := s.stage([]string{"hyperbricks", "mode"}, "hyperbricks.mode", ystr("live"), false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.entry, []byte("imports: [config/runtime.yaml]\nhyperbricks: {mode: debug}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.reload(); err == nil {
		t.Fatal("shadowed edit silently reapplied")
	}
	if len(s.pending) == 0 {
		t.Fatal("pending edits lost")
	}
}
func TestSettingsSingleFileDefaultsAndRemoval(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, PackageConfigFileName)
	if err := os.WriteFile(entry, []byte("hyperbricks: {mode: development}"), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := openSettingsSession(root, entry)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.stage([]string{"hyperbricks", "server", "port"}, "hyperbricks.server.port", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "8089"}, false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.save(); err != nil {
		t.Fatal(err)
	}
	if err = s.stage([]string{"hyperbricks", "server", "port"}, "hyperbricks.server.port", nil, false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.save(); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(entry)
	if strings.Contains(string(raw), "imports:") || strings.Contains(string(raw), "port:") {
		t.Fatal(string(raw))
	}
}
