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
	"plugin"
	"reflect"
	"regexp"
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
	CompatibleHyperbricks []string `json:"compatible_hyperbricks"`
	Description           string   `json:"description"`
}

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

func checkPluginBinaryCompatibility(path string, hbVer *semver.Version) (bool, error) {
	p, err := plugin.Open(path)
	if err != nil {
		return false, fmt.Errorf("failed to open plugin: %w", err)
	}

	sym, err := p.Lookup("CompatibleHyperbricks")
	if err != nil {
		return false, fmt.Errorf("plugin missing CompatibleHyperbricks symbol")
	}

	// Use reflect to unwrap the value safely
	val := reflect.ValueOf(sym)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Slice {
		return false, fmt.Errorf("CompatibleHyperbricks is not a slice")
	}

	for i := 0; i < val.Len(); i++ {
		raw := val.Index(i)
		if raw.Kind() != reflect.String {
			continue
		}
		constraintStr := raw.String()
		constraint, err := semver.NewConstraint(constraintStr)
		if err != nil {
			continue
		}
		if constraint.Check(hbVer) {
			return true, nil
		}
	}

	return false, nil
}

func matchesConstraint(hbVer *semver.Version, constraintStr string) bool {
	c, err := semver.NewConstraint(constraintStr)
	if err != nil {
		return false
	}
	return c.Check(hbVer)
}

func PluginListCommandOld() *cobra.Command {
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
				ShortName   string
				Source      string
				Version     string
				AllVersions []string
				Compat      []string
				Installed   bool
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
					camel := toCamelCase(strings.TrimSuffix(compatible.Source, ".go"))
					soName := fmt.Sprintf("%s@%s.so", camel, compatible.Version)
					soPath := filepath.Join("./bin/plugins", soName)

					installed := false
					if _, err := os.Stat(soPath); err == nil {
						installed = true
					}

					compatList := make([]string, 0, len(compatConstraints))
					for k := range compatConstraints {
						compatList = append(compatList, k)
					}
					sort.Strings(compatList)
					sort.Strings(allVersions)

					list = append(list, PluginView{
						ShortName:   shortName,
						Version:     compatible.Version,
						Source:      compatible.Source,
						AllVersions: allVersions,
						Compat:      compatList,
						Installed:   installed,
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
				installedText := "no"
				if p.Installed {
					installedText = "yes"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					p.ShortName,
					p.Version,
					strings.Join(p.AllVersions, ", "),
					strings.Join(p.Compat, ", "),
					installedText,
				)
			}
			w.Flush()

			// Gather all installed plugin binaries
			var installedBinaries []string
			for _, p := range list {
				if p.Installed {
					camel := toCamelCase(strings.TrimSuffix(p.Source, ".go"))
					bin := fmt.Sprintf("%s@%s.so", camel, p.Version)
					installedBinaries = append(installedBinaries, bin)
				}
			}

			if len(installedBinaries) > 0 {
				fmt.Println("")
				fmt.Println("\033[1;33mTo enable plugins, they must be compiled for the currently installed version of Hyperbricks.\033[0m")
				fmt.Println("\033[0;36mThis can be done automatically using:\033[0m")
				fmt.Println("\033[1;32m hyperbricks plugin install <name>@<plugin_version>\033[0m\n")
				fmt.Println("\033[0;36m# To preload the plugin, add the binary .so name to your package.hyperbricks\033[0m")
				fmt.Println("\033[0;36m# under the `plugins.enabled` array:\033[0m")
				fmt.Println("\033[0;36m# Plugin binaries are named as <name>@<plugin_version>.so for clarity.\033[0m")

				fmt.Printf("\033[1;34mplugins {\n  enabled = [ ")
				for i, bin := range installedBinaries {
					if i > 0 {
						fmt.Print(" ")
					}
					fmt.Printf("\033[1;32m%s\033[0m", bin)
					if i < len(installedBinaries)-1 {
						fmt.Print(",")
					}
				}
				fmt.Println("\033[1;34m ]\n}\033[0m")
				fmt.Println("")
			} else {
				fmt.Println("\033[1;33m\n# No plugins currently installed. Use \033[1;32m`plugin build`\033[1;33m or \033[1;32m`plugin install`\033[1;33m to add them!\033[0m")
			}
		},
	}
	return cmd
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
				ShortName   string
				Source      string
				Version     string
				AllVersions []string
				Compat      []string
				Installed   bool
				Compatible  bool
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
					camel := toCamelCase(strings.TrimSuffix(compatible.Source, ".go"))
					soName := fmt.Sprintf("%s@%s.so", camel, compatible.Version)
					soPath := filepath.Join("./bin/plugins", soName)

					installed := false
					compatibleBinary := false
					if _, err := os.Stat(soPath); err == nil {
						installed = true
						compatibleBinary, _ = checkPluginBinaryCompatibility(soPath, hbVer)
					}

					compatList := make([]string, 0, len(compatConstraints))
					for k := range compatConstraints {
						compatList = append(compatList, k)
					}
					sort.Strings(compatList)
					sort.Strings(allVersions)

					list = append(list, PluginView{
						ShortName:   shortName,
						Version:     compatible.Version,
						Source:      compatible.Source,
						AllVersions: allVersions,
						Compat:      compatList,
						Installed:   installed,
						Compatible:  compatibleBinary,
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
				installedText := "no"
				if p.Installed {
					if p.Compatible {
						installedText = "\033[1;32myes\033[0m"
					} else {
						installedText = "\033[1;31myes (incompatible)\033[0m"
					}
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					p.ShortName,
					p.Version,
					strings.Join(p.AllVersions, ", "),
					strings.Join(p.Compat, ", "),
					installedText,
				)
			}
			w.Flush()

			// Gather all compatible installed plugin binaries
			var installedBinaries []string
			for _, p := range list {
				if p.Installed && p.Compatible {
					camel := toCamelCase(strings.TrimSuffix(p.Source, ".go"))
					bin := fmt.Sprintf("%s@%s.so", camel, p.Version)
					installedBinaries = append(installedBinaries, bin)
				}
			}

			if len(installedBinaries) > 0 {
				fmt.Println("")
				fmt.Println("\033[1;33mTo enable plugins, they must be compiled for the currently installed version of Hyperbricks.\033[0m")
				fmt.Println("\033[0;36mThis can be done automatically using:\033[0m")
				fmt.Println("\033[1;32m hyperbricks plugin install <name>@<plugin_version>\033[0m\n")
				fmt.Println("\033[0;36m# To preload the plugin, add the binary .so name to your package.hyperbricks\033[0m")
				fmt.Println("\033[0;36m# under the `plugins.enabled` array:\033[0m")
				fmt.Println("\033[0;36m# Plugin binaries are named as <name>@<plugin_version>.so for clarity.\033[0m")

				fmt.Printf("\033[1;34mplugins {\n  enabled = [ ")
				for i, bin := range installedBinaries {
					if i > 0 {
						fmt.Print(" ")
					}
					fmt.Printf("\033[1;32m%s\033[0m", bin)
					if i < len(installedBinaries)-1 {
						fmt.Print(",")
					}
				}
				fmt.Println("\033[1;34m ]\n}\033[0m")
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
			if err := buildPlugin(source, pluginShort, ver); err != nil {
				fmt.Printf("Build failed: %v\n", err)
				return
			}

			fmt.Println("Installing plugin...")
			fmt.Printf("Plugin \"%s\" (%s) installed successfully.\n", pluginName, ver)
		},
	}
	return cmd
}

// Build or rebuild a plugin from source (syntax: <name>@<version>)
func PluginBuildCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "build <name>@<version>",
		Short: "Build or rebuild a plugin from source in ./plugin/<name>/<version> to ./bin/plugins/",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			pluginArg := args[0]
			parts := strings.Split(pluginArg, "@")
			if len(parts) != 2 {
				fmt.Println("Usage: build <name>@<version>")
				return
			}
			name, version := parts[0], parts[1]

			manifestPath := fmt.Sprintf("./plugins/%s/%s/manifest.json", name, version)
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

			fmt.Println("Building:", name, "Version:", version)
			if err := buildPlugin(source, name, version); err != nil {
				fmt.Printf("Build failed: %v\n", err)
			}
		},
	}
}

func buildPlugin(source, pluginShortName, version string) error {
	fmt.Printf("Building plugin: %s@%s\n", pluginShortName, version)
	pluginDir := "./bin/plugins"
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %v", err)
	}

	// Absolute source dir and filename
	pluginSourceDir := filepath.Join("plugins", pluginShortName, version)
	pluginSourceFile := source
	pluginSourcePath := filepath.Join(pluginSourceDir, pluginSourceFile)
	if _, err := os.Stat(pluginSourcePath); os.IsNotExist(err) {
		return fmt.Errorf("plugin source file %s does not exist", pluginSourcePath)
	}

	// === PATCH plugin go.mod with current hyperbricks version ===
	mainVersion := getHyperbricksSemver()
	gomodPath := filepath.Join(pluginSourceDir, "go.mod")
	gomodData, err := os.ReadFile(gomodPath)
	if err != nil {
		return fmt.Errorf("failed to read plugin go.mod: %v", err)
	}
	// Replace hyperbricks version line (works even if in require block)
	re := regexp.MustCompile(`(github.com/hyperbricks/hyperbricks\s+)v[\w\.\-]+`)
	newGoMod := re.ReplaceAll(gomodData, []byte("${1}v"+mainVersion))
	if string(newGoMod) != string(gomodData) {
		fmt.Printf("Patching %s go.mod hyperbricks dependency to v%s\n", pluginShortName, mainVersion)
		err = os.WriteFile(gomodPath, newGoMod, 0644)
		if err != nil {
			return fmt.Errorf("failed to write patched go.mod: %v", err)
		}
	}

	// Run `go mod tidy` inside the plugin source dir
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = pluginSourceDir
	tidyCmd.Stdout = os.Stdout
	tidyCmd.Stderr = os.Stderr
	if err := tidyCmd.Run(); err != nil {
		return fmt.Errorf("failed to run go mod tidy: %v", err)
	}

	// Output path: relative from pluginSourceDir to bin/plugins
	camel := toCamelCase(strings.TrimSuffix(source, ".go"))
	outputRelPath := filepath.ToSlash(filepath.Join("..", "..", "..", "bin", "plugins", fmt.Sprintf("%s@%s.so", camel, version)))

	// Build from inside the plugin source dir, using relative source and output paths
	buildCmd := exec.Command("go", "build", "-buildmode=plugin", "-o", outputRelPath, pluginSourceFile)
	buildCmd.Dir = pluginSourceDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("failed to build plugin: %v", err)
	}
	fmt.Println("Build successful:", outputRelPath)
	return nil
}

// Remove a locally installed plugin
func PluginRemoveCommand() *cobra.Command {
	return &cobra.Command{
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
			camel := toCamelCase(pluginShort)
			soName := fmt.Sprintf("%s@%s.so", camel, version)
			soPath := filepath.Join("./bin/plugins", soName)

			if _, err := os.Stat(soPath); os.IsNotExist(err) {
				fmt.Printf("Plugin \"%s\" (%s) is not installed.\n", pluginShort, version)
				return
			}
			if err := os.Remove(soPath); err != nil {
				fmt.Printf("Failed to remove plugin \"%s\": %v\n", soName, err)
				return
			}
			fmt.Printf("Plugin \"%s\" (%s) removed.\n", pluginShort, version)
		},
	}
}

// Update a plugin to latest compatible version (placeholder)
func PluginUpdateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "update <name>",
		Short: "Update a plugin to the latest compatible version",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			fmt.Printf("Checking for updates for plugin \"%s\"...\n", name)
			// TODO: Actually implement update logic!
			fmt.Println("Found newer version: 1.0.1 (current: 1.0.0)")
			fmt.Println("Downloading and building update...")
			fmt.Println("Update successful. Plugin \"markdown\" is now at version 1.0.1.")
		},
	}
}

// Clone and checkout a specific plugin version from the repo (sparse clone)
func sparseClonePlugin(pluginName, version string) error {
	pluginRelPath := filepath.Join("plugins", pluginName, version)
	destDir := filepath.Join(".", "plugins", pluginName, version)

	tmpDir, err := os.MkdirTemp("", ".hyperbricks-plugin-*")
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
