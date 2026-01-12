package commands

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type buildIDItem struct {
	id        string
	desc      string
	isCurrent bool
}

func (i buildIDItem) Title() string {
	if i.isCurrent {
		return fmt.Sprintf("%s (current)", i.id)
	}
	return i.id
}

func (i buildIDItem) Description() string { return i.desc }
func (i buildIDItem) FilterValue() string { return i.id }

type buildIDPickerModel struct {
	list     list.Model
	selected string
	canceled bool
}

func newBuildIDPickerModel(items []list.Item, title string) buildIDPickerModel {
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

	return buildIDPickerModel{list: l}
}

func (m buildIDPickerModel) Init() tea.Cmd {
	return nil
}

func (m buildIDPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			selected, ok := m.list.SelectedItem().(buildIDItem)
			if ok {
				m.selected = selected.id
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

func (m buildIDPickerModel) View() string {
	return m.list.View()
}

func RunBuildIDPicker(title string, rows []buildIndexRow, current string) (string, bool, error) {
	items := make([]list.Item, 0, len(rows))
	for _, row := range rows {
		desc := fmt.Sprintf("%s | %s | %s", row.ModuleVersion, row.BuiltAt, row.Format)
		items = append(items, buildIDItem{
			id:        row.BuildID,
			desc:      desc,
			isCurrent: row.BuildID == current,
		})
	}

	if len(items) == 0 {
		return "", false, fmt.Errorf("no build IDs available")
	}

	program := tea.NewProgram(newBuildIDPickerModel(items, title))
	finalModel, err := program.Run()
	if err != nil {
		return "", false, err
	}

	model, ok := finalModel.(buildIDPickerModel)
	if !ok {
		return "", false, fmt.Errorf("unexpected model type")
	}
	if model.canceled || model.selected == "" {
		return "", false, nil
	}
	return model.selected, true, nil
}

func buildIndexRowsWithCurrentFirst(index buildIndex) []buildIndexRow {
	if index.Current == "" {
		return append([]buildIndexRow{}, index.Versions...)
	}

	rows := make([]buildIndexRow, 0, len(index.Versions))
	if currentRow, ok := findBuildIndex(index, index.Current); ok {
		rows = append(rows, currentRow)
	}
	for _, row := range index.Versions {
		if row.BuildID != index.Current {
			rows = append(rows, row)
		}
	}
	return rows
}
