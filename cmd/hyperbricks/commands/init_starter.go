package commands

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"
)

type StarterMeta struct {
	Name                  string   `json:"name"`
	Version               string   `json:"version"`
	Path                  string   `json:"path"`
	Entrypoint            string   `json:"entrypoint"`
	Description           string   `json:"description"`
	CompatibleHyperbricks []string `json:"compatible_hyperbricks"`
	Tags                  []string `json:"tags,omitempty"`
}

var (
	initStarterModule  string
	starterIndexURL    = "https://raw.githubusercontent.com/hyperbricks/hyperbricks-starters/main/starters.index.json"
	starterArchiveURL  = "https://github.com/hyperbricks/hyperbricks-starters/archive/refs/heads/main.zip"
	starterArchiveRoot = "hyperbricks-starters-main"
)

func InitStarterCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init-starter",
		Short: "Initialize a module from an official HyperBricks starter",
	}
	cmd.AddCommand(InitStarterListCommand())
	cmd.AddCommand(InitStarterGetCommand())
	return cmd
}

func InitStarterListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available starters compatible with this HyperBricks version",
		Run: func(cmd *cobra.Command, args []string) {
			Exit = true

			hbVer, err := semver.NewVersion(getHyperbricksSemver())
			if err != nil {
				fmt.Println("Error: could not parse HyperBricks version:", err)
				return
			}

			starters, err := fetchStarterIndex()
			if err != nil {
				fmt.Println("Error fetching starter index:", err)
				return
			}

			type StarterView struct {
				Name        string
				Version     string
				AllVersions []string
				Compat      []string
				Description string
			}

			var list []StarterView

			for name, versions := range starters {
				var compatible *StarterMeta
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
								metaCopy := meta
								compatible = &metaCopy
								compatibleVer = sv
							}
						}
					}
				}

				if compatible == nil {
					continue
				}

				compatList := make([]string, 0, len(compatConstraints))
				for k := range compatConstraints {
					compatList = append(compatList, k)
				}
				sort.Strings(compatList)
				sort.Strings(allVersions)

				list = append(list, StarterView{
					Name:        name,
					Version:     compatible.Version,
					AllVersions: allVersions,
					Compat:      compatList,
					Description: compatible.Description,
				})
			}

			sort.Slice(list, func(i, j int) bool {
				return list[i].Name < list[j].Name
			})

			if len(list) == 0 {
				fmt.Println("No compatible starters found for this HyperBricks version.")
				return
			}

			fmt.Println("")
			w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintln(w, "Name\tStarter Version\tAvailable Versions\tCompatible HyperBricks\tDescription")
			fmt.Fprintln(w, "----\t---------------\t------------------\t----------------------\t-----------")
			for _, starter := range list {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					starter.Name,
					starter.Version,
					strings.Join(starter.AllVersions, ", "),
					strings.Join(starter.Compat, ", "),
					starter.Description,
				)
			}
			w.Flush()
			fmt.Println("")
			fmt.Println("Install with:")
			fmt.Println("  hyperbricks init-starter get <name>[@version] -m <module>")
		},
	}
	return cmd
}

func InitStarterGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <name>[@<version>]",
		Short: "Download an official HyperBricks starter into ./modules/<module>",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			Exit = true

			moduleName, starter, err := runInitStarterGet(args[0], initStarterModule)
			if err != nil {
				fmt.Printf("Error installing starter: %v\n", err)
				return
			}

			fmt.Printf("Starter \"%s@%s\" installed to modules/%s\n", starter.Name, starter.Version, moduleName)
			fmt.Printf("Next: hyperbricks start -m %s\n", moduleName)
		},
	}
	cmd.Flags().StringVarP(&initStarterModule, "module", "m", "", "name-of-module (defaults to starter name)")
	return cmd
}

func runInitStarterGet(nameArg string, moduleOverride string) (string, StarterMeta, error) {
	starterName, requestedVersion := parseNameVersionArg(nameArg)
	if strings.TrimSpace(starterName) == "" {
		return "", StarterMeta{}, fmt.Errorf("starter name cannot be empty")
	}

	starters, err := fetchStarterIndex()
	if err != nil {
		return "", StarterMeta{}, err
	}

	starter, err := resolveStarterVersion(starters, starterName, requestedVersion)
	if err != nil {
		return "", StarterMeta{}, err
	}

	moduleName := strings.TrimSpace(moduleOverride)
	if moduleName == "" {
		moduleName = starter.Name
	}
	if moduleName == "" {
		return "", StarterMeta{}, fmt.Errorf("module name cannot be empty")
	}

	if err := installStarter(starter, moduleName); err != nil {
		return "", StarterMeta{}, err
	}

	return moduleName, starter, nil
}

func fetchStarterIndex() (map[string]map[string]StarterMeta, error) {
	resp, err := http.Get(starterIndexURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch starter index: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch starter index, status code: %d", resp.StatusCode)
	}

	var starters map[string]map[string]StarterMeta
	if err := json.NewDecoder(resp.Body).Decode(&starters); err != nil {
		return nil, fmt.Errorf("failed to decode starter index JSON: %v", err)
	}

	for name, versions := range starters {
		for version, meta := range versions {
			starters[name][version] = normalizeStarterMeta(name, version, meta)
		}
	}

	return starters, nil
}

func normalizeStarterMeta(name string, version string, meta StarterMeta) StarterMeta {
	if strings.TrimSpace(meta.Name) == "" {
		meta.Name = name
	}
	if strings.TrimSpace(meta.Version) == "" {
		meta.Version = version
	}
	if strings.TrimSpace(meta.Path) == "" {
		meta.Path = filepath.ToSlash(filepath.Join("starters", name, version))
	}
	if strings.TrimSpace(meta.Entrypoint) == "" {
		meta.Entrypoint = "package.hyperbricks"
	}
	return meta
}

func parseNameVersionArg(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if idx := strings.LastIndex(raw, "@"); idx != -1 {
		return strings.TrimSpace(raw[:idx]), strings.TrimSpace(raw[idx+1:])
	}
	return raw, ""
}

func resolveStarterVersion(starters map[string]map[string]StarterMeta, starterName string, requestedVersion string) (StarterMeta, error) {
	versions, ok := starters[starterName]
	if !ok {
		return StarterMeta{}, fmt.Errorf("starter not found: %s", starterName)
	}

	hbVer, err := semver.NewVersion(getHyperbricksSemver())
	if err != nil {
		return StarterMeta{}, fmt.Errorf("could not parse HyperBricks version: %w", err)
	}

	if strings.TrimSpace(requestedVersion) != "" {
		meta, ok := versions[requestedVersion]
		if !ok {
			return StarterMeta{}, fmt.Errorf("starter version not found: %s@%s", starterName, requestedVersion)
		}
		if !starterCompatible(meta, hbVer) {
			return StarterMeta{}, fmt.Errorf("starter %s@%s is not compatible with HyperBricks %s", starterName, requestedVersion, hbVer.String())
		}
		return meta, nil
	}

	var latest StarterMeta
	var latestVer *semver.Version
	for version, meta := range versions {
		if !starterCompatible(meta, hbVer) {
			continue
		}
		sv, err := semver.NewVersion(version)
		if err != nil {
			continue
		}
		if latestVer == nil || sv.GreaterThan(latestVer) {
			latest = meta
			latestVer = sv
		}
	}

	if latestVer == nil {
		return StarterMeta{}, fmt.Errorf("no compatible versions found for starter %s", starterName)
	}

	return latest, nil
}

func starterCompatible(meta StarterMeta, hbVer *semver.Version) bool {
	if len(meta.CompatibleHyperbricks) == 0 {
		return true
	}
	for _, compat := range meta.CompatibleHyperbricks {
		constraints, err := semver.NewConstraint(compat)
		if err != nil {
			continue
		}
		if constraints.Check(hbVer) {
			return true
		}
	}
	return false
}

func installStarter(meta StarterMeta, moduleName string) error {
	moduleDir := filepath.Join("modules", moduleName)
	if err := ensureEmptyOrMissingDir(moduleDir); err != nil {
		return err
	}

	archivePath, err := downloadStarterArchive()
	if err != nil {
		return err
	}
	defer os.Remove(archivePath)

	stageDir, err := os.MkdirTemp("", ".hyperbricks-starter-stage-*")
	if err != nil {
		return fmt.Errorf("failed to create starter staging directory: %w", err)
	}
	defer os.RemoveAll(stageDir)

	prefix := filepath.ToSlash(filepath.Join(starterArchiveRoot, meta.Path))
	if err := extractZipSubdirArchive(archivePath, stageDir, prefix); err != nil {
		return err
	}

	entrypoint := filepath.Join(stageDir, filepath.FromSlash(meta.Entrypoint))
	if _, err := os.Stat(entrypoint); err != nil {
		return fmt.Errorf("starter entrypoint not found after extraction: %s", meta.Entrypoint)
	}
	if err := os.Remove(filepath.Join(stageDir, "manifest.json")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove starter manifest from staging directory: %w", err)
	}

	if err := copyDir(stageDir, moduleDir); err != nil {
		return fmt.Errorf("failed to copy starter into %s: %w", moduleDir, err)
	}

	createModuleDirectories(moduleName)
	return nil
}

func ensureEmptyOrMissingDir(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to inspect %s: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path already exists and is not a directory: %s", path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("module directory already exists and is not empty: %s", path)
	}
	return nil
}

func downloadStarterArchive() (string, error) {
	resp, err := http.Get(starterArchiveURL)
	if err != nil {
		return "", fmt.Errorf("failed to download starter archive: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download starter archive, status code: %d", resp.StatusCode)
	}

	archiveFile, err := os.CreateTemp("", ".hyperbricks-starter-*.zip")
	if err != nil {
		return "", fmt.Errorf("failed to create temp archive file: %w", err)
	}

	if _, err := io.Copy(archiveFile, resp.Body); err != nil {
		archiveFile.Close()
		return "", fmt.Errorf("failed to write starter archive: %w", err)
	}

	if err := archiveFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close starter archive: %w", err)
	}

	return archiveFile.Name(), nil
}

func extractZipSubdirArchive(archivePath string, dest string, prefix string) error {
	if _, err := os.Stat(archivePath); err != nil {
		return fmt.Errorf("archive not found: %s", archivePath)
	}

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive %s: %w", archivePath, err)
	}
	defer reader.Close()

	destClean := filepath.Clean(dest)
	normalizedPrefix := normalizeArchivePrefix(prefix)
	matched := 0

	for _, file := range reader.File {
		entryName := filepath.ToSlash(file.Name)
		if normalizedPrefix != "" {
			if !strings.HasPrefix(entryName, normalizedPrefix) {
				continue
			}
			entryName = strings.TrimPrefix(entryName, normalizedPrefix)
		}

		entryName = strings.TrimPrefix(entryName, "/")
		if entryName == "" {
			continue
		}

		targetPath, err := safeArchivePath(destClean, entryName)
		if err != nil {
			return err
		}
		matched++

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, file.Mode()); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(targetPath), err)
		}

		source, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open archive entry %s: %w", file.Name, err)
		}

		out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
		if err != nil {
			source.Close()
			return fmt.Errorf("failed to create file %s: %w", targetPath, err)
		}

		if _, err := io.Copy(out, source); err != nil {
			out.Close()
			source.Close()
			return fmt.Errorf("failed to write file %s: %w", targetPath, err)
		}
		if err := out.Close(); err != nil {
			source.Close()
			return fmt.Errorf("failed to close file %s: %w", targetPath, err)
		}
		if err := source.Close(); err != nil {
			return fmt.Errorf("failed to close archive entry %s: %w", file.Name, err)
		}
	}

	if matched == 0 {
		return fmt.Errorf("starter archive path not found: %s", prefix)
	}

	return nil
}

func normalizeArchivePrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	prefix = filepath.ToSlash(prefix)
	prefix = strings.TrimPrefix(prefix, "./")
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return ""
	}
	return prefix + "/"
}
