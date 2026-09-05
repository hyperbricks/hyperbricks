package commands

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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
		fmt.Printf("Directory already exists: %s\n", dir)
		return nil
	}
	if os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		fmt.Printf("Created directory: %s\n", dir)
		return nil
	}
	return fmt.Errorf("failed to inspect directory %s: %w", dir, err)
}

// createModuleDirectories sets up the standard directory layout for a module
// under ./modules/<module>, plus a global ./bin/plugins directory.
func createModuleDirectories(module string) {
	for _, dir := range initModuleDirectories(module) {
		if err := ensureDir(dir); err != nil {
			fmt.Println(err)
		}
	}
}

func initModuleDirectories(module string) []string {
	baseDir := "modules"
	moduleDir := filepath.Join(baseDir, module)
	subDirs := []string{
		"rendered",
		"static",
		"hyperbricks",
		"resources",
		"templates",
		"logs",
	}

	dirs := []string{
		baseDir,
		moduleDir,
	}
	for _, sub := range subDirs {
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
	plan := initPlan{directories: initModuleDirectories(moduleName)}
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
	plan.files = append(plan.files, initFile{
		source: defaultConfigPath,
		target: filepath.Join(moduleDir, PackageConfigFileName),
		data:   defaultConfigContent,
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
			fmt.Printf("Skipping existing file: %s\n", file.target)
			continue
		}
		if file.source == "assets/default-config.hyperbricks.yaml" {
			fmt.Printf("Config file created successfully at %s\n", file.target)
		} else {
			fmt.Printf("Extracted file: %s -> %s\n", file.source, file.target)
		}
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

// NewInitCommand creates the "init" subcommand.
func NewInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "init",
		Short:         "Create package.hyperbricks.yaml and required directories",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			Exit = true
			ExitCode = 0
			if err := initializeModule(module); err != nil {
				ExitCode = 1
				return fmt.Errorf("initialize module: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&module, "module", "m", "default", "module name below ./modules")
	return cmd
}
