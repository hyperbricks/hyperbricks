package shared

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

// ComponentError represents an error associated with a component.
type ComponentError struct {
	Hash           string // non-collision hash
	File           string // Hyperbricks file
	Err            string // A descriptive error message
	Key            string // Current key of the component where the error occured
	Path           string // Path of the component in the hierarchy
	Rejected       bool   // Rendering is rejected
	Type           string // Which HyperBricks type?
	Level          string // INFO, WARNING, otherwise ERROR
	Line           int
	Column         int
	Resource       string
	ResourceLine   int
	ResourceColumn int
	Phase          string
	Cause          error `json:"-"`
}

// ComponentError represents an error associated with a component.
type CompositeError struct {
	Err      string // A descriptive error message
	Key      string // Key of the component
	Path     string // Path of the component in the hierarchy
	Rejected bool   // Rendering is rejected
	Type     string // Which HyperBricks type?
	Level    string // INFO, WARNING, otherwise ERROR
}

// Error implements the error interface for ComponentError.
func (e CompositeError) Error() string {
	return e.Err
}

// Error implements the error interface for ComponentError.
func (e ComponentError) Error() string {
	return e.Err
}

func (e ComponentError) Unwrap() error { return e.Cause }

// AsComponentError preserves structured metadata through ordinary Go wrapping.
func AsComponentError(err error) (ComponentError, bool) {
	var value ComponentError
	if errors.As(err, &value) {
		return value, true
	}
	var pointer *ComponentError
	if errors.As(err, &pointer) && pointer != nil {
		return *pointer, true
	}
	return ComponentError{}, false
}

// Diagnostic fills only missing context, so parents never relabel child errors.
func Diagnostic(err error, meta Meta, phase string) ComponentError {
	diagnostic, structured := AsComponentError(err)
	if !structured {
		diagnostic = ComponentError{Err: err.Error(), Cause: err}
	} else if diagnostic.Err != err.Error() {
		diagnostic.Err, diagnostic.Cause = err.Error(), err
	}
	if structured {
		diagnostic.Cause = err
	}
	if structured && diagnostic.Type != "" && meta.ConfigType != "" && diagnostic.Type != meta.ConfigType {
		return diagnostic
	}
	local := diagnostic.Path == "" || diagnostic.Path == meta.HyperBricksPath
	if diagnostic.File == "" {
		diagnostic.File = meta.HyperBricksFile
	}
	if diagnostic.Path == "" {
		diagnostic.Path = meta.HyperBricksPath
	}
	if diagnostic.Key == "" {
		diagnostic.Key = meta.HyperBricksKey
	}
	if diagnostic.Type == "" {
		diagnostic.Type = meta.ConfigType
	}
	if diagnostic.Phase == "" {
		diagnostic.Phase = phase
	}
	if local && meta.Source != nil {
		if diagnostic.File == "" {
			diagnostic.File = meta.Source.File
		}
		if diagnostic.Line == 0 {
			diagnostic.Line, diagnostic.Column = meta.Source.Line, meta.Source.Column
		}
	}
	return diagnostic
}

func ResourceDiagnostic(err error, meta Meta, phase, field string) ComponentError {
	diagnostic := Diagnostic(err, meta, phase)
	if meta.Source != nil {
		if field == "template" && meta.Resource(field) == "" {
			if _, inline := meta.Source.Fields["inline"]; inline {
				field = "inline"
			}
		}
		if location, ok := meta.Source.Fields[field]; ok {
			diagnostic.File, diagnostic.Line, diagnostic.Column = location.File, location.Line, location.Column
		}
		diagnostic.Resource = meta.Resource(field)
	}
	if field == "template" || field == "inline" {
		// html/template execution errors expose positions only in their message.
		if match := templateErrorPosition.FindStringSubmatch(err.Error()); match != nil {
			diagnostic.ResourceLine, _ = strconv.Atoi(match[1])
			diagnostic.ResourceColumn, _ = strconv.Atoi(match[2])
		}
	}
	return diagnostic
}

var templateErrorPosition = regexp.MustCompile(`template: .*?:([0-9]+)(?::([0-9]+))?:`)

func EnrichDiagnostics(diagnostics []error, meta Meta, phase string) []error {
	for index, err := range diagnostics {
		if err != nil {
			diagnostics[index] = Diagnostic(err, meta, phase)
		}
	}
	return diagnostics
}

func HasDiagnosticFailure(diagnostics []error) bool {
	for _, err := range diagnostics {
		if err == nil {
			continue
		}
		diagnostic, ok := AsComponentError(err)
		if ok && !diagnostic.Rejected && (strings.EqualFold(diagnostic.Level, "WARNING") || strings.EqualFold(diagnostic.Level, "WARN") || strings.EqualFold(diagnostic.Level, "INFO")) {
			continue
		}
		return true
	}
	return false
}
