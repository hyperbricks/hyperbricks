package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/spaces"
	"github.com/spf13/cobra"
)

type spaceOptions struct{ Module, Config, Source, Name, Title, Route string }
type localSpacePlan struct {
	module  *authoringModule
	create  *spaces.CreatePlan
	preview *scaffoldPlan
}

func prepareLocalSpace(o spaceOptions) (*localSpacePlan, error) {
	m, err := loadAuthoringModule(o.Module, o.Config)
	if err != nil {
		return nil, err
	}
	c, err := spaces.NewCreator(m.Root, m.Directories, m.Config)
	if err != nil {
		return nil, err
	}
	p, err := c.Prepare(o.Source, o.Name, o.Title, o.Route)
	if err != nil {
		return nil, err
	}
	preview := &scaffoldPlan{Name: o.Name, Type: "space"}
	for _, f := range p.Files() {
		action := "create"
		if f.Before != "" {
			action = "modify"
		}
		preview.Files = append(preview.Files, scaffoldFile{Path: f.Path, Action: action, Before: f.Before, After: f.After})
	}
	preview.Command = spaceCommandLine(o)
	return &localSpacePlan{m, p, preview}, nil
}
func (p *localSpacePlan) apply() error {
	b, err := os.ReadFile(p.module.ConfigPath)
	if err != nil || !bytes.Equal(b, p.module.PackageBytes) {
		return fmt.Errorf("package changed since preview; prepare again")
	}
	return p.create.Apply()
}
func spaceCommandLine(o spaceOptions) string {
	parts := []string{"hyperbricks", "space"}
	for _, f := range [][2]string{{"module", o.Module}, {"config", o.Config}, {"source", o.Source}, {"name", o.Name}, {"title", o.Title}, {"route", o.Route}} {
		if f[1] != "" {
			parts = append(parts, "--"+f[0], "'"+strings.ReplaceAll(f[1], "'", "'\"'\"'")+"'")
		}
	}
	return strings.Join(parts, " ")
}
func NewSpaceCommand() *cobra.Command {
	o := spaceOptions{}
	var listSources, dry, jsonOutput bool
	cmd := &cobra.Command{Use: "space", Short: "Create an inheriting Space from an existing hypermedia source"}
	f := cmd.Flags()
	f.StringVarP(&o.Module, "module", "m", "", "module name or directory")
	f.StringVar(&o.Config, "config", PackageConfigFileName, "package profile relative to module")
	f.StringVar(&o.Source, "source", "", "existing hypermedia root to inherit")
	f.StringVar(&o.Name, "name", "", "instance root name")
	f.StringVar(&o.Title, "title", "", "instance title")
	f.StringVar(&o.Route, "route", "", "instance route")
	f.BoolVar(&listSources, "list", false, "list available hypermedia sources")
	f.BoolVar(&dry, "dry-run", false, "preview without writing")
	f.BoolVar(&jsonOutput, "json", false, "structured output")
	authoringCommandErrors(cmd, &jsonOutput)
	cmd.RunE = func(c *cobra.Command, args []string) error {
		Exit = true
		ExitCode = 0
		if len(args) != 0 {
			return scaffoldReport(c, nil, fmt.Errorf("space accepts named options only"), dry, jsonOutput)
		}
		if listSources {
			m, err := loadAuthoringModule(o.Module, o.Config)
			if err != nil {
				return scaffoldReport(c, nil, err, dry, jsonOutput)
			}
			creator, err := spaces.NewCreator(m.Root, m.Directories, m.Config)
			if err != nil {
				return scaffoldReport(c, nil, err, dry, jsonOutput)
			}
			sources, err := creator.Sources()
			if err != nil {
				return scaffoldReport(c, nil, err, dry, jsonOutput)
			}
			if jsonOutput {
				return json.NewEncoder(c.OutOrStdout()).Encode(sources)
			}
			for _, s := range sources {
				fmt.Fprintf(c.OutOrStdout(), "%s\t%s\n", s.Name, s.Title)
			}
			return nil
		}
		var p *localSpacePlan
		var err error
		if o.Source != "" || o.Name != "" || o.Route != "" || o.Title != "" || jsonOutput || dry || NonInteractive || os.Getenv("HB_NO_KEYBOARD") == "1" {
			p, err = prepareLocalSpace(o)
		} else {
			p, err = runSpaceWizard(c, o)
		}
		if err != nil {
			return scaffoldReport(c, nil, err, dry, jsonOutput)
		}
		if p == nil {
			fmt.Fprintln(c.OutOrStdout(), "Canceled; no files written.")
			return nil
		}
		if !dry {
			err = p.apply()
		}
		return scaffoldReport(c, p.preview, err, dry, jsonOutput)
	}
	return cmd
}
func runSpaceWizard(cmd *cobra.Command, o spaceOptions) (*localSpacePlan, error) {
	initial := scaffoldSpec{Module: o.Module, Config: o.Config}
	m := newAuthoringWizard(func(v map[string]string) []wizardStep {
		steps := moduleWizardSteps(initial, v)
		options := []wizardOption{}
		if module, err := loadAuthoringModule(wizardModule(initial, v), o.Config); err == nil {
			if creator, err := spaces.NewCreator(module.Root, module.Directories, module.Config); err == nil {
				if sources, err := creator.Sources(); err == nil {
					for _, s := range sources {
						options = append(options, wizardOption{s.Name, s.Name, s.Title})
					}
				}
			}
		}
		steps = append(steps, wizardStep{key: "source", title: "Hypermedia source", kind: "select", options: options, validate: func(s string) error {
			if s == "" {
				return fmt.Errorf("no source selected; check module/imports or create a hypermedia source first")
			}
			return nil
		}}, wizardStep{key: "name", title: "Space name", kind: "input"}, wizardStep{key: "route", title: "Space route", kind: "input", help: "Use index for /."}, wizardStep{key: "title", title: "Space title", kind: "input", initial: v["name"]})
		return steps
	}, func(v map[string]string) (interface{}, string, error) {
		opts := o
		opts.Module = wizardModule(initial, v)
		opts.Source = v["source"]
		opts.Name = v["name"]
		opts.Route = v["route"]
		opts.Title = v["title"]
		p, err := prepareLocalSpace(opts)
		if err != nil {
			return nil, "", err
		}
		var b strings.Builder
		for _, f := range p.preview.Files {
			fmt.Fprintf(&b, "%s %s\n%s\n", f.Action, f.Path, f.After)
		}
		fmt.Fprintf(&b, "Equivalent command:\n%s\n", p.preview.Command)
		return p, b.String(), nil
	})
	m.title = "Space"
	result, err := runAuthoringWizard(cmd, m)
	if result == nil || err != nil {
		return nil, err
	}
	return result.(*localSpacePlan), nil
}
