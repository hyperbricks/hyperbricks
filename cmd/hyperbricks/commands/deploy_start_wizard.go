package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func RunDeployStartWizard() {
	selected, ok, err := RunModulePicker("Select a deploy module")
	if err != nil {
		failf("Error selecting module: %v\n", err)
		Exit = true
		return
	}
	if !ok {
		Exit = true
		return
	}
	StartModule = selected

	reader := bufio.NewReader(os.Stdin)
	deployDir, err := promptInput(reader, "Deploy directory (default deploy): ")
	if err != nil {
		failf("Error reading deploy directory: %v\n", err)
		Exit = true
		return
	}
	deployDir = strings.TrimSpace(deployDir)
	if deployDir == "" {
		deployDir = "deploy"
	}
	StartDeployDir = deployDir

	indexPath := filepath.Join(deployDir, StartModule, versionIndexFile)
	index, err := loadBuildIndex(indexPath)
	if err != nil {
		failf("Error reading build index: %v\n", err)
		Exit = true
		return
	}
	if len(index.Versions) == 0 {
		failf("no builds found in %s; create an archive with hyperbricks build first", indexPath)
		Exit = true
		return
	}

	if index.Current != "" {
		useCurrent, err := promptYesNoDefault(reader, fmt.Sprintf("Use current build (%s)? (Y/n): ", index.Current), true)
		if err != nil {
			failf("Error reading input: %v\n", err)
			Exit = true
			return
		}
		if useCurrent {
			StartBuildID = ""
			return
		}
	}

	rows := buildIndexRowsWithCurrentFirst(index)
	selectedID, ok, err := RunBuildIDPicker("Select build to start", rows, index.Current)
	if err != nil {
		failf("Error selecting build ID: %v\n", err)
		Exit = true
		return
	}
	if !ok {
		Exit = true
		return
	}
	StartBuildID = selectedID
}
