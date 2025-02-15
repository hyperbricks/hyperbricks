package commands

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	module string
)

func createHbConfig(module string) {
	// Module code template
	moduleCode := `# here you can set the current module
$module = modules/%s

`

	// Get the current working directory
	dir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Failed to get the working directory: %v\n", err)
		return
	}

	// Path to the new module file
	newConfigPath := filepath.Join(dir, fmt.Sprintf("modules/%s/package.hyperbricks", module))

	// Read the embedded default config content
	defaultConfigPath := "assets/default-config.hyperbricks"
	defaultConfigContent, err := embeddedFiles.ReadFile(defaultConfigPath)
	if err != nil {
		fmt.Printf("Failed to read the embedded default config file (%s): %v\n", defaultConfigPath, err)
		return
	}

	// Prepare the top lines with the module code
	topLines := fmt.Sprintf(moduleCode, module)

	// Combine top lines with the default config content
	newConfigContent := topLines + string(defaultConfigContent)

	// Write the combined content to the new module file
	err = os.WriteFile(newConfigPath, []byte(newConfigContent), 0644)
	if err != nil {
		fmt.Printf("Failed to write the new config file: %v\n", err)
		return
	}

	fmt.Printf("Config file created successfully at %s\n", newConfigPath)
}

func createModuleDirectories(module string) {
	// Define the base path for modules
	baseDir := "./modules"
	moduleDir := filepath.Join(baseDir, module)

	// List of subdirectories to create under the module directory
	subDirs := []string{"rendered", "static", "hyperbricks", "resources", "templates"}

	// Check if the base directory exists, create it if not
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		err = os.Mkdir(baseDir, 0755)
		if err != nil {
			fmt.Printf("Failed to create base directory %s: %v\n", baseDir, err)
			return
		}
		fmt.Printf("Created base directory: %s\n", baseDir)
	} else {
		fmt.Printf("Base directory already exists: %s\n", baseDir)
	}

	// Check if the module directory exists, create it if not
	if _, err := os.Stat(moduleDir); os.IsNotExist(err) {
		err = os.Mkdir(moduleDir, 0755)
		if err != nil {
			fmt.Printf("Failed to create module directory %s: %v\n", moduleDir, err)
			return
		}
		fmt.Printf("Created module directory: %s\n", moduleDir)
	} else {
		fmt.Printf("Module directory already exists: %s\n", moduleDir)
	}

	// Create the subdirectories under the module directory
	for _, subDir := range subDirs {
		subDirPath := filepath.Join(moduleDir, subDir)
		if _, err := os.Stat(subDirPath); os.IsNotExist(err) {
			err = os.Mkdir(subDirPath, 0755)
			if err != nil {
				fmt.Printf("Failed to create subdirectory %s: %v\n", subDirPath, err)
				continue
			}
			fmt.Printf("Created subdirectory: %s\n", subDirPath)
		} else {
			fmt.Printf("Subdirectory already exists: %s\n", subDirPath)
		}
	}
}

//go:embed assets/**
var embeddedFiles embed.FS

func extractEmbeddedFiles(module string) {
	// Define the base path for the module
	baseDir := filepath.Join("./modules", module)

	// Ensure the base directory exists
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		fmt.Printf("Failed to create base directory %s: %v\n", baseDir, err)
		return
	}

	// Access the embedded directory
	embeddedDir := "assets/default"

	// Walk through the embedded files
	fs.WalkDir(embeddedFiles, embeddedDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Printf("Error accessing embedded path %s: %v\n", path, err)
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Determine the relative path
		relativePath, err := filepath.Rel(embeddedDir, path)
		if err != nil {
			fmt.Printf("Error getting relative path for %s: %v\n", path, err)
			return err
		}

		// Determine the target path
		targetPath := filepath.Join(baseDir, relativePath)

		// Ensure the target directory exists
		targetDir := filepath.Dir(targetPath)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			fmt.Printf("Failed to create directory %s: %v\n", targetDir, err)
			return err
		}

		// Check if the target file already exists
		if _, err := os.Stat(targetPath); err == nil {
			fmt.Printf("Skipping existing file: %s\n", targetPath)
			return nil // Skip writing if file already exists
		} else if !os.IsNotExist(err) {
			fmt.Printf("Error checking file existence %s: %v\n", targetPath, err)
			return err // Return if there's another error
		}

		// Read the embedded file
		data, err := embeddedFiles.ReadFile(path)
		if err != nil {
			fmt.Printf("Failed to read embedded file %s: %v\n", path, err)
			return err
		}

		// Write the file to the target directory
		if err := os.WriteFile(targetPath, data, 0644); err != nil {
			fmt.Printf("Failed to write file %s: %v\n", targetPath, err)
			return err
		}

		fmt.Printf("Extracted file: %s -> %s\n", path, targetPath)
		return nil
	})
}

// NewGoodbyeCommand creates the "goodbye" subcommand
func NewInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create package.hyperbricks and required directories",
		Run: func(cmd *cobra.Command, args []string) {

			createModuleDirectories(module)
			createHbConfig(module)
			extractEmbeddedFiles(module)
			os.Exit(0)
		},
	}

	cmd.Flags().StringVarP(&module, "module", "m", "default", "name-of-module")
	return cmd
}
