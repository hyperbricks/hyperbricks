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
	StartMode         bool
	StartModule       string
	StartDeploy       bool
	StartDeployDir    string
	StartBuildID      string
	StartDeployRemote bool
	StartDeployLocal  bool
	StartDeployInit   string
	Port              int32
	Production        bool
	Debug             bool
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
			if strings.TrimSpace(StartDeployInit) != "" {
				if err := writeDeployInitConfig(StartDeployInit); err != nil {
					fmt.Printf("Error creating deploy config: %v\n", err)
					Exit = true
					return
				}
				fmt.Println("Created deploy.hyperbricks.")
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

			if StartModule == "" {
				StartModule = "default"
			}

			if StartDeployRemote || StartDeployLocal {
				StartMode = true
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
	cmd.Flags().BoolVar(&StartDeployRemote, "deploy-remote", false, "Start deploy API daemon (remote)")
	cmd.Flags().BoolVar(&StartDeployLocal, "deploy-local", false, "Start local deploy dashboard")
	cmd.Flags().StringVar(&StartDeployInit, "deploy-init-config", "", "Create a default deploy.hyperbricks (local or remote)")
	cmd.Flags().Int32VarP(&Port, "port", "p", 8080, "port")
	cmd.Flags().BoolVarP(&Production, "production", "P", false, "set production mode")
	cmd.Flags().BoolVarP(&Debug, "debug", "d", false, "debug")
	return cmd
}

func writeDeployInitConfig(mode string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "local" && mode != "remote" {
		return fmt.Errorf("deploy-init-config must be 'local' or 'remote'")
	}

	path := "deploy.hyperbricks"
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
deploy {
  # shared HMAC secret (set via env)
  hmac_secret = {{ENV:HB_DEPLOY_SECRET}}

  remote {
    # enable deploy api daemon
    api_enabled = true
    api_bind = 127.0.0.1
    # api_bind controls exposure:
    # - localhost/LAN: use SSH tunnel or LAN access
    # - WAN: bind to public IP and put HTTPS in front (reverse proxy)
    api_port = 9090
    root = deploy
    port_start = 8080
    logs_enabled = true
    # binary = /usr/local/bin/hyperbricks
  }
}
`
	}

	return `# Deploy config (local build dashboard)
deploy {
  # shared HMAC secret (set via env)
  hmac_secret = {{ENV:HB_DEPLOY_SECRET}}

  remote {
    # used as defaults for push/sync
    api_enabled = true
    api_bind = 127.0.0.1
    # api_bind controls exposure:
    # - localhost/LAN: use SSH tunnel or LAN access
    # - WAN: bind to public IP and put HTTPS in front (reverse proxy)
    api_port = 9090
    root = deploy
    port_start = 8080
    logs_enabled = true
  }

  local {
    bind = 127.0.0.1
    port = 9091
    modules_dir = modules
    build_root = deploy
  }

  # push targets for build --push and deploy-local
  client {
    target = prod
    targets {
      prod {
        host = 192.168.2.35
        user = deploy
        port = 22
        root = /opt/hyperbricks/deploy
        api = http://192.168.2.35:9090
        # For WAN use, point api to your public HTTPS endpoint instead of SSH tunnel.
        # Use SSH keys for push (recommended, no passwords).
      }
    }
  }
}
`
}

func prepareDeployRuntime(module string, deployDir string, buildID string) (string, error) {
	archivePath, resolvedID, err := ResolveDeployArchive(module, deployDir, buildID)
	if err != nil {
		return "", err
	}
	return EnsureRuntimeExtracted(archivePath, deployDir, module, resolvedID)
}
