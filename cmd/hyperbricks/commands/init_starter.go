package commands

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/hyperbricks/hyperbricks/assets"
	"github.com/hyperbricks/hyperbricks/pkg/packagemetadata"
	"github.com/spf13/cobra"
)

type StarterMeta struct {
	Name                  string   `json:"name"`
	Path                  string   `json:"path"`
	Entrypoint            string   `json:"entrypoint"`
	Description           string   `json:"description"`
	CompatibleHyperbricks []string `json:"compatible_hyperbricks"`
	Tags                  []string `json:"tags,omitempty"`
	FixedModuleName       bool     `json:"fixed_module_name,omitempty"`
}

const starterDefaultRef = "main"

var (
	initStarterModule       string
	starterIndexURL               = "https://api.github.com/repos/hyperbricks/hyperbricks/contents/starters.index.json"
	starterArchiveURL             = "https://github.com/hyperbricks/hyperbricks/archive/%s.zip"
	starterCommitRE               = regexp.MustCompile("^[a-fA-F0-9]{40}$")
	starterIndexClient            = &http.Client{Timeout: 15 * time.Second}
	starterArchiveClient          = &http.Client{Timeout: 2 * time.Minute}
	starterMaxArchiveBytes  int64 = 256 << 20
	starterMaxModuleBytes   int64 = 128 << 20
	starterMaxModuleEntries       = 10000
	starterModuleNameRE           = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
	starterRefRE                  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
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
	var requestedRef string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List compatible starter modules from the HyperBricks repository",
		Run: func(cmd *cobra.Command, args []string) {
			Exit = true

			ref, err := normalizeStarterRef(requestedRef)
			if err != nil {
				failf("Error: %v", err)
				return
			}
			hbVer, err := semver.NewVersion(getHyperbricksSemver())
			if err != nil {
				failf("Error: could not parse HyperBricks version:"+" %v", err)
				return
			}

			starters, err := fetchStarterIndex(ref)
			if err != nil {
				failf("Error fetching starter index:"+" %v", err)
				return
			}

			type StarterView struct {
				Name        string
				Compat      []string
				Description string
			}

			var list []StarterView
			for name, meta := range starters {
				if !starterCompatible(meta, hbVer) {
					continue
				}
				compat := append([]string(nil), meta.CompatibleHyperbricks...)
				if len(compat) == 0 {
					compat = []string{"any"}
				}
				sort.Strings(compat)
				list = append(list, StarterView{
					Name:        name,
					Compat:      compat,
					Description: meta.Description,
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
			fmt.Fprintln(w, "Name\tCompatible HyperBricks\tDescription")
			fmt.Fprintln(w, "----\t----------------------\t-----------")
			for _, starter := range list {
				fmt.Fprintf(w, "%s\t%s\t%s\n",
					starter.Name,
					strings.Join(starter.Compat, ", "),
					starter.Description,
				)
			}
			w.Flush()
			fmt.Println("")
			fmt.Println("Install with:")
			fmt.Println("  hyperbricks init-starter get <name>[@tag-or-commit] -m <module>")
		},
	}
	cmd.Flags().StringVar(&requestedRef, "ref", "latest", "Git tag or commit to list (default: latest main branch)")
	return cmd
}

func InitStarterGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <name>[@<tag-or-commit>]",
		Short: "Download an official HyperBricks starter into ./modules/<module>",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			Exit = true

			moduleName, starter, err := runInitStarterGet(args[0], initStarterModule)
			if err != nil {
				failf("Error installing starter: %v\n", err)
				return
			}

			_, requestedRef, _ := parseStarterArg(args[0])
			if requestedRef == "" {
				requestedRef = "latest"
			}
			fmt.Printf("Starter \"%s\" from ref \"%s\" installed to modules/%s\n", starter.Name, requestedRef, moduleName)
			fmt.Printf("Next: hyperbricks start -m %s\n", moduleName)
		},
	}
	cmd.Flags().StringVarP(&initStarterModule, "module", "m", "", "name-of-module (defaults to starter name)")
	return cmd
}

func runInitStarterGet(nameArg string, moduleOverride string) (string, StarterMeta, error) {
	starterName, requestedRef, err := parseStarterArg(nameArg)
	if err != nil {
		return "", StarterMeta{}, err
	}
	ref, err := normalizeStarterRef(requestedRef)
	if err != nil {
		return "", StarterMeta{}, err
	}
	moduleName := strings.TrimSpace(moduleOverride)
	if moduleName != "" {
		var err error
		moduleName, err = validateInitModuleName(moduleName)
		if err != nil {
			return "", StarterMeta{}, err
		}
	}

	archivePath, err := downloadStarterArchive(ref)
	if err != nil {
		return "", StarterMeta{}, err
	}
	defer os.Remove(archivePath)
	starters, archiveRoot, err := readStarterIndexFromArchive(archivePath)
	if err != nil {
		return "", StarterMeta{}, err
	}

	starter, err := resolveStarter(starters, starterName)
	if err != nil {
		return "", StarterMeta{}, err
	}

	if moduleName == "" {
		moduleName = starter.Name
		moduleName, err = validateInitModuleName(moduleName)
		if err != nil {
			return "", StarterMeta{}, err
		}
	}
	if starter.FixedModuleName && moduleName != starter.Name {
		return "", StarterMeta{}, fmt.Errorf("starter %s requires module name %q because its plugin names refer to that module", starter.Name, starter.Name)
	}

	if err := installStarter(starter, moduleName, archivePath, archiveRoot); err != nil {
		return "", StarterMeta{}, err
	}

	return moduleName, starter, nil
}

func fetchStarterIndex(ref string) (map[string]StarterMeta, error) {
	indexURL, err := url.Parse(starterIndexURL)
	if err != nil {
		return nil, fmt.Errorf("invalid starter index URL: %w", err)
	}
	query := indexURL.Query()
	query.Set("ref", starterArchiveRefPath(ref))
	indexURL.RawQuery = query.Encode()
	request, err := http.NewRequest(http.MethodGet, indexURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create starter index request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github.raw+json")
	resp, err := starterIndexClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch starter index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch starter index, status code: %d", resp.StatusCode)
	}

	return decodeStarterIndex(resp.Body)
}

func decodeStarterIndex(source io.Reader) (map[string]StarterMeta, error) {
	const maxIndexBytes = 2 << 20
	data, err := io.ReadAll(io.LimitReader(source, maxIndexBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read starter index JSON: %w", err)
	}
	if len(data) > maxIndexBytes {
		return nil, fmt.Errorf("starter index exceeds %d bytes", maxIndexBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var starters map[string]StarterMeta
	if err := decoder.Decode(&starters); err != nil {
		return nil, fmt.Errorf("failed to decode starter index JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("starter index must contain one JSON object")
	}
	if starters == nil {
		return nil, fmt.Errorf("starter index must be a JSON object")
	}
	for name, meta := range starters {
		if !starterModuleNameRE.MatchString(name) {
			return nil, fmt.Errorf("invalid starter name in index: %q", name)
		}
		if meta.Name == "" {
			meta.Name = name
		} else if meta.Name != name {
			return nil, fmt.Errorf("starter %q has a different metadata name %q", name, meta.Name)
		}
		if meta.Path == "" {
			meta.Path = "modules/" + name
		}
		if meta.Path != "modules/"+name {
			return nil, fmt.Errorf("starter %q must point to modules/%s", name, name)
		}
		if meta.Entrypoint == "" {
			meta.Entrypoint = "package.hyperbricks.yaml"
		} else if meta.Entrypoint != "package.hyperbricks.yaml" {
			return nil, fmt.Errorf("starter %q uses unsupported entrypoint %q", name, meta.Entrypoint)
		}
		for _, constraint := range meta.CompatibleHyperbricks {
			if _, err := semver.NewConstraint(constraint); err != nil {
				return nil, fmt.Errorf("starter %q has invalid HyperBricks compatibility %q: %w", name, constraint, err)
			}
		}
		starters[name] = meta
	}
	return starters, nil
}

func readStarterIndexFromArchive(archivePath string) (map[string]StarterMeta, string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open starter archive: %w", err)
	}
	defer reader.Close()
	root, err := starterArchiveRoot(reader.File)
	if err != nil {
		return nil, "", err
	}
	indexPath := root + "/starters.index.json"
	for _, file := range reader.File {
		if file.Name != indexPath {
			continue
		}
		indexFile, err := file.Open()
		if err != nil {
			return nil, "", fmt.Errorf("failed to open starter index in archive: %w", err)
		}
		starters, decodeErr := decodeStarterIndex(indexFile)
		closeErr := indexFile.Close()
		if decodeErr != nil {
			return nil, "", decodeErr
		}
		if closeErr != nil {
			return nil, "", fmt.Errorf("failed to close starter index in archive: %w", closeErr)
		}
		return starters, root, nil
	}
	return nil, "", fmt.Errorf("starter index not found in archive at %s", indexPath)
}

func starterArchiveRoot(files []*zip.File) (string, error) {
	var root string
	for _, file := range files {
		name := file.Name
		sep := strings.IndexByte(name, '/')
		if sep < 1 || strings.Contains(name, "\\") {
			return "", fmt.Errorf("starter archive has an invalid root entry: %q", name)
		}
		entryRoot := name[:sep]
		if entryRoot == "." || entryRoot == ".." {
			return "", fmt.Errorf("starter archive has an invalid root entry: %q", name)
		}
		if root == "" {
			root = entryRoot
		} else if root != entryRoot {
			return "", fmt.Errorf("starter archive has multiple root directories")
		}
	}
	if root == "" {
		return "", fmt.Errorf("starter archive is empty")
	}
	return root, nil
}

func parseStarterArg(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", fmt.Errorf("starter name cannot be empty")
	}
	if idx := strings.LastIndex(raw, "@"); idx != -1 {
		name, ref := strings.TrimSpace(raw[:idx]), strings.TrimSpace(raw[idx+1:])
		if ref == "" {
			return "", "", fmt.Errorf("starter Git ref cannot be empty")
		}
		if !starterModuleNameRE.MatchString(name) {
			return "", "", fmt.Errorf("invalid starter name: %q", name)
		}
		return name, ref, nil
	}
	if !starterModuleNameRE.MatchString(raw) {
		return "", "", fmt.Errorf("invalid starter name: %q", raw)
	}
	return raw, "", nil
}

func normalizeStarterRef(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" || ref == "latest" {
		return starterDefaultRef, nil
	}
	if len(ref) > 255 {
		return "", fmt.Errorf("invalid starter Git ref: %q", ref)
	}
	for _, segment := range strings.Split(ref, "/") {
		if !starterRefRE.MatchString(segment) || strings.Contains(segment, "..") || strings.HasSuffix(segment, ".") || strings.HasSuffix(segment, ".lock") {
			return "", fmt.Errorf("invalid starter Git ref: %q", ref)
		}
	}
	return ref, nil
}

func starterEscapedRefPath(ref string) string {
	segments := strings.Split(ref, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}

func starterArchiveRefPath(ref string) string {
	if ref == starterDefaultRef {
		return "refs/heads/main"
	}
	if starterCommitRE.MatchString(ref) {
		return ref
	}
	return "refs/tags/" + starterEscapedRefPath(ref)
}

func resolveStarter(starters map[string]StarterMeta, starterName string) (StarterMeta, error) {
	meta, ok := starters[starterName]
	if !ok {
		return StarterMeta{}, fmt.Errorf("starter not found: %s", starterName)
	}

	hbVer, err := semver.NewVersion(getHyperbricksSemver())
	if err != nil {
		return StarterMeta{}, fmt.Errorf("could not parse HyperBricks version: %w", err)
	}

	if !starterCompatible(meta, hbVer) {
		return StarterMeta{}, fmt.Errorf("starter %s is not compatible with HyperBricks %s", starterName, hbVer.String())
	}
	return meta, nil
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

func installStarter(meta StarterMeta, moduleName string, archivePath string, archiveRoot string) error {
	moduleDir := filepath.Join("modules", moduleName)
	destinationExisted, destinationMode, err := inspectEmptyOrMissingDir(moduleDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(moduleDir), 0755); err != nil {
		return fmt.Errorf("failed to create modules directory: %w", err)
	}

	stageDir, err := os.MkdirTemp(filepath.Dir(moduleDir), "."+filepath.Base(moduleDir)+"-starter-stage-*")
	if err != nil {
		return fmt.Errorf("failed to create starter staging directory: %w", err)
	}
	stageMoved := false
	defer func() {
		if !stageMoved {
			_ = os.RemoveAll(stageDir)
		}
	}()

	prefix := filepath.ToSlash(filepath.Join(archiveRoot, meta.Path))
	if err := extractZipSubdirArchive(archivePath, stageDir, prefix); err != nil {
		return err
	}

	entrypoint := filepath.Join(stageDir, filepath.FromSlash(meta.Entrypoint))
	if _, err := os.Stat(entrypoint); err != nil {
		return fmt.Errorf("starter entrypoint not found after extraction: %s", meta.Entrypoint)
	}
	if _, err := packagemetadata.ReconcileSourceFile(entrypoint, packagemetadata.ReconcileOptions{
		Module:             moduleName,
		HyperBricks:        strings.TrimSpace(assets.VersionMD),
		ResetModuleVersion: true,
	}); err != nil {
		return fmt.Errorf("prepare starter package metadata: %w", err)
	}
	if err := os.Remove(filepath.Join(stageDir, "manifest.json")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove starter manifest from staging directory: %w", err)
	}
	for _, subdirectory := range standardModuleSubdirectories {
		if err := ensureDir(filepath.Join(stageDir, subdirectory)); err != nil {
			return fmt.Errorf("prepare starter module directory: %w", err)
		}
	}
	if err := ensureDir(filepath.Join("bin", "plugins")); err != nil {
		return fmt.Errorf("prepare global plugin directory: %w", err)
	}

	if err := os.Chmod(stageDir, 0755); err != nil {
		return fmt.Errorf("failed to set starter module permissions: %w", err)
	}
	if destinationExisted {
		if err := os.Remove(moduleDir); err != nil {
			return fmt.Errorf("failed to prepare empty module directory %s: %w", moduleDir, err)
		}
	}
	if err := os.Rename(stageDir, moduleDir); err != nil {
		if destinationExisted {
			if restoreErr := os.Mkdir(moduleDir, destinationMode.Perm()); restoreErr != nil {
				return fmt.Errorf("failed to install starter into %s: %w (also failed to restore the original empty directory: %v)", moduleDir, err, restoreErr)
			}
		}
		return fmt.Errorf("failed to install starter into %s: %w", moduleDir, err)
	}
	stageMoved = true

	return nil
}

func inspectEmptyOrMissingDir(path string) (bool, os.FileMode, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, fmt.Errorf("failed to inspect %s: %w", path, err)
	}
	if !info.IsDir() {
		return false, 0, fmt.Errorf("path already exists and is not a directory: %s", path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return false, 0, fmt.Errorf("failed to read %s: %w", path, err)
	}
	if len(entries) > 0 {
		return false, 0, fmt.Errorf("module directory already exists and is not empty: %s", path)
	}
	return true, info.Mode(), nil
}

func downloadStarterArchive(ref string) (string, error) {
	resp, err := starterArchiveClient.Get(fmt.Sprintf(starterArchiveURL, starterArchiveRefPath(ref)))
	if err != nil {
		return "", fmt.Errorf("failed to download starter archive: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download starter archive, status code: %d", resp.StatusCode)
	}

	archiveFile, err := os.CreateTemp("", "hyperbricks-starter-*.zip")
	if err != nil {
		return "", fmt.Errorf("failed to create temp archive file: %w", err)
	}
	complete := false
	defer func() {
		if !complete {
			_ = archiveFile.Close()
			_ = os.Remove(archiveFile.Name())
		}
	}()

	written, err := io.Copy(archiveFile, io.LimitReader(resp.Body, starterMaxArchiveBytes+1))
	if err != nil {
		return "", fmt.Errorf("failed to write starter archive: %w", err)
	}
	if written > starterMaxArchiveBytes {
		return "", fmt.Errorf("starter archive exceeds %d bytes", starterMaxArchiveBytes)
	}

	if err := archiveFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close starter archive: %w", err)
	}

	complete = true
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
	var extractedBytes int64

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
		if matched > starterMaxModuleEntries {
			return fmt.Errorf("starter module has more than %d archive entries", starterMaxModuleEntries)
		}
		if file.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("starter module contains unsupported symlink: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, file.Mode()); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
			continue
		}
		remaining := starterMaxModuleBytes - extractedBytes
		if file.UncompressedSize64 > uint64(remaining) {
			return fmt.Errorf("starter module exceeds %d extracted bytes", starterMaxModuleBytes)
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

		copied, err := io.Copy(out, io.LimitReader(source, remaining+1))
		if err != nil {
			out.Close()
			source.Close()
			return fmt.Errorf("failed to write file %s: %w", targetPath, err)
		}
		if copied > remaining {
			out.Close()
			source.Close()
			return fmt.Errorf("starter module exceeds %d extracted bytes", starterMaxModuleBytes)
		}
		extractedBytes += copied
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
