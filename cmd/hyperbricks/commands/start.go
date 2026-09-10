package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type Config struct {
	Port int32 `json:"port"`
}

var (
	StartMode              bool
	StartModule            string
	StartConfigPath        string
	StartDeploy            bool
	StartDeployDir         string
	StartBuildID           string
	StartDeployRemote      bool
	StartDeployLocal       bool
	StartDeployInit        string
	StartRuntimeGateway    bool
	StartRuntimeDomain     string
	StartRuntimeHostSuffix string
	StartRuntimeResolver   string
	Port                   int32
	Production             bool
	Debug                  bool
)

const DeployConfigFileName = "deploy.hyperbricks.yaml"
const DefaultDeploySecretEnvPrefix = "HB_DEPLOY_SECRET_"

func GetModule() string {
	return StartModule
}

func NewDeployDaemonCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy-daemon",
		Short: "Start the remote deploy daemon",
		Run: func(cmd *cobra.Command, args []string) {
			if strings.TrimSpace(os.Getenv("HB_DEPLOY_CONFIG")) == "" {
				if _, err := os.Stat(DeployConfigFileName); os.IsNotExist(err) {
					if err := os.WriteFile(DeployConfigFileName, []byte(deployInitTemplate("remote")), 0644); err != nil {
						fmt.Printf("Error creating deploy config: %v\n", err)
						Exit = true
						return
					}
					fmt.Printf("Created %s.\n\n", DeployConfigFileName)
					fmt.Println("Next steps:")
					fmt.Println("1. In Composer, create a deploy target for this project.")
					fmt.Println("2. Copy the generated deploy secret.")
					fmt.Println("3. Set it on this server using the env var Composer shows, for example:")
					fmt.Println("")
					fmt.Println("   export HB_DEPLOY_SECRET_OWNER_EXAMPLE_TEST_TEST_PROD=\"hbd_...\"")
					fmt.Println("")
					fmt.Println("4. Start again:")
					fmt.Println("")
					fmt.Println("   hyperbricks deploy-daemon")
					Exit = true
					return
				} else if err != nil {
					fmt.Printf("Error checking deploy config: %v\n", err)
					Exit = true
					return
				}
			}

			StartDeployRemote = true
			StartMode = true
		},
	}
	return cmd
}

// NewStartCommand creates the "start" subcommand
func NewStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start server",
		Example: "  hyperbricks start -m demo\n" +
			"  hyperbricks start -m ./modules/demo",
		Run: func(cmd *cobra.Command, args []string) {
			if strings.TrimSpace(StartDeployInit) != "" {
				if err := writeDeployInitConfig(StartDeployInit); err != nil {
					fmt.Printf("Error creating deploy config: %v\n", err)
					Exit = true
					return
				}
				fmt.Printf("Created %s.\n", DeployConfigFileName)
				Exit = true
				return
			}

			if StartDeploy && cmd.Flags().NFlag() == 1 {
				RunDeployStartWizard()
				if Exit {
					return
				}
			}

			if StartDeployRemote && StartDeployLocal {
				fmt.Println("Use only one of --deploy-remote or --deploy-local.")
				Exit = true
				return
			}

			config := Config{
				Port: Port,
			}

			if StartModule == "" && !cmd.Flags().Changed("module") {
				StartModule = "default"
			}

			if StartDeployRemote || StartDeployLocal {
				StartMode = true
				return
			}

			if StartDeploy && strings.TrimSpace(StartConfigPath) != "" {
				fmt.Println("Use --config only when starting a module directly, not with --deploy.")
				Exit = true
				ExitCode = 1
				return
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
				ModuleConfigPath = filepath.Join(runtimeDir, "package.hyperbricks.yaml")
			} else {
				workingDirectory, err := os.Getwd()
				if err != nil {
					fmt.Printf("Error resolving the current working directory: %v\n", err)
					Exit = true
					ExitCode = 1
					return
				}
				moduleRoot, err := resolveDirectStartModuleRoot(StartModule, workingDirectory)
				if err != nil {
					fmt.Printf("Invalid module selection %q: %v\n", StartModule, err)
					Exit = true
					ExitCode = 1
					return
				}
				StartModule = strings.TrimSpace(StartModule)
				ModuleRoot = moduleRoot
				ModuleConfigPath = ""
			}

			if strings.TrimSpace(StartConfigPath) != "" {
				configPath, err := resolveModuleConfigPath(GetModuleRoot(), StartConfigPath)
				if err != nil {
					fmt.Printf("Invalid module config path: %v\n", err)
					Exit = true
					ExitCode = 1
					return
				}
				ModuleConfigPath = configPath
			}

			configPath := GetModuleConfigPath()
			data, err := os.ReadFile(configPath)
			if err != nil {
				fmt.Printf("Error reading module config %q: %v\n", configPath, err)
				Exit = true
				ExitCode = 1
				return
			}
			if err := json.Unmarshal(data, &config); err != nil {
				StartMode = true
				return
			}

			fmt.Printf("Starting server with config: %s on port: %d\n", configPath, config.Port)
		},
	}
	cmd.Flags().StringVarP(&StartModule, "module", "m", "default", "module name or directory path")
	_ = cmd.RegisterFlagCompletionFunc("module", completeStartModule)
	cmd.Flags().StringVar(&StartConfigPath, "config", "", "package config path relative to the selected module")
	cmd.Flags().BoolVar(&StartDeploy, "deploy", false, "Start server from the deploy folder using the current build")
	cmd.Flags().StringVar(&StartDeployDir, "deploy-dir", "deploy", "deploy directory containing module builds")
	cmd.Flags().StringVar(&StartBuildID, "build", "", "Deploy build ID to start (defaults to current)")
	cmd.Flags().BoolVar(&StartDeployRemote, "deploy-remote", false, "Start deploy API daemon (remote)")
	cmd.Flags().BoolVar(&StartDeployLocal, "deploy-local", false, "Start local deploy dashboard")
	cmd.Flags().StringVar(&StartDeployInit, "deploy-init-config", "", "Create a default deploy.hyperbricks.yaml (local or remote)")
	cmd.Flags().BoolVar(&StartRuntimeGateway, "runtime-gateway", false, "Enable host-based runtime gateway before normal route rendering")
	cmd.Flags().StringVar(&StartRuntimeDomain, "runtime-domain", "", "Runtime host suffix to match, for example runtime.local")
	cmd.Flags().StringVar(&StartRuntimeHostSuffix, "runtime-host-suffix", "", "Flat runtime host suffix to match, for example -runtime.example.com")
	cmd.Flags().StringVar(&StartRuntimeResolver, "runtime-resolver", "", "Resolver endpoint used by the runtime gateway")
	cmd.Flags().Int32VarP(&Port, "port", "p", 8080, "port")
	cmd.Flags().BoolVarP(&Production, "production", "P", false, "set production mode")
	cmd.Flags().BoolVarP(&Debug, "debug", "d", false, "debug")
	return cmd
}

func completeStartModule(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
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

func writeDeployInitConfig(mode string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "local" && mode != "remote" {
		return fmt.Errorf("deploy-init-config must be 'local' or 'remote'")
	}

	path := DeployConfigFileName
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	} else if !os.IsNotExist(err) {
		return err
	}

	template := deployInitTemplate(mode)
	return os.WriteFile(path, []byte(template), 0644)
}

func deployInitTemplate(mode string) string {
	if mode == "remote" {
		return `# Deploy config (remote runtime API)
deploy:
  # Optional admin/dashboard HMAC secret. Composer deploys can use per-module env secrets below.
  hmac_secret:
    env: HB_DEPLOY_ADMIN_SECRET

  remote:
    # Enable deploy API daemon.
    api_enabled: true
    api_bind: 127.0.0.1
    # api_bind controls exposure:
    # - localhost/LAN: use direct local or private-network access
    # - WAN: bind to public IP and put HTTPS in front (reverse proxy)
    api_port: 9090
    root: deploy
    port_start: 8080
    logs_enabled: true
    auth:
      # Composer shows the exact env var name for each deploy target:
      # HB_DEPLOY_SECRET_<NORMALIZED_MODULE>_<NORMALIZED_KEY_ID>
      env_prefix: HB_DEPLOY_SECRET_
    # binary: /usr/local/bin/hyperbricks
`
	}

	return `# Deploy config (local build dashboard)
deploy:
  # Shared HMAC secret. Prefer setting HB_DEPLOY_SECRET in the service environment.
  hmac_secret:
    env: HB_DEPLOY_SECRET

  remote:
    # Used as defaults for push/sync.
    api_enabled: true
    api_bind: 127.0.0.1
    # api_bind controls exposure:
    # - localhost/LAN: use direct local or private-network access
    # - WAN: bind to public IP and put HTTPS in front (reverse proxy)
    api_port: 9090
    root: deploy
    port_start: 8080
    logs_enabled: true

  local:
    bind: 127.0.0.1
    port: 9091
    modules_dir: modules
    build_root: deploy

  # HTTP deploy targets for build --push and deploy-local.
  client:
    target: prod
    targets:
      prod:
        api: https://deploy.example.com
        # Optional. If set, the client signs with X-HB-Key-ID and reads:
        # HB_DEPLOY_SECRET_<NORMALIZED_MODULE>_<NORMALIZED_KEY_ID>
        key_id: prod
`
}

func prepareDeployRuntime(module string, deployDir string, buildID string) (string, error) {
	archivePath, resolvedID, err := ResolveDeployArchive(module, deployDir, buildID)
	if err != nil {
		return "", err
	}
	return EnsureRuntimeExtracted(archivePath, deployDir, module, resolvedID)
}
