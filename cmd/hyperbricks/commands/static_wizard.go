package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
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

func promptYesNo(reader *bufio.Reader, prompt string) (bool, error) {
	for {
		fmt.Print(prompt)
		input, err := readLine(reader)
		if err != nil {
			return false, err
		}
		input = strings.ToLower(strings.TrimSpace(input))
		if input == "" {
			return false, nil
		}
		if input == "y" || input == "yes" {
			return true, nil
		}
		if input == "n" || input == "no" {
			return false, nil
		}
		fmt.Println("Please enter y or n.")
	}
}

func promptInput(reader *bufio.Reader, prompt string) (string, error) {
	fmt.Print(prompt)
	return readLine(reader)
}

func promptPort(reader *bufio.Reader, prompt string, defaultPort int) (int, error) {
	for {
		fmt.Print(prompt)
		input, err := readLine(reader)
		if err != nil {
			return 0, err
		}
		input = strings.TrimSpace(input)
		if input == "" {
			return defaultPort, nil
		}
		port, err := strconv.Atoi(input)
		if err != nil || port <= 0 || port > 65535 {
			fmt.Println("Please enter a valid port number.")
			continue
		}
		return port, nil
	}
}

func readLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
