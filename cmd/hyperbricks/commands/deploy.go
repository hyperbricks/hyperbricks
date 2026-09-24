package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type DeployService string

const (
	DeployServiceNone   DeployService = ""
	DeployServiceLocal  DeployService = "local"
	DeployServiceRemote DeployService = "remote"
)

var (
	DeployServiceMode DeployService
	DeployConfigPath  string
	DeployModule      string
	DeployDir         string
	DeployBuildID     string
)

// NewDeployCommand creates the deployment command family. Deployment runtime,
// local control-plane, remote control-plane, and config initialization all live
// below this one explicit command boundary.
func NewDeployCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Run and manage deployments",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newDeployInitCommand())
	cmd.AddCommand(newDeployRunCommand())
	cmd.AddCommand(newDeployLocalCommand())
	cmd.AddCommand(newDeployRemoteCommand())
	return cmd
}

func newDeployInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a neutral deployment configuration",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			if cmd.Flags().Changed("config") && strings.TrimSpace(DeployConfigPath) == "" {
				failf("--config cannot be empty")
				return
			}
			path := GetDeployConfigPath()
			if err := writeDeployInitConfig(path); err != nil {
				failf("Error creating deploy config: %v\n", err)
				return
			}
			fmt.Printf("Created %s.\n", path)
			fmt.Println()
			fmt.Println("The file can configure three roles:")
			fmt.Println("- local: local deployment interface and locally managed modules")
			fmt.Println("- client: outgoing remote deployment targets")
			fmt.Println("- remote: remote deployment interface, API, and managed modules")
			fmt.Println("Unused role sections may remain in the file or be removed.")
			fmt.Println()
			fmt.Println("No credentials or HMAC secrets have default values.")
			fmt.Println("Set the environment variables referenced by the roles you run:")
			fmt.Println("  local:  HB_DEPLOY_LOCAL_USER, HB_DEPLOY_LOCAL_PASSWORD")
			fmt.Println("  client: HB_DEPLOY_CLIENT_PRODUCTION_USER, HB_DEPLOY_CLIENT_PRODUCTION_PASSWORD,")
			fmt.Println("          HB_DEPLOY_CLIENT_PRODUCTION_HMAC_SECRET")
			fmt.Println("  remote: HB_DEPLOY_REMOTE_USER, HB_DEPLOY_REMOTE_PASSWORD,")
			fmt.Println("          HB_DEPLOY_REMOTE_HMAC_SECRET")
			fmt.Println()
			fmt.Println("Start the local interface:  hyperbricks deploy local")
			fmt.Println("Start the remote service:  hyperbricks deploy remote")
			Exit = true
		},
	}
	cmd.Flags().StringVar(&DeployConfigPath, "config", "", "deployment configuration file")
	return cmd
}

func newDeployRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run an extracted deployment build",
		Args:  cobra.NoArgs,
		Example: "  hyperbricks deploy run -m demo\n" +
			"  hyperbricks deploy run -m demo --build build-id",
		Run: func(cmd *cobra.Command, _ []string) {
			moduleInput := strings.TrimSpace(DeployModule)
			if moduleInput == "" {
				moduleInput = "default"
			}
			workingDirectory, err := os.Getwd()
			if err != nil {
				failf("Error resolving the current working directory: %v\n", err)
				return
			}
			selection, err := resolveModuleSelection(moduleInput, workingDirectory)
			if err != nil {
				failf("Invalid module selection %q: %v\n", moduleInput, err)
				return
			}
			module := selection.Name
			deployDir := strings.TrimSpace(DeployDir)
			if deployDir == "" {
				deployDir = "deploy"
			}

			runtimeDir, err := prepareDeployRuntime(module, deployDir, DeployBuildID)
			if err != nil {
				failf("Error preparing deploy runtime: %v\n", err)
				return
			}
			configPath := filepath.Join(runtimeDir, "package.hyperbricks.yaml")
			if _, err := os.ReadFile(configPath); err != nil {
				failf("Error reading module config %q: %v\n", configPath, err)
				return
			}

			DeployModule = module
			StartModule = module
			ModuleRoot = runtimeDir
			ModuleConfigPath = configPath
			StartMode = true
		},
	}
	cmd.Flags().StringVarP(&DeployModule, "module", "m", "default", "module name or directory path")
	_ = cmd.RegisterFlagCompletionFunc("module", completeModuleSelection)
	cmd.Flags().StringVar(&DeployBuildID, "build", "", "deploy build ID to run (defaults to current)")
	cmd.Flags().StringVar(&DeployDir, "deploy-dir", "deploy", "deploy directory containing module builds")
	cmd.Flags().Int32VarP(&Port, "port", "p", 8080, "port")
	cmd.Flags().BoolVarP(&Production, "production", "P", false, "set production mode")
	cmd.Flags().BoolVarP(&Debug, "debug", "d", false, "debug")
	return cmd
}

func newDeployLocalCommand() *cobra.Command {
	return newDeployServiceCommand(
		"local",
		"Start the local deployment interface",
		DeployServiceLocal,
	)
}

func newDeployRemoteCommand() *cobra.Command {
	return newDeployServiceCommand(
		"remote",
		"Start the remote deployment interface and API",
		DeployServiceRemote,
	)
}

func newDeployServiceCommand(use, short string, service DeployService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			if cmd.Flags().Changed("config") && strings.TrimSpace(DeployConfigPath) == "" {
				failf("--config cannot be empty")
				return
			}
			DeployServiceMode = service
			StartMode = true
		},
	}
	cmd.Flags().StringVar(&DeployConfigPath, "config", "", "deployment configuration file")
	return cmd
}

func writeDeployInitConfig(path string) error {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." || path == "" {
		return fmt.Errorf("deployment config path cannot be empty")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("%s already exists", path)
		}
		return err
	}
	if _, err := file.WriteString(deployInitTemplate()); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func deployInitTemplate() string {
	return `# HyperBricks deployment configuration.
# Each role owns the settings it consumes. Secret values are resolved from the
# environment; there are no built-in credentials or HMAC defaults.
deploy:
  local:
    bind: 127.0.0.1
    port: 9091
    modules_dir: modules
    build_root: deploy
    port_start: 8080
    logs_enabled: true
    credentials:
      user:
        env: HB_DEPLOY_LOCAL_USER
      password:
        env: HB_DEPLOY_LOCAL_PASSWORD

  client:
    target: production
    targets:
      production:
        api: https://deploy.example.com
        credentials:
          user:
            env: HB_DEPLOY_CLIENT_PRODUCTION_USER
          password:
            env: HB_DEPLOY_CLIENT_PRODUCTION_PASSWORD
        hmac_secret:
          env: HB_DEPLOY_CLIENT_PRODUCTION_HMAC_SECRET
        # Optional keyed HMAC mode:
        # key_id: production

  remote:
    bind: 127.0.0.1
    port: 9090
    root: deploy
    port_start: 8080
    logs_enabled: true
    credentials:
      user:
        env: HB_DEPLOY_REMOTE_USER
      password:
        env: HB_DEPLOY_REMOTE_PASSWORD
    hmac_secret:
      env: HB_DEPLOY_REMOTE_HMAC_SECRET
    auth:
      # Used only when a client sends X-HB-Key-ID.
      env_prefix: HB_DEPLOY_SECRET_
    # binary: /usr/local/bin/hyperbricks
`
}

func prepareDeployRuntime(module string, deployDir string, buildID string) (string, error) {
	archivePath, resolvedID, err := ResolveDeployArchive(module, deployDir, buildID)
	if err != nil {
		return "", err
	}
	return EnsureRuntimeExtracted(archivePath, deployDir, module, resolvedID)
}
