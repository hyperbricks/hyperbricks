package commands

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/spf13/cobra"
)

type wizardOption struct{ value, label, description string }

func (i wizardOption) Title() string       { return i.label }
func (i wizardOption) Description() string { return i.description }
func (i wizardOption) FilterValue() string { return i.label + " " + i.description }

type wizardStep struct {
	key, title, help, kind, initial string
	options                         []wizardOption
	validate                        func(string) error
}
type authoringWizard struct {
	title                  string
	values                 map[string]string
	steps                  func(map[string]string) []wizardStep
	preview                func(map[string]string) (interface{}, string, error)
	index, width, height   int
	list                   list.Model
	input                  textinput.Model
	text                   textarea.Model
	view                   viewport.Model
	checked                map[string]bool
	error                  string
	review, done, canceled bool
	result                 interface{}
}

func newAuthoringWizard(steps func(map[string]string) []wizardStep, preview func(map[string]string) (interface{}, string, error)) *authoringWizard {
	var previous map[string]string
	var cached []wizardStep
	// Source discovery depends on answers, not cursor movement or blink events.
	cachedSteps := func(values map[string]string) []wizardStep {
		if cached == nil || !maps.Equal(previous, values) {
			previous = maps.Clone(values)
			cached = steps(values)
		}
		return cached
	}
	m := &authoringWizard{title: "Scaffold", values: map[string]string{}, steps: cachedSteps, preview: preview, width: 80, height: 24}
	m.enter()
	return m
}
func (m *authoringWizard) current() wizardStep {
	steps := m.steps(m.values)
	if m.index >= len(steps) {
		m.index = len(steps) - 1
	}
	return steps[m.index]
}
func (m *authoringWizard) enter() {
	m.error = ""
	s := m.current()
	value, ok := m.values[s.key]
	if s.kind == "select" && ok && value != "" && !wizardHasOption(s.options, value) {
		// Retain selections while they remain valid, but never let a module
		// deleted during the flow keep stale dependent answers alive.
		delete(m.values, s.key)
		if s.key == "module" {
			m.clearDependents(s.key)
			delete(m.values, s.key)
		}
		value, ok = "", false
	}
	if !ok {
		value = s.initial
	}
	m.input = textinput.New()
	m.input.CharLimit = 0
	m.input.Width = max(20, m.width-8)
	m.input.SetValue(value)
	m.input.Focus()
	m.text = textarea.New()
	m.text.CharLimit = 0
	m.text.SetWidth(max(20, m.width-6))
	m.text.SetHeight(max(3, m.height-11))
	m.text.SetValue(value)
	m.text.Focus()
	m.checked = map[string]bool{}
	_ = json.Unmarshal([]byte(value), &m.checked)
	if m.checked == nil {
		m.checked = map[string]bool{}
	}
	items := []list.Item{}
	selected := 0
	for i, opt := range s.options {
		if opt.value == value {
			selected = i
		}
		if s.kind == "multi" {
			prefix := "[ ] "
			if m.checked[opt.value] {
				prefix = "[x] "
			}
			opt.label = prefix + opt.label
		}
		items = append(items, opt)
	}
	m.list = list.New(items, list.NewDefaultDelegate(), max(20, m.width-4), max(4, m.height-8))
	m.list.Title = s.title
	m.list.SetShowTitle(false)
	m.list.SetShowStatusBar(false)
	m.list.SetShowHelp(false)
	m.list.DisableQuitKeybindings()
	m.list.Select(selected)
}

func wizardHasOption(options []wizardOption, value string) bool {
	for _, option := range options {
		if option.value == value {
			return true
		}
	}
	return false
}
func (m *authoringWizard) Init() tea.Cmd { return textinput.Blink }
func (m *authoringWizard) value() string {
	s := m.current()
	switch s.kind {
	case "select":
		if v, ok := m.list.SelectedItem().(wizardOption); ok {
			return v.value
		}
		return ""
	case "multi":
		b, _ := json.Marshal(m.checked)
		return string(b)
	case "text":
		return m.text.Value()
	default:
		return m.input.Value()
	}
}
func (m *authoringWizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = max(24, size.Width)
		m.height = max(12, size.Height)
		if m.review {
			m.view.Width = max(20, m.width-4)
			m.view.Height = max(3, m.height-8)
		} else {
			m.input.Width = max(16, m.width-8)
			m.text.SetWidth(max(16, m.width-6))
			m.text.SetHeight(max(3, m.height-11))
			m.list.SetSize(max(20, m.width-4), max(3, m.height-8))
		}
		return m, nil
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		if key.String() == "ctrl+c" {
			m.canceled = true
			return m, tea.Quit
		}
		if m.review {
			switch key.String() {
			case "esc", "shift+tab":
				m.review = false
				m.enter()
				return m, nil
			case "enter":
				m.done = true
				return m, tea.Quit
			}
			var cmd tea.Cmd
			m.view, cmd = m.view.Update(msg)
			return m, cmd
		}
		s := m.current()
		if key.String() == "esc" && (s.kind == "select" || s.kind == "multi") && m.list.FilterState() != list.Unfiltered {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}
		if key.String() == "esc" || key.String() == "shift+tab" {
			m.values[s.key] = m.value()
			if m.index == 0 {
				// Ctrl+C is the explicit cancellation gesture. Reaching the first
				// step while navigating back must not discard the wizard.
				return m, nil
			}
			m.index--
			m.enter()
			return m, nil
		}
		if s.kind == "multi" && key.String() == " " && m.list.FilterState() != list.Filtering {
			if opt, ok := m.list.SelectedItem().(wizardOption); ok {
				m.checked[opt.value] = !m.checked[opt.value]
				for i, item := range m.list.Items() {
					entry := item.(wizardOption)
					if entry.value == opt.value {
						prefix := "[ ] "
						if m.checked[opt.value] {
							prefix = "[x] "
						}
						entry.label = prefix + strings.TrimPrefix(strings.TrimPrefix(entry.label, "[ ] "), "[x] ")
						m.list.SetItem(i, entry)
						break
					}
				}
			}
			return m, nil
		}
		next := key.String() == "ctrl+n" || (key.String() == "enter" && s.kind != "text")
		if next {
			if (s.kind == "select" || s.kind == "multi") && m.list.FilterState() == list.Filtering {
				var cmd tea.Cmd
				m.list, cmd = m.list.Update(msg)
				return m, cmd
			}
			v := m.value()
			if s.validate != nil {
				if err := s.validate(v); err != nil {
					m.error = err.Error()
					return m, nil
				}
			}
			previous := m.values[s.key]
			m.values[s.key] = v
			if (s.key == "category" || s.key == "type" || s.key == "module") && previous != "" && previous != v {
				m.clearDependents(s.key)
			}
			if m.index+1 >= len(m.steps(m.values)) {
				result, preview, err := m.preview(m.values)
				if err != nil {
					m.error = err.Error()
					return m, nil
				}
				m.result = result
				m.review = true
				m.view = viewport.New(max(20, m.width-4), max(3, m.height-8))
				m.view.SetContent(preview)
				m.error = ""
				return m, nil
			}
			m.index++
			m.enter()
			return m, textinput.Blink
		}
	}
	var cmd tea.Cmd
	switch m.current().kind {
	case "select", "multi":
		m.list, cmd = m.list.Update(msg)
	case "text":
		m.text, cmd = m.text.Update(msg)
	default:
		m.input, cmd = m.input.Update(msg)
	}
	return m, cmd
}
func (m *authoringWizard) clearDependents(key string) {
	keep := map[string]bool{"category": true, "type": key != "category", "module": true, "module-path": true}
	if key == "module" {
		keep["module-path"] = false
	}
	for k := range m.values {
		if !keep[k] {
			delete(m.values, k)
		}
	}
}
func (m *authoringWizard) View() string {
	if m.done || m.canceled {
		return ""
	}
	heading := lipgloss.NewStyle().Bold(true).Foreground(terminalAccent)
	if m.review {
		return heading.Render("Review files") + "\n\n" + m.view.View() + "\n\nEnter Create  |  Esc Back  |  Ctrl+C Cancel\n"
	}
	s := m.current()
	wrap := lipgloss.NewStyle().Width(max(20, m.width-4))
	help := "Enter Next  |  Esc Back  |  Ctrl+C Cancel"
	if s.kind == "text" {
		help = "Ctrl+N Next  |  Esc Back  |  Ctrl+C Cancel"
	}
	if s.kind == "multi" {
		help = "Space Toggle  |  Enter Next  |  / Filter  |  Esc Back"
	} else if s.kind == "select" {
		help = "Enter Next  |  / Filter  |  Esc Back  |  Ctrl+C Cancel"
	}
	header := wrap.Render(heading.Render(fmt.Sprintf("%s  %d/%d  %s", m.title, m.index+1, len(m.steps(m.values)), s.title))) + "\n" + wrap.Render(s.help) + "\n\n"
	footer := "\n" + wrap.Foreground(lipgloss.Color("#e15d55")).Render(m.error) + "\n" + wrap.Render(help) + "\n"
	available := max(3, m.height-lipgloss.Height(header)-lipgloss.Height(footer)+2)
	var content string
	switch s.kind {
	case "select", "multi":
		selectedText := ""
		if s.kind == "multi" {
			var selected []string
			for _, opt := range s.options {
				if m.checked[opt.value] {
					selected = append(selected, opt.label)
				}
			}
			selectedText = wrap.Render("Selected: "+strings.Join(selected, ", ")) + "\n"
			available -= lipgloss.Height(selectedText) - 1
		}
		m.list.SetHeight(max(3, available))
		content = selectedText + m.list.View()
	case "text":
		m.text.SetHeight(available)
		content = m.text.View()
	default:
		content = m.input.View()
	}
	return header + content + footer
}

func runAuthoringWizard(cmd *cobra.Command, m *authoringWizard) (interface{}, error) {
	in, ok := cmd.InOrStdin().(*os.File)
	if !ok || !term.IsTerminal(in.Fd()) || !logging.IsTerminal(cmd.ErrOrStderr()) {
		return nil, fmt.Errorf("interactive wizard requires a terminal; use hyperbricks author for non-interactive changes to an existing project")
	}
	final, err := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(cmd.ErrOrStderr()), tea.WithAltScreen()).Run()
	if err != nil {
		return nil, err
	}
	w := final.(*authoringWizard)
	if w.canceled || !w.done {
		return nil, nil
	}
	return w.result, nil
}

func moduleWizardSteps(initial scaffoldSpec, v map[string]string) []wizardStep {
	if initial.Module != "" {
		return nil
	}
	options := []wizardOption{}
	items, _ := loadModuleItems()
	for _, item := range items {
		m := item.(moduleItem)
		options = append(options, wizardOption{m.name, m.name, ""})
	}
	options = append(options, wizardOption{"@path", "Module directory...", ""})
	steps := []wizardStep{{key: "module", title: "Module", kind: "select", options: options, validate: func(s string) error {
		if s == "@path" {
			return nil
		}
		_, err := loadAuthoringModule(s, initial.Config)
		return err
	}}}
	if v["module"] == "@path" {
		steps = append(steps, wizardStep{key: "module-path", title: "Module directory", kind: "input", validate: func(s string) error { _, err := loadAuthoringModule(s, initial.Config); return err }})
	}
	return steps
}
func wizardModule(initial scaffoldSpec, v map[string]string) string {
	if initial.Module != "" {
		return initial.Module
	}
	if v["module"] == "@path" {
		return v["module-path"]
	}
	return v["module"]
}
func authoringFiles(root, suffix string) []wizardOption {
	var opts []wizardOption
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), suffix) {
			rel, _ := filepath.Rel(root, p)
			rel = filepath.ToSlash(rel)
			if strings.HasPrefix(rel, "spaces/") && d.Name() == "index.hyperbricks.yaml" {
				return nil
			}
			opts = append(opts, wizardOption{rel, rel, ""})
		}
		return nil
	})
	return opts
}

func scaffoldWizardRootNames(m *authoringModule, file string) (map[string]string, error) {
	if m == nil {
		return nil, fmt.Errorf("select a valid module first")
	}
	p, err := newScaffoldPlan(m)
	if err != nil {
		return nil, err
	}
	_, names, err := p.graph(false)
	if err != nil || file == "" {
		return names, err
	}
	// Include a selected nested file even when it has not been imported yet.
	target, err := authoringPath(m.Directories["hyperbricks"], file)
	if err != nil {
		return nil, err
	}
	raw, err := p.read(target)
	if os.IsNotExist(err) {
		return names, nil
	}
	if err != nil {
		return nil, err
	}
	doc, err := sourceYAML(raw)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(doc.Content[0].Content); i += 2 {
		name := doc.Content[0].Content[i].Value
		if name != "imports" && name != "vars" {
			names[name] = target
		}
	}
	return names, nil
}

func scaffoldWizardSteps(initial scaffoldSpec, v map[string]string) []wizardStep {
	steps := moduleWizardSteps(initial, v)
	steps = append(steps, wizardStep{key: "category", title: "Category", kind: "select", initial: "composite", options: []wizardOption{{"composite", "Composite", "Containers, templates, pages, and fragments"}, {"component", "Component", "Content, assets, data, and plugins"}}})
	category := v["category"]
	if category == "" {
		category = "composite"
	}
	types := []wizardOption{}
	starters, _ := scaffoldLibraryTypes(category)
	for _, starter := range starters {
		templates, _ := scaffoldLibraryTemplates()
		template := templates[starter]
		def, _ := scaffoldDefinition(template.Type)
		label := template.Type
		switch starter {
		case "template_inline":
			label = "template (inline)"
		case "template_file":
			label = "template (file)"
		case "markdown_inline":
			label = "markdown (inline)"
		case "markdown_file":
			label = "markdown (file)"
		}
		types = append(types, wizardOption{starter, label, def.Description})
	}
	steps = append(steps, wizardStep{key: "type", title: "Type", kind: "select", options: types})
	templates, _ := scaffoldLibraryTemplates()
	selected := templates[v["type"]]
	def, _ := scaffoldDefinition(selected.Type)
	kind := scaffoldTypeName(def)
	m, _ := loadAuthoringModule(wizardModule(initial, v), initial.Config)
	files := []wizardOption{{"@new", "New configuration file...", ""}}
	if m != nil {
		for _, file := range authoringFiles(m.Directories["hyperbricks"], ".hyperbricks.yaml") {
			if !strings.Contains(file.value, "/") {
				files = append(files, file)
			}
		}
	}
	steps = append(steps, wizardStep{key: "file-choice", title: "Configuration file", kind: "select", options: files, initial: "@new"})
	if v["file-choice"] == "@new" || v["file-choice"] == "" {
		steps = append(steps, wizardStep{key: "file", title: "New configuration file", kind: "input", initial: "components.hyperbricks.yaml", help: "Top-level file relative to the configured HyperBricks directory", validate: func(value string) error {
			if filepath.Base(value) != value || !strings.HasSuffix(value, ".hyperbricks.yaml") {
				return fmt.Errorf("use a top-level .hyperbricks.yaml filename; use hyperbricks author for new nested files")
			}
			return nil
		}})
	}
	file := v["file-choice"]
	if file == "@new" {
		file = v["file"]
	}
	names, _ := scaffoldWizardRootNames(m, file)
	name := scaffoldLibraryRootName(v["type"], kind)
	for suffix := 2; names[name] != ""; suffix++ {
		name = fmt.Sprintf("new_%s_%d", kind, suffix)
	}
	steps = append(steps, wizardStep{key: "name", title: "Root name", kind: "input", initial: name, help: "Adds a new named root; existing definitions stay unchanged.", validate: func(s string) error {
		if !scaffoldNamePattern.MatchString(s) || s == "imports" || s == "vars" {
			return fmt.Errorf("start with a letter; use letters, digits, underscores or hyphens; imports/vars are reserved")
		}
		// Recheck on Next: files may have changed while the wizard was open.
		names, err := scaffoldWizardRootNames(m, file)
		if err != nil {
			return err
		}
		if owner := names[s]; owner != "" {
			rel, _ := filepath.Rel(m.Directories["hyperbricks"], owner)
			return fmt.Errorf("root %s already exists in %s; choose a different root name", s, filepath.ToSlash(rel))
		}
		return nil
	}})
	if scaffoldRouteOwner(kind) {
		steps = append(steps, wizardStep{key: "route", title: "Route", kind: "input", help: "Optional. Leave empty for a reusable root; index serves /."})
	}
	if kind == "hypermedia" {
		steps = append(steps, wizardStep{key: "title", title: "Title", kind: "input", initial: v["name"]})
	}
	return steps
}
func wizardScaffoldSpec(initial scaffoldSpec, v map[string]string) scaffoldSpec {
	s := initial
	s.Module = wizardModule(initial, v)
	s.Category = v["category"]
	s.Starter = v["type"]
	templates, _ := scaffoldLibraryTemplates()
	s.Type = templates[s.Starter].Type
	s.File = v["file-choice"]
	if s.File == "@new" {
		s.File = v["file"]
	}
	s.Name = v["name"]
	s.Route = v["route"]
	s.Title = v["title"]
	return s
}
func runScaffoldWizard(cmd *cobra.Command, initial scaffoldSpec) (*scaffoldPlan, error) {
	m := newAuthoringWizard(func(v map[string]string) []wizardStep { return scaffoldWizardSteps(initial, v) }, func(v map[string]string) (interface{}, string, error) {
		p, err := prepareScaffoldLibrary(wizardScaffoldSpec(initial, v))
		if err != nil {
			return nil, "", err
		}
		var preview strings.Builder
		for _, f := range p.Files {
			fmt.Fprintf(&preview, "%s %s\n%s\n", strings.ToUpper(f.Action), f.Path, f.After)
		}
		fmt.Fprintf(&preview, "Generated from %s\n", scaffoldLibraryPath)
		return p, preview.String(), nil
	})
	result, err := runAuthoringWizard(cmd, m)
	if result == nil || err != nil {
		return nil, err
	}
	return result.(*scaffoldPlan), nil
}
