package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func RunStaticWizard() {
	selected, ok, err := RunModulePicker("Select a module")
	if err != nil {
		fmt.Printf("Error selecting module: %v\n", err)
		Exit = true
		return
	}
	if !ok {
		Exit = true
		return
	}
	StartModule = selected

	reader := bufio.NewReader(os.Stdin)

	force, err := promptYesNo(reader, "Override existing rendered output? (y/N): ")
	if err != nil {
		fmt.Printf("Error reading input: %v\n", err)
		Exit = true
		return
	}
	ForceStatic = force

	exportZip, err := promptYesNo(reader, "Export rendered output to zip? (y/N): ")
	if err != nil {
		fmt.Printf("Error reading input: %v\n", err)
		Exit = true
		return
	}
	ExportZip = exportZip
	if exportZip {
		outDir, err := promptInput(reader, fmt.Sprintf("Zip output directory (default ./exports/%s): ", StartModule))
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			Exit = true
			return
		}
		ExportOutDir = strings.TrimSpace(outDir)
	}

	serve, err := promptYesNo(reader, "Serve rendered files? (y/N): ")
	if err != nil {
		fmt.Printf("Error reading input: %v\n", err)
		Exit = true
		return
	}
	ServeStatic = serve
	if serve {
		port, err := promptPort(reader, "Port (default 8080): ", 8080)
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			Exit = true
			return
		}
		StaticServePort = port
	}

	StaticWizard = true
	RenderStatic = true
}
