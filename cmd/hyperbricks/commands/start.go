package commands

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
)

type Config struct {
	Port int32 `json:"port"`
}

var (
	StartMode              bool
	StartWithProcesses     bool
	StartModule            string
	StartConfigPath        string
	StartRuntimeGateway    bool
	StartRuntimeDomain     string
	StartRuntimeHostSuffix string
	StartRuntimeResolver   string
	Port                   int32
	Production             bool
	Debug                  bool
)

func GetModule() string {
	return StartModule
}

// NewStartCommand creates the "start" subcommand
func NewStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start server",
		Example: "  hyperbricks start -m demo\n" +
			"  hyperbricks start -m ./modules/demo",
		Run: func(cmd *cobra.Command, args []string) {
			if StartModule == "" && !cmd.Flags().Changed("module") {
				StartModule = "default"
			}

			workingDirectory, err := os.Getwd()
			if err != nil {
				failf("Error resolving the current working directory: %v\n", err)
				Exit = true
				ExitCode = 1
				return
			}
			moduleRoot, err := resolveModuleRoot(StartModule, workingDirectory)
			if err != nil {
				failf("Invalid module selection %q: %v\n", StartModule, err)
				Exit = true
				ExitCode = 1
				return
			}
			StartModule = strings.TrimSpace(StartModule)
			ModuleRoot = moduleRoot
			ModuleConfigPath = ""

			if strings.TrimSpace(StartConfigPath) != "" {
				configPath, err := resolveModuleConfigPath(GetModuleRoot(), StartConfigPath)
				if err != nil {
					failf("Invalid module config path: %v\n", err)
					Exit = true
					ExitCode = 1
					return
				}
				ModuleConfigPath = configPath
			}

			configPath := GetModuleConfigPath()
			_, err = os.ReadFile(configPath)
			if err != nil {
				failf("Error reading module config %q: %v\n", configPath, err)
				Exit = true
				ExitCode = 1
				return
			}
			StartMode = true
		},
	}
	cmd.Flags().StringVarP(&StartModule, "module", "m", "default", "module name or directory path")
	_ = cmd.RegisterFlagCompletionFunc("module", completeModuleSelection)
	cmd.Flags().StringVar(&StartConfigPath, "config", "", "package config path relative to the selected module")
	cmd.Flags().BoolVar(&StartRuntimeGateway, "runtime-gateway", false, "Enable host-based runtime gateway before normal route rendering")
	cmd.Flags().StringVar(&StartRuntimeDomain, "runtime-domain", "", "Runtime host suffix to match, for example runtime.local")
	cmd.Flags().StringVar(&StartRuntimeHostSuffix, "runtime-host-suffix", "", "Flat runtime host suffix to match, for example -runtime.example.com")
	cmd.Flags().StringVar(&StartRuntimeResolver, "runtime-resolver", "", "Resolver endpoint used by the runtime gateway")
	cmd.Flags().Int32VarP(&Port, "port", "p", 8080, "port")
	cmd.Flags().BoolVarP(&Production, "production", "P", false, "set production mode")
	cmd.Flags().BoolVarP(&Debug, "debug", "d", false, "debug")
	cmd.Flags().BoolVar(&StartWithProcesses, "with-processes", false, "Run module development hooks and managed services (direct development/debug starts only)")
	return cmd
}

func completeModuleSelection(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if isModulePath(toComplete) {
		return nil, cobra.ShellCompDirectiveDefault
	}

	entries, err := os.ReadDir("modules")
	if err != nil {
		return nil, cobra.ShellCompDirectiveDefault
	}
	completions := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), toComplete) {
			completions = append(completions, entry.Name()+"\tmodule in ./modules")
		}
	}
	return completions, cobra.ShellCompDirectiveDefault
}
