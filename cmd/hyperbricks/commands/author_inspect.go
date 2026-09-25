package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/spaces"
)

type authorInspectionComponent struct {
	Path      string `json:"path"`
	Type      string `json:"type"`
	Source    string `json:"source,omitempty"`
	Reference string `json:"reference,omitempty"`
}

type authorInspectionField struct {
	Path      []string               `json:"path"`
	Key       string                 `json:"key"`
	Type      string                 `json:"type"`
	Label     string                 `json:"label"`
	Required  bool                   `json:"required"`
	Rows      int                    `json:"rows,omitempty"`
	Max       int                    `json:"max,omitempty"`
	Upload    *spaces.UploadPolicy   `json:"upload,omitempty"`
	Directory *spaces.Directory      `json:"directory,omitempty"`
	Edit      *spaces.DocumentPolicy `json:"edit,omitempty"`
	Order     int                    `json:"order"`
	Group     string                 `json:"group,omitempty"`
}

type authorInspection struct {
	Version        int                         `json:"version"`
	Revision       string                      `json:"revision,omitempty"`
	Target         string                      `json:"target"`
	Root           string                      `json:"root"`
	File           string                      `json:"file"`
	Inherit        string                      `json:"inherit,omitempty"`
	Metadata       map[string]interface{}      `json:"metadata"`
	Components     []authorInspectionComponent `json:"components"`
	Templates      []string                    `json:"templates"`
	EditableFields []authorInspectionField     `json:"editable_fields"`
	Checks         []string                    `json:"checks"`
	Diagnostics    []authorDiagnostic          `json:"diagnostics"`
}

type authorInspectionResult struct {
	Target     string            `json:"target"`
	Inspection *authorInspection `json:"inspection,omitempty"`
	Error      string            `json:"error,omitempty"`
}

type authorInspections struct {
	Version  int                      `json:"version"`
	Revision string                   `json:"revision"`
	Results  []authorInspectionResult `json:"results"`
}

func inspectAuthorTargets(ctx *authorContext, targets []string) (authorInspections, bool) {
	result := authorInspections{Version: 2, Revision: ctx.Revision, Results: []authorInspectionResult{}}
	failed := false
	for _, target := range targets {
		entry := authorInspectionResult{Target: target}
		inspection, err := inspectAuthorContext(ctx, target)
		if err != nil {
			entry.Error, failed = err.Error(), true
		} else {
			entry.Inspection = inspection
			// The envelope owns the shared snapshot revision.
			entry.Inspection.Revision = ""
		}
		result.Results = append(result.Results, entry)
	}
	return result, failed
}

func inspectAuthorContext(ctx *authorContext, target string) (*authorInspection, error) {
	parts := strings.Split(target, ".")
	var effective map[string]interface{}
	result := &authorInspection{Version: 2, Revision: ctx.Revision, Target: target, Root: parts[0],
		Metadata: map[string]interface{}{}, Components: []authorInspectionComponent{}, Templates: []string{},
		EditableFields: []authorInspectionField{}, Diagnostics: []authorDiagnostic{},
		Checks: []string{"loaded_configuration", "resolved_inheritance", "editing_contracts", "template_file_presence"}}
	for _, root := range ctx.Roots {
		if root.Name == parts[0] {
			effective, result.File = root.Effective, root.File
		}
	}
	for _, part := range parts[1:] {
		effective, _ = effective[part].(map[string]interface{})
	}
	if effective == nil {
		return nil, authorUnknownTarget(ctx, target)
	}
	owner, err := filepath.Rel(ctx.Module, filepath.Join(ctx.Directories["hyperbricks"], filepath.FromSlash(result.File)))
	if err != nil {
		return nil, err
	}
	for _, source := range ctx.Files {
		if source.Path != filepath.ToSlash(owner) {
			continue
		}
		doc, err := sourceYAML([]byte(source.YAML))
		if err != nil {
			return nil, err
		}
		node := yget(doc, parts[0])
		if node != nil {
			if inherit := yget(node, "inherit"); inherit != nil {
				result.Inherit = inherit.Value
			}
		}
	}
	for _, key := range []string{"route", "title", "section", "index"} {
		if v, ok := effective[key]; ok {
			result.Metadata[key] = v
		}
	}
	fields, err := spaces.SourceFields(effective)
	if err != nil {
		return nil, err
	}
	for _, field := range fields {
		result.EditableFields = append(result.EditableFields, authorInspectionField{
			Path: field.Path, Key: field.Key, Type: field.Type, Label: field.Label,
			Required: field.Required, Rows: field.Rows, Max: field.Max,
			Upload: field.Upload, Directory: field.Directory, Edit: field.Edit, Order: field.Order, Group: field.Group})
	}
	seen := map[string]bool{}
	var visit func(map[string]interface{}, string)
	visit = func(obj map[string]interface{}, path string) {
		if token, ok := obj["@type"].(string); ok {
			entry := authorInspectionComponent{Path: path, Type: strings.ToLower(strings.Trim(token, "<>"))}
			if file, ok := obj["template"].(string); ok {
				entry.Source, entry.Reference = "template_file", file
				if !seen[file] {
					seen[file] = true
					result.Templates = append(result.Templates, file)
					full, err := authoringPath(ctx.Directories["templates"], file)
					if err == nil {
						var info os.FileInfo
						info, err = os.Stat(full)
						if err == nil && !info.Mode().IsRegular() {
							err = fmt.Errorf("not a regular file")
						}
					}
					if err != nil {
						result.Diagnostics = append(result.Diagnostics, authorDiagnostic{Severity: "error", Code: "template_reference", Path: path, Message: fmt.Sprintf("template %s: %v", file, err)})
					}
				}
			} else if file, ok := obj["file"].(string); ok {
				entry.Source, entry.Reference = "file", file
			} else if _, ok := obj["inline"]; ok {
				entry.Source = "inline"
			} else if _, ok := obj["content"]; ok && entry.Type == "markdown" {
				entry.Source = "inline"
			} else if _, ok := obj["value"]; ok {
				entry.Source = "inline"
			}
			result.Components = append(result.Components, entry)
		}
		keys := make([]string, 0, len(obj))
		for key := range obj {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if child, ok := obj[key].(map[string]interface{}); ok {
				visit(child, path+"."+key)
			}
		}
	}
	visit(effective, target)
	sort.Strings(result.Templates)
	return result, nil
}
