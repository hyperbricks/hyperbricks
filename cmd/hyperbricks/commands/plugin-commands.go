package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"text/tabwriter"
	"unicode"

	"github.com/Masterminds/semver/v3"
	"github.com/hyperbricks/hyperbricks/assets"
	"github.com/spf13/cobra"
)

// PluginMeta describes one plugin version's manifest.
type PluginMeta struct {
	Plugin                string   `json:"plugin"`
	Version               string   `json:"version"`
	Source                string   `json:"source"`
	Runtime               string   `json:"runtime,omitempty"`
	Binary                string   `json:"binary,omitempty"`
	CompatibleHyperbricks []string `json:"compatible_hyperbricks"`
	Description           string   `json:"description"`
}

var (
	RequestedHyperbricksVersion string
	RequestedHyperbricksPath    string
)

const pluginIndexURL = "https://raw.githubusercontent.com/hyperbricks/hyperbricks-plugins/main/plugins.index.json"
const pluginRepoURL = "https://github.com/hyperbricks/hyperbricks-plugins"

// Root "plugin" command
func PluginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Hyperbricks plugin manager",
	}
	cmd.AddCommand(PluginListCommand())
	cmd.AddCommand(PluginInstallCommand())
	cmd.AddCommand(PluginRemoveCommand())
	cmd.AddCommand(PluginBuildCommand())
	cmd.AddCommand(PluginUpdateCommand())
	return cmd
}

func goToolPath() string {
	if goroot := strings.TrimSpace(runtime.GOROOT()); goroot != "" {
		candidate := filepath.Join(goroot, "bin", "go")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "go"
}

func goToolCommand(args ...string) *exec.Cmd {
	return exec.Command(goToolPath(), args...)
}

func extractHyperbricksVersionFromBinary(soPath string) (string, error) {
	cmd := goToolCommand("version", "-m", soPath)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to inspect binary: %v", err)
	}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "github.com/hyperbricks/hyperbricks") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				return strings.TrimPrefix(fields[2], "v"), nil
			}
		}
	}
	return "", fmt.Errorf("hyperbricks version not found in binary")
}

func PluginListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available plugins compatible with this Hyperbricks version",
		Run: func(cmd *cobra.Command, args []string) {
			Exit = true
			hbVer, err := semver.NewVersion(getHyperbricksSemver())
			if err != nil {
				fmt.Println("Error: could not parse Hyperbricks version:", err)
				return
			}

			plugins, err := fetchPluginIndex()
			if err != nil {
				fmt.Println("Error fetching plugin index:", err)
				return
			}

			type PluginView struct {
				ShortName    string
				Source       string
				Version      string
				AllVersions  []string
				Compat       []string
				Installed    string
				InstalledRaw string
				InstalledSo  string
			}

			var list []PluginView

			for name, versions := range plugins {
				var compatible *PluginMeta
				var compatibleVer *semver.Version
				allVersions := make([]string, 0, len(versions))
				compatConstraints := make(map[string]struct{})

				for ver, meta := range versions {
					allVersions = append(allVersions, ver)
					for _, compat := range meta.CompatibleHyperbricks {
						compatConstraints[compat] = struct{}{}
						constraints, err := semver.NewConstraint(compat)
						if err == nil && constraints.Check(hbVer) {
							sv, _ := semver.NewVersion(ver)
							if compatible == nil || sv.GreaterThan(compatibleVer) {
								compatible = &meta
								compatibleVer = sv
							}
						}
					}
				}

				if compatible != nil {
					shortName := pluginShortName(name)
					_, artifactName := pluginOutputNames(*compatible, compatible.Source, "", compatible.Version)
					artifactPath := filepath.Join("./bin/plugins", artifactName)

					installed := "no"
					installedRaw := "no"
					installedArtifact := ""

					if _, err := os.Stat(artifactPath); err == nil {
						installedArtifact = artifactName
						if pluginRuntime(*compatible) == "wasm" {
							installed = "\033[1;32myes\033[0m"
							installedRaw = "yes"
						} else {
							ver, err := extractHyperbricksVersionFromBinary(artifactPath)
							if err != nil {
								installed = "\033[1;33myes (might be incompatible)\033[0m"
								installedRaw = "yes-maybe"
							} else {
								parsed, err := semver.NewVersion(ver)
								if err != nil {
									installed = "\033[1;33myes (might be incompatible)\033[0m"
									installedRaw = "yes-maybe"
								} else if parsed.Equal(hbVer) {
									installed = "\033[1;32myes\033[0m"
									installedRaw = "yes"
								} else {
									installed = "\033[1;31myes (incompatible)\033[0m"
									installedRaw = "no"
								}
							}
						}
					}

					compatList := make([]string, 0, len(compatConstraints))
					for k := range compatConstraints {
						compatList = append(compatList, k)
					}
					sort.Strings(compatList)
					sort.Strings(allVersions)

					list = append(list, PluginView{
						ShortName:    shortName,
						Version:      compatible.Version,
						Source:       compatible.Source,
						AllVersions:  allVersions,
						Compat:       compatList,
						Installed:    installed,
						InstalledRaw: installedRaw,
						InstalledSo:  installedArtifact,
					})
				}
			}

			sort.Slice(list, func(i, j int) bool {
				return list[i].ShortName < list[j].ShortName
			})

			if len(list) == 0 {
				fmt.Println("No compatible plugins found for this version.")
				return
			}

			fmt.Println("")
			w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintln(w, "Name\tPlugin Version\tAvailable Versions\tCompatible Hyperbricks\tInstalled")
			fmt.Fprintln(w, "----\t--------------\t------------------\t----------------------\t---------")
			for _, p := range list {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					p.ShortName,
					p.Version,
					strings.Join(p.AllVersions, ", "),
					strings.Join(p.Compat, ", "),
					p.Installed,
				)
			}
			w.Flush()

			// List binaries that are fully compatible
			var installedBinaries []string
			for _, p := range list {
				if p.InstalledRaw == "yes" {
					installedBinaries = append(installedBinaries, p.InstalledSo)
				}
			}

			if len(installedBinaries) > 0 {
				fmt.Println("")
				fmt.Println("\033[1;33mTo enable plugins, they must be compiled for the currently installed version of Hyperbricks.\033[0m")
				fmt.Println("\033[0;36mThis can be done automatically using:\033[0m")
				fmt.Println("\033[1;32m hyperbricks plugin install <name>@<plugin_version>\033[0m")
				fmt.Println("")
				fmt.Println("\033[0;36m# To preload the plugin, add the artifact name (without .so or .wasm) to your package.hyperbricks.yaml\033[0m")
				fmt.Println("\033[0;36m# under the `plugins.enabled` array:\033[0m")
				fmt.Println("\033[0;36m# Plugin binaries are named as <name>@<plugin_version> for clarity.\033[0m")

				fmt.Print("\033[1;34mhyperbricks:\n  plugins:\n    enabled:\n")
				for _, bin := range installedBinaries {
					binName := pluginConfigNameFromArtifact(bin)
					fmt.Printf("\033[1;34m      - \033[1;32m%s\033[0m\n", binName)
				}
				fmt.Println("")
			} else {
				fmt.Println("\033[1;33m\n# No compatible plugins currently installed. Use \033[1;32m`plugin build`\033[1;33m or \033[1;32m`plugin install`\033[1;33m to add them!\033[0m")
			}
		},
	}
	return cmd
}

// Installs a plugin (syntax: <name>[@<version>])
func PluginInstallCommand() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "install <name>[@<version>]",
		Short: "Install a plugin's source to ./plugin folder and build, by name (optionally @version, e.g. esbuild@1.0.0)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {

			nameArg := args[0]
			pluginName := nameArg
			version := ""
			if idx := strings.LastIndex(nameArg, "@"); idx != -1 {
				pluginName = nameArg[:idx]
				version = nameArg[idx+1:]
			}

			plugins, err := fetchPluginIndex()
			if err != nil {
				fmt.Println("Error fetching plugin index:", err)
				return
			}

			var fullName string

			for k := range plugins {
				short := pluginShortName(k)

				if short == pluginName || k == pluginName {
					fullName = k
					break
				}
			}
			if fullName == "" {
				fmt.Printf("Plugin %q not found.\n", pluginName)
				return
			}

			available := plugins[fullName]
			ver := version
			if ver == "" {
				highest := ""
				for v := range available {
					// Parse and compare versions
					verSem, err := semver.NewVersion(v)
					if err != nil {
						continue
					}
					if highest == "" {
						highest = v
					} else {
						highestSem, err := semver.NewVersion(highest)
						if err != nil {
							continue
						}
						if verSem.GreaterThan(highestSem) {
							highest = v
						}
					}
				}
				ver = highest
			}

			meta, ok := available[ver]
			if !ok {
				fmt.Printf("Version %s for plugin %q not found.\n", ver, fullName)
				return
			}

			fmt.Printf("Installing %s v%s - %s\n", pluginName, ver, meta.Description)
			fmt.Println("Compatible with Hyperbricks versions:", strings.Join(meta.CompatibleHyperbricks, ", "))

			pluginShort := pluginShortName(fullName)
			if err := sparseClonePlugin(pluginShort, ver); err != nil {
				fmt.Printf("Sparse clone failed: %v\n", err)
				return
			}

			// Build the plugin after cloning
			fmt.Println("Building plugin...")
			source := meta.Source
			if strings.TrimSpace(source) == "" {
				fmt.Println("Warning: 'source' field is missing in manifest.json")
				return
			}
			configName, outputName := pluginOutputNames(meta, source, "", ver)
			sourceDir := pluginSourceDirFor("", pluginShort, ver)
			if err := buildPlugin(pluginBuildSpec{
				SourceDir:          sourceDir,
				SourceFile:         source,
				OutputName:         outputName,
				DisplayName:        pluginShort,
				ExpectedModulePath: expectedModulePathForBuild("", pluginShort),
				Runtime:            pluginRuntime(meta),
			}); err != nil {
				fmt.Printf("Build failed: %v\n", err)
				return
			}

			fmt.Println("Installing plugin...")
			fmt.Printf("Plugin \"%s\" (%s) installed successfully.\n", pluginName, ver)
			fmt.Printf("Config name: %s\n", configName)
		},
	}

	cmd.Flags().StringVarP(
		&RequestedHyperbricksVersion,
		"hyperbricks-version",
		"v",
		"",
		"Specify the Hyperbricks version to build against",
	)

	cmd.Flags().StringVar(
		&RequestedHyperbricksPath,
		"hyperbricks-path",
		"",
		"Use a local Hyperbricks checkout when building plugins",
	)
	return cmd
}

// Build or rebuild a plugin from source (syntax: <name>@<version>)
func PluginBuildCommand() *cobra.Command {
	var pluginBuildModule string

	cmd := &cobra.Command{
		Use:   "build <name>@<version>",
		Short: "Build or rebuild a plugin from source in ./plugins or modules/<module>/plugins",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			pluginArg := args[0]
			parts := strings.Split(pluginArg, "@")
			if len(parts) != 2 {
				fmt.Println("Usage: build <name>@<version>")
				return
			}
			name, version := parts[0], parts[1]

			module := strings.TrimSpace(pluginBuildModule)
			manifestPath := pluginManifestPathFor(module, name, version)
			manifestData, err := os.ReadFile(manifestPath)
			if err != nil {
				fmt.Printf("Warning: manifest.json not found at '%s'\n", manifestPath)
				return
			}

			var meta PluginMeta
			if err := json.Unmarshal(manifestData, &meta); err != nil {
				fmt.Printf("Warning: could not parse manifest.json: %v\n", err)
				return
			}

			if meta.Source == "" {
				fmt.Println("Warning: 'source' field is missing in manifest.json")
				return
			}

			source := meta.Source
			configName, outputName := pluginOutputNames(meta, source, module, version)
			sourceDir := pluginSourceDirFor(module, name, version)

			fmt.Println("Building:", name, "Version:", version)
			if err := buildPlugin(pluginBuildSpec{
				SourceDir:          sourceDir,
				SourceFile:         source,
				OutputName:         outputName,
				DisplayName:        name,
				ExpectedModulePath: expectedModulePathForBuild(module, name),
				Runtime:            pluginRuntime(meta),
			}); err != nil {
				fmt.Printf("Build failed: %v\n", err)
				return
			}
			fmt.Printf("Config name: %s\n", configName)
		},
	}
	cmd.Flags().StringVarP(&pluginBuildModule, "module", "m", "", "Module name for custom plugins")
	return cmd
}

type pluginBuildSpec struct {
	SourceDir          string
	SourceFile         string
	OutputName         string
	DisplayName        string
	ExpectedModulePath string
	Runtime            string
	LogWriter          io.Writer
}

func expectedBuiltInPluginModulePath(pluginShortName string) string {
	pluginShortName = strings.TrimSpace(pluginShortName)
	if pluginShortName == "" {
		return ""
	}
	return "github.com/hyperbricks/plugins/" + pluginShortName
}

func expectedModulePathForBuild(module string, pluginShortName string) string {
	if strings.TrimSpace(module) != "" {
		return ""
	}
	return expectedBuiltInPluginModulePath(pluginShortName)
}

func declaredModulePath(goModPath string) (string, error) {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "module ")), nil
		}
	}
	return "", fmt.Errorf("module directive not found in %s", goModPath)
}

func extractMainModulePathFromBinary(soPath string) (string, error) {
	cmd := goToolCommand("version", "-m", soPath)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to inspect binary: %v", err)
	}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "path" {
			return strings.TrimSpace(fields[1]), nil
		}
	}
	return "", fmt.Errorf("main module path not found in binary metadata")
}

func buildPlugin(spec pluginBuildSpec) error {
	writer := spec.LogWriter
	if writer == nil {
		writer = os.Stdout
	}
	fmt.Fprintf(writer, "Building plugin: %s\n", spec.DisplayName)
	switch pluginRuntimeFromString(spec.Runtime) {
	case "native":
		return buildNativePlugin(spec, writer)
	case "wasm":
		return buildWasmPlugin(spec, writer)
	default:
		return fmt.Errorf("unsupported plugin runtime %q", spec.Runtime)
	}
}

func buildNativePlugin(spec pluginBuildSpec, writer io.Writer) error {
	fmt.Fprintf(writer, "Using Go toolchain: %s\n", goToolPath())
	pluginDir := filepath.Join(".", "bin", "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %v", err)
	}

	pluginSourceDir := spec.SourceDir
	if abs, err := filepath.Abs(pluginSourceDir); err == nil {
		pluginSourceDir = abs
	}
	pluginSourcePath := filepath.Join(pluginSourceDir, spec.SourceFile)
	if _, err := os.Stat(pluginSourcePath); os.IsNotExist(err) {
		return fmt.Errorf("plugin source file %s does not exist", pluginSourcePath)
	}
	goModPath := filepath.Join(pluginSourceDir, "go.mod")
	declaredPath, err := declaredModulePath(goModPath)
	if err != nil {
		return fmt.Errorf("failed to read plugin module path: %v", err)
	}
	if expected := strings.TrimSpace(spec.ExpectedModulePath); expected != "" && declaredPath != expected {
		return fmt.Errorf("plugin module path mismatch: expected %s, found %s in %s", expected, declaredPath, goModPath)
	}

	localHyperbricksPath := strings.TrimSpace(RequestedHyperbricksPath)
	if localHyperbricksPath == "" {
		localHyperbricksPath = strings.TrimSpace(os.Getenv("HYPERBRICKS_LOCAL_PATH"))
	}

	if localHyperbricksPath != "" {
		absLocalPath, err := filepath.Abs(localHyperbricksPath)
		if err != nil {
			return fmt.Errorf("failed to resolve local Hyperbricks path: %v", err)
		}

		if _, err := os.Stat(filepath.Join(absLocalPath, "go.mod")); err != nil {
			return fmt.Errorf("local Hyperbricks path does not contain go.mod: %s", absLocalPath)
		}

		fmt.Fprintf(writer, "Using local Hyperbricks checkout: %s\n", absLocalPath)

		reqCmd := goToolCommand("mod", "edit", "-require=github.com/hyperbricks/hyperbricks@v0.0.0")
		reqCmd.Dir = pluginSourceDir
		reqCmd.Stdout = writer
		reqCmd.Stderr = writer
		if err := reqCmd.Run(); err != nil {
			return fmt.Errorf("failed to set local Hyperbricks require: %v", err)
		}

		replaceCmd := goToolCommand("mod", "edit", "-replace=github.com/hyperbricks/hyperbricks="+absLocalPath)
		replaceCmd.Dir = pluginSourceDir
		replaceCmd.Stdout = writer
		replaceCmd.Stderr = writer
		if err := replaceCmd.Run(); err != nil {
			return fmt.Errorf("failed to set local Hyperbricks replace: %v", err)
		}
	} else {
		mainVersion := getHyperbricksSemver()
		if RequestedHyperbricksVersion != "" {
			mainVersion = RequestedHyperbricksVersion
		}
		mainVersion = strings.TrimPrefix(strings.TrimSpace(mainVersion), "v")

		fmt.Fprintf(writer, "Patching %s go.mod Hyperbricks dependency to v%s\n", spec.DisplayName, mainVersion)

		reqCmd := goToolCommand("mod", "edit", "-require=github.com/hyperbricks/hyperbricks@v"+mainVersion)
		reqCmd.Dir = pluginSourceDir
		reqCmd.Stdout = writer
		reqCmd.Stderr = writer
		if err := reqCmd.Run(); err != nil {
			return fmt.Errorf("failed to set Hyperbricks version: %v", err)
		}

		dropReplaceCmd := goToolCommand("mod", "edit", "-dropreplace=github.com/hyperbricks/hyperbricks")
		dropReplaceCmd.Dir = pluginSourceDir
		dropReplaceCmd.Stdout = writer
		dropReplaceCmd.Stderr = writer
		_ = dropReplaceCmd.Run()
	}

	// Run `go mod tidy` inside the plugin source dir
	tidyCmd := goToolCommand("mod", "tidy")
	tidyCmd.Dir = pluginSourceDir
	tidyCmd.Stdout = writer
	tidyCmd.Stderr = writer
	if err := tidyCmd.Run(); err != nil {
		return fmt.Errorf("failed to run go mod tidy: %v", err)
	}

	outputPath := filepath.Join(pluginDir, spec.OutputName)
	if abs, err := filepath.Abs(outputPath); err == nil {
		outputPath = abs
	}

	// Build from inside the plugin source dir, using absolute output path
	buildCmd := goToolCommand("build", "-buildmode=plugin", "-o", outputPath, ".")
	buildCmd.Dir = pluginSourceDir
	buildCmd.Stdout = writer
	buildCmd.Stderr = writer
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("failed to build plugin: %v", err)
	}
	fmt.Fprintf(writer, "Build successful: %s\n", outputPath)
	return nil
}

func buildWasmPlugin(spec pluginBuildSpec, writer io.Writer) error {
	pluginDir := filepath.Join(".", "bin", "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %v", err)
	}

	pluginSourceDir := spec.SourceDir
	if abs, err := filepath.Abs(pluginSourceDir); err == nil {
		pluginSourceDir = abs
	}
	pluginSourcePath := filepath.Join(pluginSourceDir, spec.SourceFile)
	if _, err := os.Stat(pluginSourcePath); os.IsNotExist(err) {
		return fmt.Errorf("plugin source file %s does not exist", pluginSourcePath)
	}

	outputPath := filepath.Join(pluginDir, spec.OutputName)
	if abs, err := filepath.Abs(outputPath); err == nil {
		outputPath = abs
	}
	if !strings.HasSuffix(outputPath, ".wasm") {
		return fmt.Errorf("wasm plugin output must end with .wasm: %s", outputPath)
	}

	switch strings.ToLower(filepath.Ext(spec.SourceFile)) {
	case ".go":
		return buildGoWasmPlugin(writer, pluginSourceDir, pluginSourcePath, outputPath)
	case ".c":
		return buildClangWasmPlugin(writer, pluginSourceDir, pluginSourcePath, outputPath)
	default:
		return fmt.Errorf("unsupported wasm plugin source type %q", filepath.Ext(spec.SourceFile))
	}
}

func buildGoWasmPlugin(writer io.Writer, pluginSourceDir string, pluginSourcePath string, outputPath string) error {
	fmt.Fprintf(writer, "Using Go WASM toolchain: %s\n", goToolPath())
	buildCmd := goToolCommand("build", "-buildmode=c-shared", "-o", outputPath, pluginSourcePath)
	buildCmd.Dir = pluginSourceDir
	buildCmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	buildCmd.Stdout = writer
	buildCmd.Stderr = writer
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("failed to build go wasm plugin: %v", err)
	}
	fmt.Fprintf(writer, "Build successful: %s\n", outputPath)
	return nil
}

func buildClangWasmPlugin(writer io.Writer, pluginSourceDir string, pluginSourcePath string, outputPath string) error {
	clangPath, err := exec.LookPath("clang")
	if err != nil {
		return fmt.Errorf("clang is required to build C wasm plugins: %v", err)
	}
	fmt.Fprintf(writer, "Using clang toolchain: %s\n", clangPath)
	buildCmd := exec.Command(
		clangPath,
		"--target=wasm32",
		"-O2",
		"-nostdlib",
		"-Wl,--no-entry",
		"-Wl,--export-memory",
		"-Wl,--export=alloc",
		"-Wl,--export=render",
		"-Wl,--initial-memory=131072",
		"-Wl,--max-memory=131072",
		"-o",
		outputPath,
		pluginSourcePath,
	)
	buildCmd.Dir = pluginSourceDir
	buildCmd.Stdout = writer
	buildCmd.Stderr = writer
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("failed to build C wasm plugin: %v", err)
	}
	fmt.Fprintf(writer, "Build successful: %s\n", outputPath)
	return nil
}

// Remove a locally installed plugin
func PluginRemoveCommand() *cobra.Command {
	var pluginRemoveModule string

	cmd := &cobra.Command{
		Use:   "remove <name>@<version>",
		Short: "Remove a locally installed plugin",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			arg := args[0]
			parts := strings.Split(arg, "@")
			if len(parts) != 2 {
				fmt.Println("Usage: remove <name>@<version>")
				return
			}
			pluginShort := parts[0]
			version := parts[1]
			module := strings.TrimSpace(pluginRemoveModule)

			var meta PluginMeta
			manifestPath := pluginManifestPathFor(module, pluginShort, version)
			if manifestData, err := os.ReadFile(manifestPath); err == nil {
				_ = json.Unmarshal(manifestData, &meta)
			}

			source := meta.Source
			if source == "" {
				source = pluginShort + ".go"
			}
			configName, outputName := pluginOutputNames(meta, source, module, version)
			soPath := filepath.Join("./bin/plugins", outputName)

			if _, err := os.Stat(soPath); os.IsNotExist(err) {
				fmt.Printf("Plugin \"%s\" (%s) is not installed.\n", configName, version)
				return
			}
			if err := os.Remove(soPath); err != nil {
				fmt.Printf("Failed to remove plugin \"%s\": %v\n", outputName, err)
				return
			}
			fmt.Printf("Plugin \"%s\" (%s) removed.\n", configName, version)
		},
	}
	cmd.Flags().StringVarP(&pluginRemoveModule, "module", "m", "", "Module name for custom plugins")
	return cmd
}

// PluginUpdateCommand reserves the update subcommand until atomic plugin
// updates are implemented. It fails without modifying the local installation.
func PluginUpdateCommand() *cobra.Command {
	return &cobra.Command{
		Use:           "update <name>",
		Short:         "Update a plugin (not implemented)",
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			Exit = true
			ExitCode = 1
			return fmt.Errorf("plugin update is not implemented; install an explicit version with hyperbricks plugin install %s@<version>", args[0])
		},
	}
}

// Clone and checkout a specific plugin version from the repo (sparse clone)
func sparseClonePlugin(pluginName, version string) error {
	pluginRelPath := filepath.Join("plugins", pluginName, version)
	destDir := filepath.Join(".", "plugins", pluginName, version)

	tmpDir, err := os.MkdirTemp("", "hyperbricks-plugin-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	// Clone the repo
	cloneCmd := exec.Command("git", "clone", "--filter=blob:none", "--no-checkout", pluginRepoURL, tmpDir)
	cloneCmd.Stdout = os.Stdout
	cloneCmd.Stderr = os.Stderr
	if err := cloneCmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %v", err)
	}

	// Sparse checkout
	cmd := exec.Command("git", "-C", tmpDir, "sparse-checkout", "init", "--cone")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sparse-checkout init failed: %v", err)
	}

	cmd = exec.Command("git", "-C", tmpDir, "sparse-checkout", "set", pluginRelPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sparse-checkout set failed: %v", err)
	}

	// Checkout
	cmd = exec.Command("git", "-C", tmpDir, "checkout")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git checkout failed: %v", err)
	}

	// Copy plugin files
	srcPath := filepath.Join(tmpDir, "plugins", pluginName, version)
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return fmt.Errorf("plugin not found in repo: %s", pluginRelPath)
	}

	if err := os.MkdirAll(filepath.Dir(destDir), 0755); err != nil {
		return err
	}
	return copyDir(srcPath, destDir)
}

// Copies a directory recursively
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}

func pluginSourceDirFor(module string, pluginShortName string, version string) string {
	if strings.TrimSpace(module) == "" {
		return filepath.Join("plugins", pluginShortName, version)
	}
	return filepath.Join("modules", module, "plugins", pluginShortName, version)
}

func pluginManifestPathFor(module string, pluginShortName string, version string) string {
	return filepath.Join(pluginSourceDirFor(module, pluginShortName, version), "manifest.json")
}

func pluginBinaryBase(meta PluginMeta, source string) string {
	if strings.TrimSpace(meta.Binary) != "" {
		base := strings.TrimSpace(meta.Binary)
		return pluginConfigNameFromArtifact(base)
	}
	return toCamelCase(strings.TrimSuffix(source, filepath.Ext(source)))
}

func pluginOutputNames(meta PluginMeta, source string, module string, version string) (string, string) {
	base := pluginBinaryBase(meta, source)
	if strings.TrimSpace(module) != "" {
		base = fmt.Sprintf("%s__%s", base, module)
	}
	configName := fmt.Sprintf("%s@%s", base, version)
	return configName, configName + pluginArtifactExtension(pluginRuntime(meta))
}

func pluginRuntime(meta PluginMeta) string {
	return pluginRuntimeFromString(meta.Runtime)
}

func pluginRuntimeFromString(runtimeName string) string {
	switch strings.ToLower(strings.TrimSpace(runtimeName)) {
	case "", "native", "go":
		return "native"
	case "wasm", "webassembly":
		return "wasm"
	default:
		return strings.ToLower(strings.TrimSpace(runtimeName))
	}
}

func pluginArtifactExtension(runtimeName string) string {
	if pluginRuntimeFromString(runtimeName) == "wasm" {
		return ".wasm"
	}
	return ".so"
}

func pluginConfigNameFromArtifact(name string) string {
	name = strings.TrimSuffix(name, ".so")
	name = strings.TrimSuffix(name, ".wasm")
	return name
}

type PluginBuildSpec = pluginBuildSpec

func BuildPlugin(spec PluginBuildSpec) error {
	return buildPlugin(spec)
}

func PluginOutputNames(meta PluginMeta, source string, module string, version string) (string, string) {
	return pluginOutputNames(meta, source, module, version)
}

func FetchPluginIndex() (map[string]map[string]PluginMeta, error) {
	return fetchPluginIndex()
}

// Returns the current Hyperbricks version as semver string
func getHyperbricksSemver() string {
	ver := assets.VersionMD
	ver = strings.TrimPrefix(ver, "v")
	parts := strings.Fields(ver)
	if len(parts) > 0 {
		return parts[0]
	}
	return ver
}

// HyperbricksSemver returns the current Hyperbricks version as a semver string.
func HyperbricksSemver() string {
	return getHyperbricksSemver()
}

// FilterPluginIndexByHyperbricks returns only plugin versions compatible with the given Hyperbricks version.
func FilterPluginIndexByHyperbricks(plugins map[string]map[string]PluginMeta, hbVersion string) map[string]map[string]PluginMeta {
	version := strings.TrimPrefix(strings.TrimSpace(hbVersion), "v")
	hbVer, err := semver.NewVersion(version)
	if err != nil {
		return plugins
	}

	filtered := make(map[string]map[string]PluginMeta)
	for name, versions := range plugins {
		for ver, meta := range versions {
			if len(meta.CompatibleHyperbricks) == 0 {
				continue
			}
			for _, compat := range meta.CompatibleHyperbricks {
				constraints, err := semver.NewConstraint(compat)
				if err != nil {
					continue
				}
				if constraints.Check(hbVer) {
					entry, ok := filtered[name]
					if !ok {
						entry = make(map[string]PluginMeta)
						filtered[name] = entry
					}
					entry[ver] = meta
					break
				}
			}
		}
	}
	return filtered
}

// Returns the plugin's short name (right of last '/')
func pluginShortName(fullName string) string {
	if ix := strings.LastIndex(fullName, "/"); ix >= 0 {
		return fullName[ix+1:]
	}
	return fullName
}

// Returns a CamelCase version of a plugin name (for filenames)
func toCamelCase(s string) string {
	var out []rune
	upperNext := true
	for _, r := range s {
		if r == '-' || r == '_' {
			upperNext = true
			continue
		}
		if upperNext {
			out = append(out, unicode.ToUpper(r))
			upperNext = false
		} else {
			out = append(out, r)
		}
	}
	return string(out)
}

// Fetches plugin index JSON from remote
func fetchPluginIndex() (map[string]map[string]PluginMeta, error) {
	resp, err := http.Get(pluginIndexURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch plugin index: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch plugin index, status code: %d", resp.StatusCode)
	}

	var plugins map[string]map[string]PluginMeta
	err = json.NewDecoder(resp.Body).Decode(&plugins)
	if err != nil {
		return nil, fmt.Errorf("failed to decode plugin index JSON: %v", err)
	}

	return plugins, nil
}
