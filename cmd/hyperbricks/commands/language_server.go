package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hyperbricks/hyperbricks/assets"
	"github.com/hyperbricks/hyperbricks/pkg/language"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/spf13/cobra"
)

// NewLanguageServerCommand creates the stdio Language Server Protocol entry
// point used by editor extensions.
func NewLanguageServerCommand() *cobra.Command {
	var stdio bool
	cmd := &cobra.Command{
		Use:   "language-server",
		Short: "Provide HyperBricks language support over the Language Server Protocol (LSP)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			Exit, ExitCode = true, 0
			if !stdio {
				ExitCode = 1
				return fmt.Errorf("language-server currently requires --stdio")
			}
			server := language.NewServer(cmd.InOrStdin(), cmd.OutOrStdout(), language.ServerOptions{
				Version:                  strings.TrimSpace(assets.VersionMD),
				ResolveRuntimeConnection: resolveLanguageRuntimeConnection,
			})
			if err := server.Serve(context.Background()); err != nil {
				ExitCode = 1
				return err
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&stdio, "stdio", true, "communicate with the editor over standard input and output")
	return cmd
}

// resolveLanguageRuntimeConnection loads the exact module/profile selected by
// the editor. Discovered credentials are used only for a literal loopback URL;
// remote authentication is intentionally unsupported by protocol version 1.
func resolveLanguageRuntimeConnection(_ context.Context, request language.RuntimeConnectionRequest) (language.RuntimeDiagnosticsConnection, error) {
	workspaceRoot := strings.TrimSpace(request.WorkspaceRoot)
	if workspaceRoot == "" {
		var err error
		workspaceRoot, err = os.Getwd()
		if err != nil {
			return language.RuntimeDiagnosticsConnection{}, fmt.Errorf("resolve language-server workspace: %w", err)
		}
	}
	workspaceRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return language.RuntimeDiagnosticsConnection{}, fmt.Errorf("resolve language-server workspace: %w", err)
	}

	module := strings.TrimSpace(request.Module)
	if module == "" {
		module = "default"
	}
	moduleRoot, err := resolveModuleRoot(module, workspaceRoot)
	if err != nil {
		return language.RuntimeDiagnosticsConnection{}, err
	}
	if !filepath.IsAbs(moduleRoot) {
		moduleRoot = filepath.Join(workspaceRoot, moduleRoot)
	}
	moduleRoot, err = filepath.Abs(moduleRoot)
	if err != nil {
		return language.RuntimeDiagnosticsConnection{}, fmt.Errorf("resolve module root: %w", err)
	}
	resolvedModuleRoot, err := filepath.EvalSymlinks(moduleRoot)
	if err != nil {
		return language.RuntimeDiagnosticsConnection{}, fmt.Errorf("resolve module root: %w", err)
	}

	config := strings.TrimSpace(request.Config)
	if config == "" {
		config = PackageConfigFileName
	}
	configPath, err := resolveModuleConfigPath(moduleRoot, config)
	if err != nil {
		return language.RuntimeDiagnosticsConnection{}, err
	}
	resolvedConfigPath, err := filepath.EvalSymlinks(configPath)
	if err != nil {
		return language.RuntimeDiagnosticsConnection{}, fmt.Errorf("resolve package configuration: %w", err)
	}
	configRelative, err := filepath.Rel(resolvedModuleRoot, resolvedConfigPath)
	if err != nil || configRelative == ".." || strings.HasPrefix(configRelative, ".."+string(filepath.Separator)) {
		return language.RuntimeDiagnosticsConnection{}, fmt.Errorf("package configuration must stay inside the selected module")
	}
	packageConfig, err := shared.LoadPackageConfigStrict(configPath, moduleRoot)
	if err != nil {
		return language.RuntimeDiagnosticsConnection{}, err
	}
	if packageConfig.Mode == shared.LIVE_MODE {
		return language.RuntimeDiagnosticsConnection{}, fmt.Errorf("%w: runtime diagnostics require a development or debug package profile; selected profile mode is live", language.ErrRuntimeDiagnosticsUnavailable)
	}

	runtimeURL := strings.TrimSpace(request.RuntimeURL)
	if runtimeURL == "" {
		runtimeURL = "http://localhost:" + strconv.Itoa(packageConfig.Server.Port)
	}
	runtimeURL, err = language.CanonicalRuntimeDiagnosticsURL(runtimeURL)
	if err != nil {
		return language.RuntimeDiagnosticsConnection{}, err
	}
	loopback, err := language.RuntimeDiagnosticsURLIsLoopback(runtimeURL)
	if err != nil {
		return language.RuntimeDiagnosticsConnection{}, err
	}
	auth := language.RuntimeDiagnosticsAuth{}
	credentials := packageConfig.Development.Dashboard.Credentials
	if loopback && credentials.Complete() {
		auth = language.RuntimeDiagnosticsAuth{
			Username: credentials.User,
			Password: credentials.Password,
			Mode:     language.RuntimeCredentialsAutomatic,
		}
	}
	connection := language.RuntimeDiagnosticsConnection{
		ModuleRoot: moduleRoot,
		ConfigPath: configPath,
		RuntimeURL: runtimeURL,
		Auth:       auth,
	}
	if packageConfig.Development.Dashboard.Enabled {
		connection.ErrorsURL = language.RuntimeDiagnosticsErrorsURL(runtimeURL)
	}
	return connection, nil
}
