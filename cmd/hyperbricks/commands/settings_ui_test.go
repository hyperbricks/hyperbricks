package commands

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"go.yaml.in/yaml/v4"
)

func TestSettingsMenuStagesTypedValueAndKeepsHelpVisible(t *testing.T) {
	session, _ := settingsFixture(t)
	model, err := newSettingsModel(session)
	if err != nil {
		t.Fatal(err)
	}
	var mode settingsItem
	for _, row := range model.allItems {
		item := row.(settingsItem)
		if item.key == "hyperbricks.mode" {
			mode = item
		}
	}
	if len(mode.choices) != 3 || mode.effective != "development" || mode.defaultValue == "" {
		t.Fatalf("%+v", mode)
	}
	model.beginEdit(mode, false)
	if err = model.apply("debug"); err != nil {
		t.Fatal(err)
	}
	if len(session.changes()) != 1 {
		t.Fatal("edit not staged")
	}
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if !strings.Contains(model.View(), "review/save") {
		t.Fatal("help clipped")
	}
	model.review()
	if !strings.Contains(model.view.View(), "runtime.yaml") {
		t.Fatal("review missing source")
	}
	model.mode = "browse"
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	if model.mode != "reload_discard" {
		t.Fatal(model.mode)
	}
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if len(model.session.changes()) != 0 {
		t.Fatal("discard failed")
	}
}
func TestSettingsStructuredHookAndRejectMultipleDocuments(t *testing.T) {
	session, _ := settingsFixture(t)
	model, err := newSettingsModel(session)
	if err != nil {
		t.Fatal(err)
	}
	var phase settingsItem
	for _, row := range model.allItems {
		item := row.(settingsItem)
		if item.key == "hyperbricks.hooks.finish" {
			phase = item
		}
	}
	if err = model.addTask(phase); err != nil {
		t.Fatal(err)
	}
	if err = model.addTask(phase); err != nil {
		t.Fatal(err)
	}
	var task settingsItem
	for _, row := range model.allItems {
		item := row.(settingsItem)
		if item.key == "hyperbricks.hooks.finish[1]" {
			task = item
		}
	}
	if err = model.moveTask(task, -1); err != nil {
		t.Fatal(err)
	}
	model.beginEdit(phase, false)
	if err = model.apply("[]\n---\n[]"); err == nil {
		t.Fatal("accepted extra YAML document")
	}
}
func TestSettingsPartialSaveRetainsUnwrittenChanges(t *testing.T) {
	session, _ := settingsFixture(t)
	if err := session.stage([]string{"hyperbricks", "mode"}, "hyperbricks.mode", ystr("debug"), false); err != nil {
		t.Fatal(err)
	}
	if err := session.stage([]string{"hyperbricks", "server", "port"}, "hyperbricks.server.port", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "9091"}, true); err != nil {
		t.Fatal(err)
	}
	calls := 0
	saved, err := session.saveWith(func(root string, file scaffoldFile) error {
		calls++
		if calls == 2 {
			return errors.New("write denied")
		}
		return writeScaffoldFile(root, file)
	})
	if err == nil || len(saved) != 1 || len(session.changes()) != 1 {
		t.Fatalf("saved=%v changes=%v err=%v", saved, session.changes(), err)
	}
	if _, err := session.save(); err != nil {
		t.Fatal(err)
	}
}
func TestSettingsDetectsSymlinkRetarget(t *testing.T) {
	session, root := settingsFixture(t)
	source := filepath.Join(root, "config/runtime.yaml")
	first := filepath.Join(root, "config/first.yaml")
	if err := os.Rename(source, first); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(first, source); err != nil {
		t.Skip(err)
	}
	session, err := openSettingsSession(root, session.entry)
	if err != nil {
		t.Fatal(err)
	}
	if err = session.stage([]string{"hyperbricks", "mode"}, "hyperbricks.mode", ystr("debug"), false); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(first)
	second := filepath.Join(root, "config/second.yaml")
	if err = os.WriteFile(second, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(source); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(second, source); err != nil {
		t.Fatal(err)
	}
	if _, err = session.save(); err == nil {
		t.Fatal("retarget accepted")
	}
}

func TestSettingsResolverEditKeepsExpression(t *testing.T) {
	session, root := settingsFixture(t)
	entry := filepath.Join(root, PackageConfigFileName)
	if err := os.WriteFile(entry, []byte("imports: [config/runtime.yaml]\nvars: {mode: development}\nhyperbricks:\n  mode: {var: mode}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	session, err := openSettingsSession(root, entry)
	if err != nil {
		t.Fatal(err)
	}
	model, err := newSettingsModel(session)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range model.allItems {
		item := row.(settingsItem)
		if item.key == "hyperbricks.mode" {
			if !item.resolver || len(item.choices) != 0 {
				t.Fatal("expression offered literal choices")
			}
			model.beginEdit(item, false)
			if err := model.apply(item.value); err != nil {
				t.Fatal(err)
			}
			if len(session.changes()) != 0 {
				t.Fatal("no-op expression edit rewrote source")
			}
			return
		}
	}
	t.Fatal("mode missing")
}
