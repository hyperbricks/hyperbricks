package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	converter "github.com/hyperbricks/hyperbricks/pkg/legacy-yaml-converter"
)

func main() {
	input := flag.String("in", "", "legacy .hyperbricks file or directory")
	output := flag.String("out", "", "target .hyperbricks.yaml file or directory")
	flag.Parse()

	if *input == "" {
		fmt.Fprintln(os.Stderr, "usage: hyperbricks-yaml-convert -in <file-or-dir> [-out <file-or-dir>]")
		os.Exit(2)
	}

	info, err := os.Stat(*input)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if info.IsDir() {
		target := *output
		if target == "" {
			target = *input
		}
		written, err := converter.ConvertDirectory(*input, target)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		for _, file := range written {
			fmt.Println(file)
		}
		return
	}

	result, err := converter.ConvertFile(*input, converter.Options{SourcePath: *input})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *output == "" {
		fmt.Print(result.YAML)
		return
	}
	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(*output, []byte(result.YAML), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
