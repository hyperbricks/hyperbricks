package commands

import (
	"reflect"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

func settingsLabel(path []string) string {
	if len(path) == 0 {
		return "Settings"
	}
	key := path[len(path)-1]
	if len(path) == 2 {
		switch key {
		case "hooks":
			return "Lifecycle hooks"
		case "live":
			return "Live caching"
		case "logger":
			return "Logging"
		case "metadata":
			return "Package metadata"
		case "__imports":
			return "Configuration sources"
		}
	}
	words := []rune(strings.ReplaceAll(key, "_", " "))
	if len(words) > 0 {
		words[0] = unicode.ToUpper(words[0])
	}
	return string(words)
}

func (m *settingsModel) hasSection(path []string) bool {
	for _, row := range m.allItems {
		item := row.(settingsItem)
		if item.section && reflect.DeepEqual(item.path, path) {
			return true
		}
	}
	return false
}
func (m *settingsModel) currentSection() settingsItem {
	for _, row := range m.allItems {
		item := row.(settingsItem)
		if item.section && reflect.DeepEqual(item.path, m.location) {
			return item
		}
	}
	return settingsItem{readonly: true, description: "Choose a section or setting."}
}
func (m *settingsModel) rememberSelection() {
	if m.searching {
		return
	}
	if item, ok := m.list.SelectedItem().(settingsItem); ok {
		m.selections[settingsDiagnosticPath(m.location)] = item.key
	}
}
func (m *settingsModel) showSettingsLevel() {
	var visible []list.Item
	for _, row := range m.allItems {
		item := row.(settingsItem)
		if m.searching || len(item.path) == len(m.location)+1 && reflect.DeepEqual(item.path[:len(m.location)], m.location) {
			item.fullTitle = m.searching
			visible = append(visible, item)
		}
	}
	m.list.ResetFilter()
	m.list.SetItems(visible)
	m.list.Select(0)
	if m.searching {
		m.list.Title = "Search all settings"
		return
	}
	crumbs := []string{"Settings"}
	for i := 1; i < len(m.location); i++ {
		label := settingsLabel(m.location[:i+1])
		for _, row := range m.allItems {
			item := row.(settingsItem)
			if item.label != "" && reflect.DeepEqual(item.path, m.location[:i+1]) {
				label = item.label
				break
			}
		}
		crumbs = append(crumbs, label)
	}
	m.list.Title = strings.Join(crumbs, " › ")
	selected := m.selections[settingsDiagnosticPath(m.location)]
	for index, row := range visible {
		if row.(settingsItem).key == selected {
			m.list.Select(index)
			break
		}
	}
}
func (m *settingsModel) openSettingsItem(item settingsItem) tea.Cmd {
	if item.key == "" {
		return nil
	}
	m.rememberSelection()
	if item.section {
		if m.searching {
			m.selections[settingsDiagnosticPath(item.path[:len(item.path)-1])] = item.key
		}
		m.searching = false
		m.location = append([]string(nil), item.path...)
		m.showSettingsLevel()
		return nil
	}
	if m.searching {
		m.searching = false
		m.location = append([]string(nil), item.path[:len(item.path)-1]...)
		m.selections[settingsDiagnosticPath(m.location)] = item.key
		m.showSettingsLevel()
	}
	return m.beginEdit(item, false)
}
