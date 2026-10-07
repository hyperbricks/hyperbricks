package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v4"
)

// Exercise the shipped configuration fixture through the settings save path.
func TestSettingsConfigurationModule(t *testing.T) {
	root := t.TempDir()
	fixture := filepath.Join("..", "..", "..", "modules", "project-lifecycle-test")
	for _, name := range []string{"package.configuration.hyperbricks.yaml", "package.static.hyperbricks.yaml", "config/lifecycle.hyperbricks.yaml"} {
		raw, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, raw, 0644); err != nil {
			t.Fatal(err)
		}
	}
	entry := filepath.Join(root, "package.configuration.hyperbricks.yaml")
	session, err := openSettingsSession(root, entry)
	if err != nil {
		t.Fatal(err)
	}
	port := settingsEffectiveAt(session.effective.Materialized, []string{"hyperbricks", "server", "port"})
	if fmt.Sprint(port) != "8107" {
		t.Fatalf("entry variable did not override imported value: %v", port)
	}
	dirs := settingsNodeAt(session.effective.Unresolved, []string{"hyperbricks", "development", "watch_dirs"})
	if dirs.Kind != yaml.SequenceNode || len(dirs.Content) != 0 {
		t.Fatal("entry did not clear imported list")
	}
	path := []string{"hyperbricks", "server", "beautify"}
	for _, override := range []bool{false, true} {
		value := "true"
		owner := "package.static.hyperbricks.yaml"
		if override {
			value = "false"
			owner = "package.configuration.hyperbricks.yaml"
		}
		if err := session.stage(path, "hyperbricks.server.beautify", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: value}, override); err != nil {
			t.Fatal(err)
		}
		changes := session.changes()
		if len(changes) != 1 || filepath.Base(changes[0].Path) != owner {
			t.Fatalf("wrong owner: %v", changes)
		}
		if _, err := session.save(); err != nil {
			t.Fatal(err)
		}
		session, err = openSettingsSession(root, entry)
		if err != nil {
			t.Fatal(err)
		}
		if got := settingsEffectiveAt(session.effective.Materialized, path); fmt.Sprint(got) != value {
			t.Fatalf("save did not survive reload: %v", got)
		}
	}
}
