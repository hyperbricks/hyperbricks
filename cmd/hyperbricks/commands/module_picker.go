package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type moduleItem struct {
	name string
}

func (i moduleItem) Title() string       { return i.name }
func (i moduleItem) Description() string { return "Module directory: " + i.name }
func (i moduleItem) FilterValue() string { return i.name }

type modulePickerModel struct {
	list     list.Model
	selected string
	canceled bool
}

func newModulePickerModel(items []list.Item, title string) modulePickerModel {
	const defaultWidth = 100
	const listHeight = 14

	delegate := list.NewDefaultDelegate()
	orange := lipgloss.Color("#FFA500")

	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(orange).Bold(true).BorderLeftForeground(orange)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(orange).BorderLeftForeground(orange)

	l := list.New(items, delegate, defaultWidth, listHeight)
	l.Title = title
	l.Styles.Title = l.Styles.Title.Background(orange)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()

	return modulePickerModel{list: l}
}

func (m modulePickerModel) Init() tea.Cmd {
	return nil
}

func (m modulePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			selected, ok := m.list.SelectedItem().(moduleItem)
			if ok {
				m.selected = selected.name
				return m, tea.Quit
			}
		case "ctrl+c", "q":
			m.canceled = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m modulePickerModel) View() string {
	return m.list.View()
}

func RunModulePicker(title string) (string, bool, error) {
	items, err := loadModuleItems()
	if err != nil {
		return "", false, err
	}

	program := tea.NewProgram(newModulePickerModel(items, title))
	finalModel, err := program.Run()
	if err != nil {
		return "", false, err
	}

	model, ok := finalModel.(modulePickerModel)
	if !ok {
		return "", false, fmt.Errorf("unexpected model type")
	}
	if model.canceled || model.selected == "" {
		return "", false, nil
	}

	return model.selected, true, nil
}

func loadModuleItems() ([]list.Item, error) {
	entries, err := os.ReadDir("./modules")
	if err != nil {
		return nil, fmt.Errorf("error reading ./modules directory: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	if len(names) == 0 {
		return nil, fmt.Errorf("no modules found in %s", filepath.Clean("./modules"))
	}

	items := make([]list.Item, 0, len(names))
	for _, name := range names {
		items = append(items, moduleItem{name: name})
	}

	return items, nil
}
