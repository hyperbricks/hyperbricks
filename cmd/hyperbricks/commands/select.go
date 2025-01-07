// commands/select.go
package commands

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// item implements the list.Item interface
type item struct {
	title string
	desc  string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

// NewSelectCommand creates the "select" subcommand
func NewSelectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "select",
		Short: "Select a .hyperbricks config file",
		Run: func(cmd *cobra.Command, args []string) {
			// IMPORTANT: Use extendedInitialModel() instead of initialModel().
			program := tea.NewProgram(extendedInitialModel())
			if err := program.Start(); err != nil {
				fmt.Printf("Error running program: %v\n", err)
				return
			}
		},
	}
	return cmd
}

// -------------------------------------------------------
// The Bubble Tea model
// -------------------------------------------------------
type model struct {
	list     list.Model
	quitting bool
}

// We’ll always return our extended model, which can handle the “enter” key.
func extendedInitialModel() model {
	var items []list.Item

	// Read .hyperbricks files from the current directory
	files, err := ioutil.ReadDir(".")
	if err != nil {
		fmt.Println("Error reading directory:", err)
		return model{}
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".hyperbricks") {
			name := strings.TrimSuffix(file.Name(), ".hyperbricks")
			items = append(items, item{
				title: name,
				desc:  "Configuration: " + name,
			})
		}
	}

	const defaultWidth = 100
	const listHeight = 14

	l := list.New(items, list.NewDefaultDelegate(), defaultWidth, listHeight)
	l.Title = "Select a .hyperbricks Configuration"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)

	return model{list: l}
}

// Init is run once when the program starts up.
func (m model) Init() tea.Cmd {
	return nil
}

// -------------------------------------------------------
// Handle selection
// -------------------------------------------------------
func (m model) HandleSelection() (string, bool) {
	selectedItem, ok := m.list.SelectedItem().(item)
	if !ok {
		return "", false
	}
	return selectedItem.title, true
}

// -------------------------------------------------------
// “Extended” Update and View
// -------------------------------------------------------
func (m model) ExtendedUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			fmt.Println("Enter key pressed!")
			selected, ok := m.HandleSelection()
			if ok {
				// Here, parse the selected configuration and execute the start command
				startCmd := NewStartCommand()
				startCmd.Flags().Set("config", selected)

				// Execute the command
				if err := startCmd.Execute(); err != nil {
					fmt.Printf("Error executing start command: %v\n", err)
				}
				return m, tea.Quit
			}
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) ExtendedView() string {
	if m.quitting {
		return "Goodbye!\n"
	}
	return m.list.View()
}

// -------------------------------------------------------
// The actual Update() and View() for the Bubble Tea program
// -------------------------------------------------------
// We delegate to ExtendedUpdate/ExtendedView so that the "enter" key works.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m.ExtendedUpdate(msg)
}

func (m model) View() string {
	return m.ExtendedView()
}
