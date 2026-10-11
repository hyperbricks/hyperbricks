package commands

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/language"
	"github.com/spf13/cobra"
)

type lintDiagnostic struct {
	File     string         `json:"file"`
	Line     int            `json:"line,omitempty"`
	Column   int            `json:"column,omitempty"`
	Range    language.Range `json:"range"`
	Severity string         `json:"severity"`
	Code     string         `json:"code"`
	Message  string         `json:"message"`
}

type lintReport struct {
	SchemaVersion int              `json:"schema_version"`
	Status        string           `json:"status"`
	Strict        bool             `json:"strict"`
	FilesChecked  int              `json:"files_checked"`
	Warnings      int              `json:"warnings"`
	Errors        int              `json:"errors"`
	Diagnostics   []lintDiagnostic `json:"diagnostics"`
}

// NewLintCommand exposes the same metadata-type and style checks used by LSP.
func NewLintCommand() *cobra.Command {
	var module string
	var jsonOutput, strict bool
	cmd := &cobra.Command{
		Use:   "lint [files...]",
		Short: "Check YAML types and style without changing source files",
		Long:  "Check YAML syntax, the moduleversion string type, and unnecessary quotes. With no file arguments, scan regular *.hyperbricks.yaml files recursively in the selected module. Explicit files are relative to the working directory. Imports are not followed and no runtime code is executed.",
		RunE: func(cmd *cobra.Command, files []string) error {
			if len(files) > 0 && cmd.Flags().Changed("module") {
				return fmt.Errorf("use either explicit files or --module, not both")
			}
			report := runLint(module, files, strict)
			Exit, ExitCode = true, 0
			if report.Errors > 0 || strict && report.Warnings > 0 {
				ExitCode = 1
			}
			if jsonOutput {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetEscapeHTML(false)
				return encoder.Encode(report)
			}
			for _, diagnostic := range report.Diagnostics {
				location := diagnostic.File
				if diagnostic.Line > 0 {
					location = fmt.Sprintf("%s:%d:%d", location, diagnostic.Line, diagnostic.Column)
				}
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s: %s %s: %s\n",
					doctorTerminalText(location),
					diagnostic.Severity, diagnostic.Code, doctorTerminalText(diagnostic.Message)); err != nil {
					return err
				}
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "Lint: %d file(s), %d warning(s), %d error(s).\n", report.FilesChecked, report.Warnings, report.Errors)
			return err
		},
	}
	cmd.Flags().StringVarP(&module, "module", "m", "default", "module name or directory path (when no files are given)")
	_ = cmd.RegisterFlagCompletionFunc("module", completeModuleSelection)
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "machine-readable output")
	cmd.Flags().BoolVar(&strict, "strict", false, "exit unsuccessfully when style warnings are found")
	return cmd
}

func runLint(module string, files []string, strict bool) lintReport {
	report := lintReport{SchemaVersion: 1, Status: "clean", Strict: strict, Diagnostics: []lintDiagnostic{}}
	addError := func(file, code, message string) {
		report.Diagnostics = append(report.Diagnostics, lintDiagnostic{File: filepath.ToSlash(file), Severity: "error", Code: code, Message: message})
		report.Errors++
	}
	cwd, err := os.Getwd()
	if err != nil {
		addError("", "lint.input", "Cannot resolve the working directory.")
	} else if len(files) == 0 {
		root, selectionErr := resolveModuleRoot(module, cwd)
		if selectionErr != nil {
			addError(module, "lint.input", selectionErr.Error())
		} else {
			info, statErr := os.Stat(root)
			if statErr != nil || !info.IsDir() {
				addError(root, "lint.input", "Module directory is unavailable; select it with --module or provide explicit files.")
			} else {
				// Resolve an explicitly selected module symlink, but do not follow
				// symlinks discovered inside the directory tree.
				resolvedRoot, resolveErr := filepath.EvalSymlinks(root)
				if resolveErr != nil {
					addError(root, "lint.input", resolveErr.Error())
				} else if walkErr := filepath.WalkDir(resolvedRoot, func(path string, entry fs.DirEntry, walkErr error) error {
					if walkErr != nil {
						addError(path, "lint.read", "Cannot read this directory entry.")
						return nil
					}
					if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".hyperbricks.yaml") {
						files = append(files, path)
					}
					return nil
				}); walkErr != nil {
					addError(root, "lint.read", walkErr.Error())
				}
			}
		}
		if len(files) == 0 && report.Errors == 0 {
			addError(root, "lint.input", "No *.hyperbricks.yaml files found in the selected module.")
		}
	}
	unique := make(map[string]bool)
	for _, file := range files {
		absolute, absErr := filepath.Abs(file)
		if absErr != nil {
			addError(file, "lint.input", absErr.Error())
			continue
		}
		unique[absolute] = true
	}
	paths := make([]string, 0, len(unique))
	for file := range unique {
		paths = append(paths, file)
	}
	sort.Strings(paths)
	for _, file := range paths {
		display := file
		if relative, relErr := filepath.Rel(cwd, file); relErr == nil {
			display = filepath.ToSlash(relative)
		}
		info, statErr := os.Lstat(file)
		if statErr != nil || !info.Mode().IsRegular() {
			addError(display, "lint.read", "Expected a readable regular file; directories and symbolic links are not linted.")
			continue
		}
		raw, readErr := os.ReadFile(file)
		if readErr != nil {
			addError(display, "lint.read", "Cannot read this file.")
			continue
		}
		report.FilesChecked++
		for _, diagnostic := range language.LintDocument(string(raw)) {
			severity := "warning"
			if diagnostic.Severity == language.DiagnosticSeverityError {
				severity = "error"
				report.Errors++
			} else {
				report.Warnings++
			}
			report.Diagnostics = append(report.Diagnostics, lintDiagnostic{
				File: display, Line: diagnostic.Range.Start.Line + 1, Column: diagnostic.Range.Start.Character + 1,
				Range: diagnostic.Range, Severity: severity, Code: diagnostic.Code, Message: diagnostic.Message,
			})
		}
	}
	for i := range report.Diagnostics {
		if filepath.IsAbs(report.Diagnostics[i].File) && cwd != "" {
			if relative, relErr := filepath.Rel(cwd, report.Diagnostics[i].File); relErr == nil {
				report.Diagnostics[i].File = filepath.ToSlash(relative)
			}
		}
	}
	if report.Errors > 0 {
		report.Status = "error"
	} else if report.Warnings > 0 {
		report.Status = "warning"
	}
	return report
}
