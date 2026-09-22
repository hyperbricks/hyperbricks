package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func RunBuildWizard() {
	selected, ok, err := RunModulePicker("Select a module")
	if err != nil {
		failf("Error selecting module: %v\n", err)
		Exit = true
		return
	}
	if !ok {
		Exit = true
		return
	}
	buildModule = selected

	reader := bufio.NewReader(os.Stdin)

	format, err := promptBuildFormat(reader)
	if err != nil {
		failf("Error reading format: %v\n", err)
		Exit = true
		return
	}
	buildZip = format == "zip"
	buildHRA = format == "hra"

	outDir, err := promptInput(reader, "Output directory (default deploy): ")
	if err != nil {
		failf("Error reading output directory: %v\n", err)
		Exit = true
		return
	}
	outDir = strings.TrimSpace(outDir)
	if outDir == "" {
		outDir = "deploy"
	}
	buildOutDir = outDir

	force, err := promptYesNo(reader, "Force rebuild even if unchanged? (y/N): ")
	if err != nil {
		failf("Error reading input: %v\n", err)
		Exit = true
		return
	}
	buildForce = force

	replaceTarget, err := promptReplaceTarget(reader, buildOutDir, buildModule)
	if err != nil {
		failf("Error reading replace target: %v\n", err)
		Exit = true
		return
	}
	buildReplaceTarget = replaceTarget

	if _, err := runBuild(); err != nil {
		failf("Error building archive: %v\n", err)
		Exit = true
		return
	}
}

func promptBuildFormat(reader *bufio.Reader) (string, error) {
	for {
		input, err := promptInput(reader, "Build format (hra/zip) [hra]: ")
		if err != nil {
			return "", err
		}
		input = strings.ToLower(strings.TrimSpace(input))
		if input == "" || input == "hra" {
			return "hra", nil
		}
		if input == "zip" {
			return "zip", nil
		}
		fmt.Fprintln(os.Stderr, "Please enter 'hra' or 'zip'.")
	}
}

func promptReplaceTarget(reader *bufio.Reader, outDir string, module string) (string, error) {
	indexPath := filepath.Join(outDir, module, versionIndexFile)
	index, err := loadBuildIndex(indexPath)
	if err != nil {
		return "", err
	}
	if len(index.Versions) == 0 {
		return "", nil
	}

	for {
		input, err := promptInput(reader, "Replace build? (n=none, c=current, p=pick id): ")
		if err != nil {
			return "", err
		}
		input = strings.ToLower(strings.TrimSpace(input))
		switch input {
		case "", "n", "no":
			return "", nil
		case "c", "current":
			if index.Current == "" {
				fmt.Fprintln(os.Stderr, "No current build set.")
				continue
			}
			return "current", nil
		case "p", "pick":
			rows := buildIndexRowsWithCurrentFirst(index)
			selected, ok, err := RunBuildIDPicker("Select build to replace", rows, index.Current)
			if err != nil {
				return "", err
			}
			if !ok {
				return "", nil
			}
			return selected, nil
		default:
			fmt.Fprintln(os.Stderr, "Please enter n, c, or p.")
		}
	}
}
