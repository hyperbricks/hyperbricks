package commands

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func selectSettingsRow(t *testing.T, m *settingsModel, key string) settingsItem {
	t.Helper()
	for index, row := range m.list.Items() {
		item := row.(settingsItem)
		if item.key == key {
			m.list.Select(index)
			return item
		}
	}
	t.Fatalf("%s not visible under %v", key, m.location)
	return settingsItem{}
}
func settingsKey(m *settingsModel, key tea.KeyType) { m.Update(tea.KeyMsg{Type: key}) }
func settingsRune(m *settingsModel, key rune) {
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
}
func TestSettingsTreeNavigationRemembersSelection(t *testing.T) {
	session, _ := settingsFixture(t)
	m, err := newSettingsModel(session)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range m.list.Items() {
		if len(row.(settingsItem).path) != 2 {
			t.Fatal("root includes nested settings")
		}
	}
	selectSettingsRow(t, m, "hyperbricks.server")
	settingsKey(m, tea.KeyEnter)
	selectSettingsRow(t, m, "hyperbricks.server.routing")
	settingsKey(m, tea.KeyEnter)
	if m.list.Title != "Settings › Server › Routing" {
		t.Fatal(m.list.Title)
	}
	selectSettingsRow(t, m, "hyperbricks.server.routing.clean_urls")
	settingsKey(m, tea.KeyEnter)
	if m.mode != "choice" {
		t.Fatal(m.mode)
	}
	settingsKey(m, tea.KeyEsc)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if !strings.Contains(m.View(), "Enable internal clean-URL matching") {
		t.Fatal("setting description clipped", m.View())
	}
	if m.mode != "browse" || len(m.location) != 3 {
		t.Fatal("cancel left the section")
	}
	settingsKey(m, tea.KeyEsc)
	if selected := m.list.SelectedItem().(settingsItem); selected.key != "hyperbricks.server.routing" {
		t.Fatal("parent selection lost", selected.key)
	}
	settingsKey(m, tea.KeyEsc)
	if selected := m.list.SelectedItem().(settingsItem); selected.key != "hyperbricks.server" {
		t.Fatal("root selection lost", selected.key)
	}
	settingsKey(m, tea.KeyEsc)
	if len(m.location) != 1 || len(session.changes()) != 0 {
		t.Fatal("root escape changed state")
	}
}
func TestSettingsTreeEditReviewAndEmptyTaskList(t *testing.T) {
	session, _ := settingsFixture(t)
	m, err := newSettingsModel(session)
	if err != nil {
		t.Fatal(err)
	}
	selectSettingsRow(t, m, "hyperbricks.server")
	settingsKey(m, tea.KeyEnter)
	selectSettingsRow(t, m, "hyperbricks.server.port")
	settingsKey(m, tea.KeyEnter)
	if err := m.apply("9090"); err != nil {
		t.Fatal(err)
	}
	if len(m.location) != 2 || m.list.SelectedItem().(settingsItem).key != "hyperbricks.server.port" {
		t.Fatal("edit lost location")
	}
	if !strings.HasSuffix(session.changes()[0].Path, "runtime.yaml") {
		t.Fatal("edit lost source owner")
	}
	settingsRune(m, 'r')
	settingsKey(m, tea.KeyEsc)
	if len(m.location) != 2 || m.mode != "browse" {
		t.Fatal("review lost location")
	}
	settingsKey(m, tea.KeyEsc)
	selectSettingsRow(t, m, "hyperbricks.hooks")
	settingsKey(m, tea.KeyEnter)
	selectSettingsRow(t, m, "hyperbricks.hooks.finish")
	settingsKey(m, tea.KeyEnter)
	if len(m.list.Items()) != 0 {
		t.Fatal("empty phase has unexpected children")
	}
	settingsRune(m, 'a')
	if len(m.list.Items()) != 1 {
		t.Fatal("could not add in empty phase", m.message)
	}
	settingsKey(m, tea.KeyEnter)
	selectSettingsRow(t, m, "hyperbricks.hooks.finish[0].command")
	settingsKey(m, tea.KeyEsc)
	if m.list.SelectedItem().(settingsItem).label != "new-task-1" {
		t.Fatal("task label missing")
	}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	view := m.View()
	if !strings.Contains(view, "Esc back") || !strings.Contains(view, "Settings › Lifecycle hooks › Finish") {
		t.Fatal("navigation hidden", view)
	}
}
func TestSettingsTreeGlobalSearchAndReturn(t *testing.T) {
	session, _ := settingsFixture(t)
	m, err := newSettingsModel(session)
	if err != nil {
		t.Fatal(err)
	}
	selectSettingsRow(t, m, "hyperbricks.server")
	settingsKey(m, tea.KeyEnter)
	selectSettingsRow(t, m, "hyperbricks.server.port")
	settingsRune(m, '/')
	if !m.searching {
		t.Fatal("search not global")
	}
	selectSettingsRow(t, m, "hyperbricks.hooks.finish")
	settingsKey(m, tea.KeyEsc)
	if !reflect.DeepEqual(m.location, []string{"hyperbricks", "server"}) || m.list.SelectedItem().(settingsItem).key != "hyperbricks.server.port" {
		t.Fatal("search cancel lost location")
	}
	settingsRune(m, '/')
	item := selectSettingsRow(t, m, "hyperbricks.mode")
	m.openSettingsItem(item)
	if m.searching || len(m.location) != 1 || m.mode != "choice" {
		t.Fatal("search result did not navigate to owning section")
	}
	settingsKey(m, tea.KeyEsc)
	if m.list.SelectedItem().(settingsItem).key != "hyperbricks.mode" {
		t.Fatal("search result selection lost")
	}
}
