package commands

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type excludeItem struct {
	name     string
	rel      string
	isDir    bool
	excluded bool
}

func (i excludeItem) Title() string {
	prefix := "[ ]"
	if i.excluded {
		prefix = "[x]"
	}
	name := i.name
	if i.isDir {
		name += "/"
	}
	return fmt.Sprintf("%s %s", prefix, name)
}

func (i excludeItem) Description() string { return "" }
func (i excludeItem) FilterValue() string { return i.name }

type excludePickerModel struct {
	list       list.Model
	root       string
	currentRel string
	excluded   map[string]bool
}

func newExcludePickerModel(root string) (excludePickerModel, error) {
	const defaultWidth = 100
	const listHeight = 18

	delegate := list.NewDefaultDelegate()
	orange := lipgloss.Color("#FFA500")
	gray := lipgloss.Color("#AAAAAA")
	white := lipgloss.Color("#FFFFFF")

	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(orange).Bold(true).BorderLeftForeground(orange)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(orange).BorderLeftForeground(orange)
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.Foreground(white)
	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.Foreground(gray)

	m := excludePickerModel{
		list:     list.New([]list.Item{}, delegate, defaultWidth, listHeight),
		root:     root,
		excluded: make(map[string]bool),
	}
	m.list.SetShowStatusBar(false)
	m.list.SetFilteringEnabled(false)
	m.list.SetShowHelp(true)
	m.list.Styles.Title = m.list.Styles.Title.Background(orange)

	if err := m.refreshList(); err != nil {
		return excludePickerModel{}, err
	}

	return m, nil
}

func (m excludePickerModel) Init() tea.Cmd {
	return nil
}

func (m excludePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			selected, ok := m.list.SelectedItem().(excludeItem)
			if ok && selected.isDir {
				m.currentRel = selected.rel
				_ = m.refreshList()
				return m, nil
			}
		case " ":
			selected, ok := m.list.SelectedItem().(excludeItem)
			if ok {
				selected.excluded = !selected.excluded
				if selected.excluded {
					m.excluded[selected.rel] = true
				} else {
					delete(m.excluded, selected.rel)
				}
				m.list.SetItem(m.list.Index(), selected)
			}
		case "esc":
			if m.currentRel == "" {
				return m, tea.Quit
			}
			parent := path.Dir(m.currentRel)
			if parent == "." {
				parent = ""
			}
			m.currentRel = parent
			_ = m.refreshList()
			return m, nil
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m excludePickerModel) View() string {
	hint := "\nSpace: toggle exclude  Enter: open folder  Esc: back/finish\n"
	return m.list.View() + hint
}

func (m *excludePickerModel) refreshList() error {
	absDir := filepath.Join(m.root, filepath.FromSlash(m.currentRel))
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return err
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return entries[i].Name() < entries[j].Name()
	})

	items := make([]list.Item, 0, len(entries))
	for _, entry := range entries {
		if entry.Name() == ".DS_Store" {
			continue
		}
		rel := filepath.ToSlash(filepath.Join(filepath.FromSlash(m.currentRel), entry.Name()))
		items = append(items, excludeItem{
			name:     entry.Name(),
			rel:      rel,
			isDir:    entry.IsDir(),
			excluded: m.excluded[rel],
		})
	}

	m.list.SetItems(items)

	titlePath := "/"
	if m.currentRel != "" {
		titlePath = "/" + m.currentRel
	}
	m.list.Title = fmt.Sprintf("Exclude from zip: %s", titlePath)

	return nil
}

func RunStaticExcludePicker(renderDir string) (string, error) {
	root := filepath.Clean(renderDir)
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("render directory not found: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("render path is not a directory: %s", root)
	}

	model, err := newExcludePickerModel(root)
	if err != nil {
		return "", err
	}

	program := tea.NewProgram(model)
	finalModel, err := program.Run()
	if err != nil {
		return "", err
	}

	final, ok := finalModel.(excludePickerModel)
	if !ok {
		return "", fmt.Errorf("unexpected model type")
	}

	excludes := make([]string, 0, len(final.excluded))
	for item := range final.excluded {
		excludes = append(excludes, item)
	}
	sort.Strings(excludes)
	return strings.Join(excludes, ","), nil
}
