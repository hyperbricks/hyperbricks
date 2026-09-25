package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/spf13/cobra"
)

type scaffoldResult struct {
	Status string        `json:"status"`
	Plan   *scaffoldPlan `json:"plan,omitempty"`
	Error  string        `json:"error,omitempty"`
}

func authoringCommandErrors(cmd *cobra.Command, jsonOutput *bool) {
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		Exit = true
		ExitCode = 1
		if jsonOutput != nil && (*jsonOutput || containsString(os.Args, "--json")) {
			_ = json.NewEncoder(c.OutOrStdout()).Encode(scaffoldResult{Status: "error", Error: err.Error()})
			return reportedError{err}
		}
		return err
	})
	help := cmd.HelpFunc()
	cmd.SetHelpFunc(func(c *cobra.Command, args []string) { Exit = true; ExitCode = 0; help(c, args) })
}
func scaffoldReport(cmd *cobra.Command, p *scaffoldPlan, err error, dry, jsonOutput bool) error {
	Exit = true
	ExitCode = 0
	r := scaffoldResult{Status: "created", Plan: p}
	if dry {
		r.Status = "preview"
	}
	if err != nil {
		ExitCode = 1
		r.Status = "error"
		r.Error = err.Error()
	}
	if jsonOutput {
		if e := json.NewEncoder(cmd.OutOrStdout()).Encode(r); e != nil {
			ExitCode = 1
			fmt.Fprintln(cmd.ErrOrStderr(), e)
		}
		return nil
	}
	if err != nil {
		logging.WriteError(cmd.ErrOrStderr(), err)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s: %s (%s)\n", r.Status, p.Name, p.Type)
	for _, f := range p.Files {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s %s\n", f.Action, f.Path)
		if dry {
			fmt.Fprintln(cmd.OutOrStdout(), f.After)
		}
	}
	return nil
}

func scaffoldStarterFlag(typeName, source string) (string, error) {
	typeName = strings.ToLower(strings.TrimSpace(typeName))
	source = strings.ToLower(strings.TrimSpace(source))
	if typeName == "" {
		return "", fmt.Errorf("--type is required in non-interactive mode")
	}
	if source != "" && typeName != "template" && typeName != "markdown" {
		return "", fmt.Errorf("--source applies only to template and markdown")
	}
	if source == "" {
		source = "inline"
	}
	switch typeName {
	case "template", "markdown":
		if source != "inline" && source != "file" {
			return "", fmt.Errorf("--source must be inline or file")
		}
		return typeName + "_" + source, nil
	default:
		if source != "inline" {
			return "", fmt.Errorf("--source applies only to template and markdown")
		}
		return typeName, nil
	}
}

func scaffoldFastSpec(s scaffoldSpec) (scaffoldSpec, error) {
	if strings.TrimSpace(s.Module) == "" {
		return scaffoldSpec{}, fmt.Errorf("--module is required in non-interactive mode")
	}
	if strings.TrimSpace(s.File) == "" {
		return scaffoldSpec{}, fmt.Errorf("--file is required in non-interactive mode")
	}
	starter, err := scaffoldStarterFlag(s.Type, s.TemplateMode)
	if err != nil {
		return scaffoldSpec{}, err
	}
	templates, err := scaffoldLibraryTemplates()
	if err != nil {
		return scaffoldSpec{}, err
	}
	template, ok := templates[starter]
	if !ok {
		return scaffoldSpec{}, fmt.Errorf("unknown scaffold starter %q; use the interactive wizard or see HYPERBRICKS_TYPE_EXAMPLES.md", starter)
	}
	def, err := scaffoldDefinition(template.Type)
	if err != nil {
		return scaffoldSpec{}, err
	}
	if s.Category != "" && s.Category != scaffoldCategory(def) {
		return scaffoldSpec{}, fmt.Errorf("%s belongs to category %s", template.Type, scaffoldCategory(def))
	}
	s.Starter = starter
	s.Type = template.Type
	if s.Name == "" {
		m, err := loadAuthoringModule(s.Module, s.Config)
		if err != nil {
			return scaffoldSpec{}, err
		}
		names, err := scaffoldWizardRootNames(m, s.File)
		if err != nil {
			return scaffoldSpec{}, err
		}
		base := scaffoldLibraryRootName(starter, template.Type)
		s.Name = base
		for suffix := 2; names[s.Name] != ""; suffix++ {
			s.Name = fmt.Sprintf("%s_%d", base, suffix)
		}
	}
	if template.Type == "hypermedia" && strings.TrimSpace(s.Title) == "" {
		s.Title = s.Name
	}
	return s, nil
}

func NewScaffoldCommand() *cobra.Command {
	s := scaffoldSpec{}
	var dry, jsonOutput bool
	cmd := &cobra.Command{Use: "scaffold", Short: "Create a root composite or component from a starter"}
	cmd.Flags().StringVarP(&s.Module, "module", "m", "", "preselect a module name or directory")
	cmd.Flags().StringVar(&s.Config, "config", PackageConfigFileName, "package profile relative to module")
	cmd.Flags().StringVar(&s.Category, "category", "", "composite or component (non-interactive mode)")
	cmd.Flags().StringVar(&s.Type, "type", "", "starter type (non-interactive mode)")
	cmd.Flags().StringVar(&s.File, "file", "", "top-level YAML destination relative to hyperbricks directory")
	cmd.Flags().StringVar(&s.Name, "name", "", "root name; defaults to a unique starter name")
	cmd.Flags().StringVar(&s.Route, "route", "", "route for route-owning starters")
	cmd.Flags().StringVar(&s.Title, "title", "", "page title for hypermedia starters")
	cmd.Flags().StringVar(&s.TemplateMode, "source", "", "inline or file for template and markdown starters")
	cmd.Flags().BoolVar(&dry, "dry-run", false, "preview without writing")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "structured output")
	authoringCommandErrors(cmd, nil)
	cmd.RunE = func(c *cobra.Command, args []string) error {
		Exit = true
		ExitCode = 0
		if len(args) != 0 {
			return scaffoldReport(c, nil, fmt.Errorf("scaffold accepts named options only"), dry, jsonOutput)
		}
		flagMode := NonInteractive || os.Getenv("HB_NO_KEYBOARD") == "1" || dry || jsonOutput || s.Type != "" || s.File != "" || s.Name != "" || s.Route != "" || s.Title != "" || s.TemplateMode != "" || s.Category != ""
		if flagMode {
			fast, err := scaffoldFastSpec(s)
			if err != nil {
				return scaffoldReport(c, nil, err, dry, jsonOutput)
			}
			p, err := prepareScaffoldLibrary(fast)
			if err != nil {
				return scaffoldReport(c, nil, err, dry, jsonOutput)
			}
			if !dry {
				err = p.apply()
			}
			return scaffoldReport(c, p, err, dry, jsonOutput)
		}
		p, err := runScaffoldWizard(c, s)
		if err != nil {
			return scaffoldReport(c, nil, err, false, false)
		}
		if p == nil {
			fmt.Fprintln(c.OutOrStdout(), "Canceled; no files written.")
			return nil
		}
		return scaffoldReport(c, p, p.apply(), false, false)
	}
	return cmd
}
