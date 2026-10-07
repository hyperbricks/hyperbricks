package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v4"
)

func NewSettingsCommand() *cobra.Command {
	var module, config string
	cmd := &cobra.Command{Use: "settings", Short: "Inspect and edit package settings with source-aware saving", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireTerminal(); err != nil {
			return err
		}
		if module == "" {
			selected, ok, err := RunModulePicker("Select a module to configure")
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			module = selected
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		root, err := resolveModuleRoot(module, cwd)
		if err != nil {
			return err
		}
		root, err = filepath.Abs(root)
		if err != nil {
			return err
		}
		entry, err := authoringPath(root, config)
		if err != nil {
			return err
		}
		session, err := openSettingsSession(root, entry)
		if err != nil {
			return err
		}
		model, err := newSettingsModel(session)
		if err != nil {
			return err
		}
		_, err = tea.NewProgram(model, tea.WithOutput(os.Stderr), tea.WithAltScreen()).Run()
		return err
	}}
	cmd.Flags().StringVarP(&module, "module", "m", "", "Module name or directory (opens picker when omitted)")
	cmd.Flags().StringVar(&config, "config", PackageConfigFileName, "Package entry relative to the module")
	return cmd
}

type settingsItem struct {
	path                            []string
	key, description, source, value string
	effective                       string
	defaultValue                    string
	kind                            reflect.Kind
	choices                         []string
	readonly                        bool
	resolver                        bool
	section                         bool
	label                           string
	fullTitle                       bool
}

func (i settingsItem) Title() string {
	title := i.label
	if title == "" {
		title = settingsLabel(i.path)
	}
	if i.fullTitle {
		title = i.key
	}
	if i.section {
		title += " ›"
	} else if i.value != "" {
		title += "  · " + strings.Split(i.value, "\n")[0]
	}
	return title
}
func (i settingsItem) Description() string {
	if i.section {
		return "Enter to open"
	}
	return i.value + " · " + i.source
}
func (i settingsItem) FilterValue() string { return i.key + " " + i.description }

type settingsModel struct {
	session       *settingsSession
	allItems      []list.Item
	location      []string
	selections    map[string]string
	searching     bool
	defaults      *shared.Config
	list          list.Model
	input         textarea.Model
	view          viewport.Model
	mode          string
	editing       settingsItem
	override      bool
	choice        int
	width, height int
	message       string
}

func newSettingsModel(session *settingsSession) (*settingsModel, error) {
	m := &settingsModel{session: session, width: 90, height: 28, mode: "browse", location: []string{"hyperbricks"}, selections: map[string]string{}}
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)
	m.list = list.New(nil, delegate, 86, 14)
	m.list.Title = "Package settings"
	m.list.DisableQuitKeybindings()
	m.list.SetShowHelp(false)
	m.list.SetShowStatusBar(false)
	m.input = textarea.New()
	m.input.CharLimit = 0
	m.input.ShowLineNumbers = true
	m.view = viewport.New(86, 18)
	return m, m.refresh()
}
func (m *settingsModel) Init() tea.Cmd { return textarea.Blink }

func (m *settingsModel) refresh() error {
	raw, err := m.session.read(m.session.entry)
	if err != nil {
		return err
	}
	config, err := shared.ValidatePackageConfigBytesAt(raw, m.session.entry, m.session.root, m.session.read)
	if err != nil {
		return err
	}
	m.defaults, err = shared.ValidatePackageConfigBytesAt([]byte("hyperbricks: {}"), m.session.entry, m.session.root, m.session.read)
	if err != nil {
		return err
	}
	var items []list.Item
	var walk func(reflect.Type, reflect.Value, []string, string)
	walk = func(typ reflect.Type, value reflect.Value, path []string, description string) {
		if typ == reflect.TypeOf(time.Duration(0)) || typ == reflect.TypeOf(shared.CacheTime{}) {
			item := m.settingItem(path, description, reflect.String, value)
			items = append(items, item)
			return
		}
		switch typ.Kind() {
		case reflect.Struct:
			if len(path) > 1 {
				item := m.settingItem(path, description, reflect.Struct, value)
				item.section, item.readonly = true, true
				if _, err := strconv.Atoi(path[len(path)-1]); err == nil {
					item.kind, item.readonly = reflect.Map, false
					if name := value.FieldByName("Name"); name.IsValid() {
						item.label = name.String()
					}
				}
				items = append(items, item)
			}
			for i := 0; i < typ.NumField(); i++ {
				field := typ.Field(i)
				key := strings.Split(field.Tag.Get("mapstructure"), ",")[0]
				if !field.IsExported() || key == "" || key == "-" {
					continue
				}
				walk(field.Type, value.Field(i), append(append([]string(nil), path...), key), field.Tag.Get("description"))
			}
		case reflect.Slice:
			item := m.settingItem(path, description, reflect.Slice, value)
			item.section = typ.Elem().Kind() == reflect.Struct
			items = append(items, item)
			if typ.Elem().Kind() == reflect.Struct {
				for i := 0; i < value.Len(); i++ {
					walk(typ.Elem(), value.Index(i), append(append([]string(nil), path...), strconv.Itoa(i)), description)
				}
			}
		case reflect.Map:
			item := m.settingItem(path, description, reflect.Map, value)
			item.section = typ.Elem().Kind() == reflect.String
			if typ.Elem().Kind() != reflect.String {
				item.readonly = true
			}
			items = append(items, item)
			if typ.Elem().Kind() == reflect.String {
				keys := value.MapKeys()
				sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
				for _, key := range keys {
					walk(typ.Elem(), value.MapIndex(key), append(append([]string(nil), path...), key.String()), description)
				}
			}
		default:
			items = append(items, m.settingItem(path, description, typ.Kind(), value))
		}
	}
	walk(reflect.TypeOf(*config), reflect.ValueOf(*config), []string{"hyperbricks"}, "")
	items = append(items, settingsItem{path: []string{"hyperbricks", "__imports"}, key: "imports", label: "Configuration sources", section: true, readonly: true, description: "Inspect the entry package and its imports."})
	for index, path := range m.session.effective.Dependencies {
		rel, _ := filepath.Rel(m.session.root, path)
		items = append(items, settingsItem{path: []string{"hyperbricks", "__imports", strconv.Itoa(index)}, label: rel, key: "imports · " + rel, description: "Configuration source. Imports remain in declaration order; editing a setting updates its winning source definition. Import restructuring is performed in YAML.", source: rel, value: "source file", readonly: true})
	}
	m.rememberSelection()
	m.allItems = items
	m.searching = false
	for len(m.location) > 1 && !m.hasSection(m.location) {
		m.location = m.location[:len(m.location)-1]
	}
	m.showSettingsLevel()
	return nil
}
func settingsDiagnosticPath(path []string) string {
	var result string
	for _, part := range path {
		if _, err := strconv.Atoi(part); err == nil {
			result += "[" + part + "]"
		} else {
			if result != "" {
				result += "."
			}
			result += part
		}
	}
	return result
}
func (m *settingsModel) settingItem(path []string, description string, kind reflect.Kind, value reflect.Value) settingsItem {
	key := settingsDiagnosticPath(path)
	item := settingsItem{path: path, key: key, description: description, source: "built-in default", kind: kind}
	if value.IsValid() {
		item.effective = fmt.Sprint(value.Interface())
	}
	item.defaultValue = settingsDefaultValue(m.defaults, path)
	node := settingsNodeAt(m.session.effective.Unresolved, path)
	if origin, ok := m.session.effective.Origins[key]; ok {
		item.source, _ = filepath.Rel(m.session.root, origin.File)
	}
	if node != nil {
		raw, _ := yaml.Marshal(node)
		item.value = strings.TrimSpace(string(raw))
	} else {
		switch kind {
		case reflect.Slice:
			item.value = "[]"
		case reflect.Map:
			item.value = "{}"
		default:
			if value.IsValid() {
				item.value = fmt.Sprint(value.Interface())
			}
		}
	}
	if kind == reflect.Bool {
		item.choices = []string{"true", "false"}
	}
	if key == "hyperbricks.mode" {
		item.choices = []string{shared.DEVELOPMENT_MODE, shared.DEBUG_MODE, shared.LIVE_MODE}
	}
	// Resolver objects require an explicit expression edit, even for enum fields.
	if node != nil && node.Kind == yaml.MappingNode {
		item.choices = nil
		item.resolver = true
	}
	return item
}

func (m *settingsModel) beginEdit(item settingsItem, override bool) tea.Cmd {
	if item.key == "" {
		return nil
	}
	if item.readonly {
		m.message = "This section is inspected here; edit its application-owned structure in YAML."
		return nil
	}
	m.editing, m.override, m.choice = item, override, 0
	if len(item.choices) > 0 {
		m.mode = "choice"
		for i, v := range item.choices {
			if v == item.value {
				m.choice = i
			}
		}
		return nil
	}
	m.mode = "edit"
	m.input.SetValue(item.value)
	m.input.SetWidth(max(10, m.width-6))
	m.input.SetHeight(max(3, m.height-12))
	m.input.Focus()
	return textarea.Blink
}
func (m *settingsModel) apply(raw string) error {
	var value yaml.Node
	// Plain text fields are literal strings unless the user explicitly supplies
	// an expression/map or a quoted YAML scalar.
	if m.editing.kind == reflect.String && !m.editing.resolver && !strings.HasPrefix(strings.TrimSpace(raw), "{") && !strings.HasPrefix(strings.TrimSpace(raw), "\"") && !strings.HasPrefix(strings.TrimSpace(raw), "'") {
		value = *ystr(raw)
	} else {
		var doc yaml.Node
		decoder := yaml.NewDecoder(strings.NewReader(raw))
		if err := decoder.Decode(&doc); err != nil {
			return err
		}
		if len(doc.Content) != 1 {
			return fmt.Errorf("enter one YAML value")
		}
		var extra yaml.Node
		if err := decoder.Decode(&extra); err != io.EOF {
			return fmt.Errorf("enter exactly one YAML value")
		}
		value = *doc.Content[0]
	}
	if err := m.session.stage(m.editing.path, m.editing.key, &value, m.override); err != nil {
		return err
	}
	m.mode = "browse"
	m.message = "Change staged. Review before saving."
	return m.refresh()
}
func (m *settingsModel) review() {
	var text strings.Builder
	keys := make([]string, 0, len(m.session.intents))
	for key := range m.session.intents {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		origin := m.session.intents[key]
		action := "edit source definition"
		old, exists := m.session.snapshot.Origins[key]
		if !exists {
			action = "explicit value replacing default"
		} else if old.File != origin.File {
			action = "override in selected entry"
		}
		fmt.Fprintf(&text, "%s (%s)\nEffective before: %v\nEffective after: %v\n\n", key, action, settingsEffectiveAt(m.session.snapshot.Materialized, origin.Path), settingsEffectiveAt(m.session.effective.Materialized, origin.Path))
	}
	for _, file := range m.session.changes() {
		rel, _ := filepath.Rel(m.session.root, file.Path)
		fmt.Fprintf(&text, "%s\n\nBefore:\n%s\nAfter:\n%s\n", rel, file.Before, file.After)
	}
	if text.Len() == 0 {
		text.WriteString("No pending changes.")
	}
	m.view.SetContent(text.String())
	m.view.GotoTop()
	m.mode = "review"
}
func (m *settingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = size.Width, size.Height
		m.list.SetSize(max(10, size.Width-4), max(3, size.Height-12))
		m.view.Width = max(10, size.Width-4)
		m.view.Height = max(3, size.Height-7)
		m.input.SetWidth(max(10, size.Width-6))
		m.input.SetHeight(max(3, size.Height-12))
		return m, nil
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		if key.String() == "ctrl+c" {
			if len(m.session.changes()) > 0 {
				m.mode = "discard"
				return m, nil
			}
			return m, tea.Quit
		}
		switch m.mode {
		case "reload_discard":
			if key.String() == "y" {
				next, err := openSettingsSession(m.session.root, m.session.entry)
				if err != nil {
					m.message = err.Error()
				} else {
					m.session = next
					m.message = "Reloaded; pending edits discarded."
					_ = m.refresh()
				}
				m.mode = "browse"
			} else if key.String() == "n" || key.String() == "esc" {
				m.mode = "browse"
			}
			return m, nil
		case "discard":
			if key.String() == "y" {
				return m, tea.Quit
			}
			if key.String() == "esc" || key.String() == "n" {
				m.mode = "browse"
			}
			return m, nil
		case "choice":
			switch key.String() {
			case "esc":
				m.mode = "browse"
			case "up", "k":
				m.choice = (m.choice + len(m.editing.choices) - 1) % len(m.editing.choices)
			case "down", "j":
				m.choice = (m.choice + 1) % len(m.editing.choices)
			case "enter":
				if err := m.apply(m.editing.choices[m.choice]); err != nil {
					m.message = err.Error()
				}
			}
			return m, nil
		case "edit":
			if key.String() == "esc" {
				m.mode = "browse"
				return m, nil
			}
			if key.String() == "ctrl+s" {
				if err := m.apply(m.input.Value()); err != nil {
					m.message = err.Error()
				}
				return m, nil
			}
		case "review":
			if key.String() == "esc" {
				m.mode = "browse"
				return m, nil
			}
			if key.String() == "enter" {
				saved, err := m.session.save()
				if err != nil {
					m.message = err.Error()
					return m, nil
				}
				m.message = fmt.Sprintf("Saved %d file(s). Restart the application to apply package changes.", len(saved))
				m.mode = "browse"
				if err = m.refresh(); err != nil {
					m.message = err.Error()
				}
				return m, nil
			}
		case "browse":
			if key.String() == "esc" {
				if m.searching {
					m.searching = false
					m.showSettingsLevel()
				} else if len(m.location) > 1 {
					m.rememberSelection()
					m.location = m.location[:len(m.location)-1]
					m.showSettingsLevel()
				}
				return m, nil
			}
			if m.list.FilterState() == list.Filtering {
				break
			}
			item, _ := m.list.SelectedItem().(settingsItem)
			switch key.String() {
			case "q":
				if len(m.session.changes()) > 0 {
					m.mode = "discard"
					return m, nil
				}
				return m, tea.Quit
			case "/":
				if !m.searching {
					m.rememberSelection()
					m.searching = true
					m.showSettingsLevel()
				}
				var cmd tea.Cmd
				m.list, cmd = m.list.Update(msg)
				return m, cmd
			case "enter":
				return m, m.openSettingsItem(item)
			case "e":
				if item.key == "" {
					item = m.currentSection()
				}
				return m, m.beginEdit(item, false)
			case "o":
				return m, m.beginEdit(item, true)
			case "a":
				if item.kind != reflect.Slice {
					item = m.currentSection()
				}
				if err := m.addTask(item); err != nil {
					m.message = err.Error()
				}
				return m, nil
			case "[", "]":
				direction := -1
				if key.String() == "]" {
					direction = 1
				}
				if err := m.moveTask(item, direction); err != nil {
					m.message = err.Error()
				}
				return m, nil
			case "r":
				m.review()
				return m, nil
			case "d":
				if !item.readonly {
					if err := m.session.stage(item.path, item.key, nil, false); err != nil {
						m.message = err.Error()
					} else {
						m.message = "Definition removed in pending changes; inherited value/default is now shown."
						_ = m.refresh()
					}
				}
				return m, nil
			case "D":
				m.mode = "reload_discard"
				return m, nil
			case "ctrl+r":
				if _, err := m.session.reload(); err != nil {
					m.message = err.Error()
				} else {
					m.message = "Reloaded configuration; pending edits retained. Review their effective values."
					_ = m.refresh()
				}
				return m, nil
			}
		}
	}
	var cmd tea.Cmd
	switch m.mode {
	case "browse":
		m.list, cmd = m.list.Update(msg)
	case "edit":
		m.input, cmd = m.input.Update(msg)
	case "review":
		m.view, cmd = m.view.Update(msg)
	}
	return m, cmd
}
func (m *settingsModel) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(terminalAccent).Render("HyperBricks settings · " + filepath.Base(m.session.root))
	rel, _ := filepath.Rel(m.session.root, m.session.entry)
	header := fmt.Sprintf("%s\nPackage: %s · %d changed file(s)\n", title, rel, len(m.session.changes()))
	var body, help string
	switch m.mode {
	case "reload_discard":
		body = "Discard all pending edits and reload current files? y / n"
		help = "Esc keeps pending edits."
	case "discard":
		body = "Discard pending changes and exit? y / n"
		help = "Esc returns to editing."
	case "review":
		body = m.view.View()
		help = "Enter save · Esc back · ↑↓ scroll"
	case "choice", "edit":
		destination := m.editing.source
		if m.override || destination == "built-in default" {
			destination = rel
		}
		body = lipgloss.NewStyle().Width(max(10, m.width-4)).Render(m.editing.key+"\nSave to: "+destination+"\n"+m.editing.description) + "\n\n"
		if m.mode == "choice" {
			for i, value := range m.editing.choices {
				prefix := "  "
				if i == m.choice {
					prefix = "> "
				}
				body += prefix + value + "\n"
			}
			help = "↑↓ choose · Enter stage · Esc cancel"
		} else {
			body += m.input.View()
			help = "Ctrl+S stage value · Esc cancel · Collections use YAML; commands are argument lists."
		}
	default:
		item, _ := m.list.SelectedItem().(settingsItem)
		detail := "Defined in: " + item.source + "\nEffective: " + item.effective + " · Default: " + item.defaultValue + "\n" + item.description + "\nTakes effect after restart. Asset housekeeping is automatic."
		if item.section || item.key == "" {
			if item.key == "" {
				item = m.currentSection()
			}
			detail = item.description + "\nEnter opens a section; Esc returns to its parent."
			if !item.readonly && item.section {
				detail += "\ne edits the collection as YAML; a adds to a task/service list."
			}
		}
		body = m.list.View() + "\n" + lipgloss.NewStyle().Width(max(10, m.width-4)).Render(detail)
		help = "Enter open/edit · Esc back · / search all · r review/save · q quit\ne edit YAML · o override · d remove · a add task · [ ] reorder\nCtrl+R reload · D discard/reload"
	}
	footer := lipgloss.NewStyle().Width(max(1, m.width)).Render(m.message + "\n" + help)
	available := max(1, m.height-lipgloss.Height(header)-lipgloss.Height(footer)-1)
	body = lipgloss.NewStyle().MaxHeight(available).Render(body)
	content := header + body + "\n" + footer
	return lipgloss.NewStyle().MaxWidth(max(1, m.width)).MaxHeight(max(1, m.height)).Render(content)
}

func (m *settingsModel) addTask(item settingsItem) error {
	if item.kind != reflect.Slice || !(strings.Contains(item.key, ".hooks.") || item.key == "hyperbricks.development.services") {
		return fmt.Errorf("select a hook phase or services list to add a structured entry")
	}
	listNode := cloneSettingsNode(settingsNodeAt(m.session.effective.Unresolved, item.path))
	if listNode == nil {
		listNode = yseq()
	}
	for index := 1; index < 10000; index++ {
		task := ymap()
		yput(task, "name", ystr(fmt.Sprintf("new-task-%d", index)))
		command := yseq()
		command.Content = []*yaml.Node{ystr("echo"), ystr("ready")}
		yput(task, "command", command)
		if item.key == "hyperbricks.development.services" {
			command.Content = []*yaml.Node{ystr("python3"), ystr("-m"), ystr("http.server"), ystr("4319")}
			ready := ymap()
			yput(ready, "http", ystr("http://127.0.0.1:4319/"))
			yput(task, "ready", ready)
		}
		candidate := cloneSettingsNode(listNode)
		candidate.Content = append(candidate.Content, task)
		err := m.session.stage(item.path, item.key, candidate, false)
		if err != nil {
			if strings.Contains(err.Error(), "duplicates") {
				continue
			}
			return err
		}
		m.message = "Entry staged. Select its fields to configure it; no command has run."
		return m.refresh()
	}
	return fmt.Errorf("could not choose an unused task name")
}
func (m *settingsModel) moveTask(item settingsItem, direction int) error {
	if len(item.path) == 0 {
		return fmt.Errorf("select a task/service entry")
	}
	index, err := strconv.Atoi(item.path[len(item.path)-1])
	if err != nil {
		return fmt.Errorf("select a task/service entry, not one of its fields")
	}
	path := item.path[:len(item.path)-1]
	listNode := cloneSettingsNode(settingsNodeAt(m.session.effective.Unresolved, path))
	if listNode == nil || listNode.Kind != yaml.SequenceNode || index+direction < 0 || index+direction >= len(listNode.Content) {
		return fmt.Errorf("entry is already at that end of its list")
	}
	listNode.Content[index], listNode.Content[index+direction] = listNode.Content[index+direction], listNode.Content[index]
	if err := m.session.stage(path, settingsDiagnosticPath(path), listNode, false); err != nil {
		return err
	}
	m.message = "Task order changed in pending edits."
	return m.refresh()
}

func settingsDefaultValue(config *shared.Config, path []string) string {
	value := reflect.ValueOf(config).Elem()
	for _, part := range path[1:] {
		if value.Kind() == reflect.Struct {
			found := false
			for i := 0; i < value.NumField(); i++ {
				if strings.Split(value.Type().Field(i).Tag.Get("mapstructure"), ",")[0] == part {
					value = value.Field(i)
					found = true
					break
				}
			}
			if !found {
				return "not defined"
			}
		} else if value.Kind() == reflect.Map && value.Type().Key().Kind() == reflect.String {
			value = value.MapIndex(reflect.ValueOf(part))
			if !value.IsValid() {
				return "not defined"
			}
		} else {
			return "not defined"
		}
	}
	if !value.IsValid() || !value.CanInterface() {
		return "not defined"
	}
	return fmt.Sprint(value.Interface())
}

func settingsEffectiveAt(root interface{}, path []string) interface{} {
	current := root
	for _, part := range path {
		switch value := current.(type) {
		case map[string]interface{}:
			next, ok := value[part]
			if !ok {
				return "built-in default / not defined"
			}
			current = next
		case []interface{}:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(value) {
				return "not defined"
			}
			current = value[index]
		default:
			return "not defined"
		}
	}
	return current
}
