package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

type Config struct {
	Port int32 `json:"port"`
}

var (
	StartMode      bool
	StartModule    string
	StartDeploy    bool
	StartDeployDir string
	StartBuildID   string
	Port           int32
	Production     bool
	Debug          bool
)

func GetModule() string {
	return StartModule
}

// NewStartCommand creates the "start" subcommand
func NewStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start server",
		Run: func(cmd *cobra.Command, args []string) {
			if StartDeploy && cmd.Flags().NFlag() == 1 {
				RunDeployStartWizard()
				if Exit {
					return
				}
			}

			config := Config{
				Port: Port,
			}

			if StartModule == "" {
				StartModule = "default"
			}

			if StartDeploy {
				if StartDeployDir == "" {
					StartDeployDir = "deploy"
				}
				runtimeDir, err := prepareDeployRuntime(StartModule, StartDeployDir, StartBuildID)
				if err != nil {
					fmt.Printf("Error preparing deploy runtime: %v\n", err)
					Exit = true
					return
				}
				ModuleRoot = runtimeDir
				ModuleConfigPath = filepath.Join(runtimeDir, "package.hyperbricks")
			}

			configPath := GetModuleConfigPath()
			data, err := os.ReadFile(configPath)
			if err != nil {
				fmt.Printf("Error reading config file: %v\n", err)
				Exit = true
				return
			}
			if err := json.Unmarshal(data, &config); err != nil {
				StartMode = true
				return
			}

			fmt.Printf("Starting server with config: %s on port: %d\n", configPath, config.Port)
		},
	}
	cmd.Flags().StringVarP(&StartModule, "module", "m", "default", "module in the modules dorectory")
	cmd.Flags().BoolVar(&StartDeploy, "deploy", false, "Start server from the deploy folder using the current build")
	cmd.Flags().StringVar(&StartDeployDir, "deploy-dir", "deploy", "deploy directory containing module builds")
	cmd.Flags().StringVar(&StartBuildID, "build", "", "Deploy build ID to start (defaults to current)")
	cmd.Flags().Int32VarP(&Port, "port", "p", 8080, "port")
	cmd.Flags().BoolVarP(&Production, "production", "P", false, "set production mode")
	cmd.Flags().BoolVarP(&Debug, "debug", "d", false, "debug")
	return cmd
}

func prepareDeployRuntime(module string, deployDir string, buildID string) (string, error) {
	archivePath, resolvedID, err := ResolveDeployArchive(module, deployDir, buildID)
	if err != nil {
		return "", err
	}
	return EnsureRuntimeExtracted(archivePath, deployDir, module, resolvedID)
}
