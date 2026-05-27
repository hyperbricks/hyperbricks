package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

func main() {
	input, err := readInput(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	doc, err := yamlparser.ParseBytes(input)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	materialized, err := doc.Materialize()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	imports := doc.Imports
	if imports == nil {
		imports = []string{}
	}
	output := map[string]interface{}{
		"imports":      imports,
		"materialized": materialized,
	}
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(encoded.String())
}

func readInput(args []string) ([]byte, error) {
	if len(args) == 0 {
		return io.ReadAll(os.Stdin)
	}
	if len(args) > 1 {
		return nil, fmt.Errorf("usage: hyperbricks-yaml-dryrun [file]")
	}
	return os.ReadFile(args[0])
}
