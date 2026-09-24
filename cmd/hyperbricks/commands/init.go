package commands

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hyperbricks/hyperbricks/assets"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/packagemetadata"
	"github.com/spf13/cobra"
)

var (
	module string
)

type initFile struct {
	source string
	target string
	data   []byte
}

type initPlan struct {
	moduleDir   string
	directories []string
	files       []initFile
}

// ensureDir checks for the existence of a directory at path.
// If it doesn’t exist, it creates it with mode 0755 and logs the result.
func ensureDir(dir string) error {
	info, err := os.Stat(dir)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("path already exists and is not a directory: %s", dir)
		}
		logging.GetLogger().Named("init").Debugw("Directory exists", "directory", dir)
		return nil
	}
	if os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		logging.GetLogger().Named("init").Debugw("Directory created", "directory", dir)
		return nil
	}
	return fmt.Errorf("failed to inspect directory %s: %w", dir, err)
}

// createModuleDirectories sets up the standard directory layout for a module
// under ./modules/<module>, plus a global ./bin/plugins directory.
func createModuleDirectories(module string) {
	for _, dir := range initModuleDirectories(module) {
		if err := ensureDir(dir); err != nil {
			ReportError(err)
		}
	}
}

var standardModuleSubdirectories = []string{
	"rendered",
	"static",
	"hyperbricks",
	"resources",
	"templates",
	"logs",
}

func initModuleDirectories(module string) []string {
	baseDir := "modules"
	moduleDir := filepath.Join(baseDir, module)

	dirs := []string{
		baseDir,
		moduleDir,
	}
	for _, sub := range standardModuleSubdirectories {
		dirs = append(dirs, filepath.Join(moduleDir, sub))
	}
	return append(dirs, filepath.Join("bin", "plugins"))
}

//go:embed assets/**
var embeddedFiles embed.FS

func validateInitModuleName(value string) (string, error) {
	moduleName := strings.TrimSpace(value)
	if moduleName == "" {
		return "", fmt.Errorf("module name cannot be empty")
	}
	if isModulePath(moduleName) {
		return "", fmt.Errorf("module must be a name below ./modules, not a directory path: %q", value)
	}
	return moduleName, nil
}

func buildInitPlan(moduleName string) (initPlan, error) {
	moduleDir := filepath.Join("modules", moduleName)
	plan := initPlan{moduleDir: moduleDir, directories: initModuleDirectories(moduleName)}
	directorySeen := make(map[string]bool, len(plan.directories))
	for _, dir := range plan.directories {
		directorySeen[dir] = true
	}
	addDirectory := func(dir string) {
		dir = filepath.Clean(dir)
		if !directorySeen[dir] {
			directorySeen[dir] = true
			plan.directories = append(plan.directories, dir)
		}
	}

	const defaultConfigPath = "assets/default-config.hyperbricks.yaml"
	defaultConfigContent, err := embeddedFiles.ReadFile(defaultConfigPath)
	if err != nil {
		return initPlan{}, fmt.Errorf("read embedded default config %s: %w", defaultConfigPath, err)
	}
	metadataResult, err := packagemetadata.ReconcileSource(defaultConfigContent, packagemetadata.ReconcileOptions{
		Module:             moduleName,
		HyperBricks:        strings.TrimSpace(assets.VersionMD),
		ResetModuleVersion: true,
	})
	if err != nil {
		return initPlan{}, fmt.Errorf("prepare embedded package metadata: %w", err)
	}
	plan.files = append(plan.files, initFile{
		source: defaultConfigPath,
		target: filepath.Join(moduleDir, PackageConfigFileName),
		data:   metadataResult.Content,
	})

	const embeddedDir = "assets/default"
	err = fs.WalkDir(embeddedFiles, embeddedDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("access embedded path %s: %w", path, walkErr)
		}
		if path == embeddedDir {
			return nil
		}

		relativePath, err := filepath.Rel(embeddedDir, path)
		if err != nil {
			return fmt.Errorf("resolve embedded path %s: %w", path, err)
		}
		if d.IsDir() {
			addDirectory(filepath.Join(moduleDir, relativePath))
			return nil
		}

		data, err := embeddedFiles.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded file %s: %w", path, err)
		}
		if relativePath == "README.md" {
			shellQuote := func(value string) string {
				return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
			}
			data = []byte(strings.NewReplacer(
				"__MODULE_NAME__", moduleName,
				"__MODULE_SHELL__", shellQuote(moduleName),
				"__EXPORT_SHELL__", shellQuote("exports/"+moduleName),
			).Replace(string(data)))
		}
		if relativePath == "gitignore.txt" {
			relativePath = ".gitignore"
		}
		targetPath := filepath.Join(moduleDir, relativePath)
		addDirectory(filepath.Dir(targetPath))
		plan.files = append(plan.files, initFile{source: path, target: targetPath, data: data})
		return nil
	})
	if err != nil {
		return initPlan{}, err
	}
	return plan, nil
}

func preflightInitPlan(plan initPlan) error {
	for _, dir := range plan.directories {
		info, err := os.Stat(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("failed to inspect directory %s: %w", dir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("path already exists and is not a directory: %s", dir)
		}
	}

	for _, file := range plan.files {
		info, err := os.Stat(file.target)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("failed to inspect file %s: %w", file.target, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("path already exists and is not a regular file: %s", file.target)
		}
	}
	return nil
}

func writeInitFileIfMissing(file initFile) (bool, error) {
	target, err := os.OpenFile(file.target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if os.IsExist(err) {
		info, statErr := os.Stat(file.target)
		if statErr != nil {
			return false, fmt.Errorf("failed to inspect existing file %s: %w", file.target, statErr)
		}
		if !info.Mode().IsRegular() {
			return false, fmt.Errorf("path already exists and is not a regular file: %s", file.target)
		}
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to create file %s: %w", file.target, err)
	}

	written, writeErr := target.Write(file.data)
	if writeErr == nil && written != len(file.data) {
		writeErr = fmt.Errorf("short write: wrote %d of %d bytes", written, len(file.data))
	}
	closeErr := target.Close()
	if writeErr != nil {
		_ = os.Remove(file.target)
		return false, fmt.Errorf("failed to write file %s: %w", file.target, writeErr)
	}
	if closeErr != nil {
		_ = os.Remove(file.target)
		return false, fmt.Errorf("failed to close file %s: %w", file.target, closeErr)
	}
	return true, nil
}

func applyInitPlan(plan initPlan) error {
	for _, dir := range plan.directories {
		if err := ensureDir(dir); err != nil {
			return err
		}
	}
	for _, file := range plan.files {
		created, err := writeInitFileIfMissing(file)
		if err != nil {
			return err
		}
		if !created {
			logging.GetLogger().Named("init").Infow("Existing file preserved", "module", filepath.Base(plan.moduleDir), "file", logging.ModulePath(plan.moduleDir, file.target))
			continue
		}
		logging.GetLogger().Named("init").Infow("File created", "module", filepath.Base(plan.moduleDir), "file", logging.ModulePath(plan.moduleDir, file.target))
	}
	return nil
}

func initializeModule(value string) error {
	moduleName, err := validateInitModuleName(value)
	if err != nil {
		return err
	}
	plan, err := buildInitPlan(moduleName)
	if err != nil {
		return err
	}
	if err := preflightInitPlan(plan); err != nil {
		return err
	}
	return applyInitPlan(plan)
}

func updateExistingModuleMetadata(value string, bump packagemetadata.Bump) (string, packagemetadata.ReconcileResult, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return "", packagemetadata.ReconcileResult{}, fmt.Errorf("resolve current working directory: %w", err)
	}
	selection, err := resolveModuleSelection(value, workingDirectory)
	if err != nil {
		return "", packagemetadata.ReconcileResult{}, fmt.Errorf("resolve module %q: %w", value, err)
	}
	configPath := filepath.Join(selection.Root, PackageConfigFileName)
	result, err := packagemetadata.ReconcileSourceFile(configPath, packagemetadata.ReconcileOptions{
		Module:                    selection.Name,
		HyperBricks:               strings.TrimSpace(assets.VersionMD),
		CanonicalizeModuleVersion: true,
		Bump:                      bump,
	})
	if err != nil {
		return "", packagemetadata.ReconcileResult{}, err
	}
	return configPath, result, nil
}

func writeMetadataUpdate(out io.Writer, path string, result packagemetadata.ReconcileResult) {
	if !result.Changed {
		fmt.Fprintf(out, "Metadata already current: %s\n", path)
		return
	}
	fmt.Fprintf(out, "Updated metadata: %s\n", path)
	for _, change := range result.Changes {
		before := change.Before
		if before == "" {
			before = "(missing)"
		}
		if change.Removed {
			fmt.Fprintf(out, "  %s: %s -> (removed)\n", change.Field, before)
			continue
		}
		fmt.Fprintf(out, "  %s: %s -> %s\n", change.Field, before, change.After)
	}
}

// NewInitCommand creates the "init" subcommand.
func NewInitCommand() *cobra.Command {
	var updateMetadata bool
	var bumpVersion string
	cmd := &cobra.Command{
		Use:           "init",
		Short:         "Create a three-page HyperBricks Starter with HTMX 4",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			Exit = true
			ExitCode = 0
			bumpChanged := cmd.Flags().Changed("bump-version")
			if updateMetadata || bumpChanged {
				bump := packagemetadata.BumpNone
				if bumpChanged {
					value := strings.ToLower(strings.TrimSpace(bumpVersion))
					if value == "" {
						ExitCode = 1
						return errors.New("bump version cannot be empty")
					}
					bump = packagemetadata.Bump(value)
				}
				path, result, err := updateExistingModuleMetadata(module, bump)
				if err != nil {
					ExitCode = 1
					return fmt.Errorf("update module metadata: %w", err)
				}
				writeMetadataUpdate(cmd.OutOrStdout(), path, result)
				return nil
			}
			if err := initializeModule(module); err != nil {
				ExitCode = 1
				return fmt.Errorf("initialize module: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Module ready: modules/%s\n", module)
			fmt.Fprintf(cmd.OutOrStdout(), "Start: hyperbricks start -m %s\n", module)
			return nil
		},
	}

	cmd.Flags().StringVarP(&module, "module", "m", "default", "module name below ./modules; metadata modes also accept a directory path")
	_ = cmd.RegisterFlagCompletionFunc("module", completeModuleSelection)
	cmd.Flags().BoolVar(&updateMetadata, "update-metadata", false, "refresh stable metadata in an existing module without changing its scaffold")
	cmd.Flags().StringVar(&bumpVersion, "bump-version", "", "bump module version: patch, minor, or major (default patch)")
	cmd.Flags().Lookup("bump-version").NoOptDefVal = "patch"
	return cmd
}
