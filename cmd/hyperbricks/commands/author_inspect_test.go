package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAuthorInspectionIsLeanAndReadOnly(t *testing.T) {
	root := starterAuthorModule(t)
	ctx, err := authorProject(root, "")
	if err != nil {
		t.Fatal(err)
	}
	focus, err := focusAuthorContext(ctx, "scaffold_page")
	if err != nil {
		t.Fatal(err)
	}
	spec := *focus.Example
	spec.Brick.Children[0].Children[0].Children[0].Properties["content"] = strings.Repeat("Source-only body marker.\n", 100)
	if r := runAuthor(t, root, spec, false); r.Status != "created" {
		t.Fatal(r.Error)
	}
	ctx, err = authorProject(root, "")
	if err != nil {
		t.Fatal(err)
	}
	before := scaffoldState(t, root)
	cmd := NewAuthorCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"inspect", "-m", root, "--target", spec.Brick.Name, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var inspect authorInspection
	if err := json.Unmarshal(out.Bytes(), &inspect); err != nil {
		t.Fatal(err)
	}
	if inspect.Revision != ctx.Revision || inspect.Inherit != "scaffold_page" || inspect.Metadata["route"] != "about" || inspect.File != spec.File {
		t.Fatalf("lost ownership: %+v", inspect)
	}
	found := false
	for _, component := range inspect.Components {
		if component.Path == spec.Brick.Name+".content.values.body.intro" && component.Type == "markdown" && component.Source == "inline" {
			found = true
		}
	}
	if !found || !reflect.DeepEqual(inspect.Templates, []string{"shell.html"}) || len(inspect.Diagnostics) != 0 {
		t.Fatalf("lost bindings/references: %+v", inspect)
	}
	focus, err = focusAuthorContext(ctx, spec.Brick.Name)
	if err != nil {
		t.Fatal(err)
	}
	full, _ := json.Marshal(focus)
	if out.Len() >= len(full) || strings.Contains(out.String(), "Source-only body marker") || strings.Contains(out.String(), "{{.body}}") {
		t.Fatal("inspection repeats source")
	}
	if !reflect.DeepEqual(before, scaffoldState(t, root)) {
		t.Fatal("inspection wrote files")
	}
	t.Logf("rich context=%d bytes inspection=%d bytes", len(full), out.Len())
}

func TestAuthorInspectionMultipleTargetsRetainsFailures(t *testing.T) {
	root := starterAuthorModule(t)
	before := scaffoldState(t, root)
	for _, targets := range [][]string{{"scaffold_page", "scaffold_page.content"}, {"scaffold_page", "missing", "scaffold_page"}} {
		cmd := NewAuthorCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		args := []string{"inspect", "-m", root, "--json"}
		for _, target := range targets {
			args = append(args, "--target", target)
		}
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		var result authorInspections
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Revision == "" || len(result.Results) != len(targets) {
			t.Fatalf("lost snapshot/results: %+v", result)
		}
		for i, target := range targets {
			entry := result.Results[i]
			if entry.Target != target {
				t.Fatal("target order changed")
			}
			if target == "missing" {
				if entry.Error == "" || entry.Inspection != nil || ExitCode != 1 {
					t.Fatalf("hidden failure: %+v exit=%d", entry, ExitCode)
				}
			} else if entry.Inspection == nil || entry.Error != "" || entry.Inspection.Revision != "" {
				t.Fatalf("lost healthy result: %+v", entry)
			}
		}
	}
	if !reflect.DeepEqual(before, scaffoldState(t, root)) {
		t.Fatal("inspection wrote files")
	}
}

func TestAuthorInspectionEditingMetadataAndNestedTargets(t *testing.T) {
	root := editableAuthorModule(t, "{body: {type: textarea, label: Body copy, rows: 8, max: 4000}, caption: text}")
	ctx, err := authorProject(root, "")
	if err != nil {
		t.Fatal(err)
	}
	value, err := inspectAuthorContext(ctx, "base.content")
	if err != nil {
		t.Fatal(err)
	}
	if len(value.EditableFields) != 2 || value.EditableFields[0].Key != "body" || value.EditableFields[0].Rows != 8 || value.EditableFields[0].Max != 4000 {
		t.Fatalf("lost editing contract: %+v", value.EditableFields)
	}
	b, _ := json.Marshal(value)
	if strings.Contains(string(b), "Plan your coastal weekend") {
		t.Fatal("inspection includes editable values")
	}
	if _, err := inspectAuthorContext(ctx, "base.missing"); err == nil || !strings.Contains(err.Error(), "--list --json") {
		t.Fatalf("unhelpful missing target: %v", err)
	}
}

func TestAuthorInspectionMissingTemplateDiagnostic(t *testing.T) {
	root := scaffoldFixtureModule(t)
	scaffoldWrite(t, filepath.Join(root, "source", "app.hyperbricks.yaml"), "base:\n  - type: hypermedia\n  - content:\n      - type: template\n      - template: {file: page.html}\n")
	scaffoldWrite(t, filepath.Join(root, "views", "page.html"), "<main>Template source marker.</main>")
	ctx, err := authorProject(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "views", "page.html")); err != nil {
		t.Fatal(err)
	}
	value, err := inspectAuthorContext(ctx, "base")
	if err != nil {
		t.Fatal(err)
	}
	if len(value.Diagnostics) != 1 || value.Diagnostics[0].Code != "template_reference" || value.Diagnostics[0].Severity != "error" {
		t.Fatalf("missing reference concealed: %+v", value)
	}
}

func TestAuthorAndDoctorNativeImageProperties(t *testing.T) {
	root := scaffoldFixtureModule(t)
	source := `painting:
  - type: image
  - src: {path: {base: resources, path: images/original.png}}
  - width: 1400
  - alt: Painting
  - editable:
      src: {type: asset, directory: {base: resources, path: images}}
      alt: text
base:
  - type: hypermedia
  - content:
      - type: template
      - inline: '{{.image}}'
      - values:
          image:
            - inherit: painting
`
	scaffoldWrite(t, filepath.Join(root, "source", "app.hyperbricks.yaml"), source)
	ctx, err := authorProject(root, "")
	if err != nil {
		t.Fatal(err)
	}
	before := scaffoldState(t, root)
	for _, target := range []string{"base", "base.content", "base.content.values.image"} {
		inspection, err := inspectAuthorContext(ctx, target)
		if err != nil || len(inspection.EditableFields) != 2 {
			t.Fatalf("%s: %+v %v", target, inspection, err)
		}
	}
	module, err := loadAuthoringModule(root, "")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := newScaffoldPlan(module)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateAuthorSources(plan); err != nil {
		t.Fatal(err)
	}
	graph, err := loadDoctorGraph(module)
	if err != nil {
		t.Fatal(err)
	}
	collector := newDoctorCollector()
	checkDoctorSpaces(collector, graph, module.Directories)
	if collector.checks["spaces.contract"].Status != doctorPass {
		t.Fatal(collector.checks)
	}
	// The same target-aware diagnostic must reach authoring and doctor.
	scaffoldWrite(t, filepath.Join(root, "source", "app.hyperbricks.yaml"), strings.Replace(source, "width: 1400", "width: -1", 1))
	_, err = authorProject(root, "")
	if err == nil || !strings.Contains(err.Error(), "/content/values/image/") {
		t.Fatalf("missing target diagnostic: %v", err)
	}
	graph, err = loadDoctorGraph(module)
	if err != nil {
		t.Fatal(err)
	}
	checkDoctorSpaces(collector, graph, module.Directories)
	if check := collector.checks["spaces.contract"]; check.Status != doctorFail || !strings.Contains(check.Message, "/content/values/image/") {
		t.Fatal(check)
	}
	scaffoldWrite(t, filepath.Join(root, "source", "app.hyperbricks.yaml"), source)
	if !reflect.DeepEqual(before, scaffoldState(t, root)) {
		t.Fatal("inspection or validation wrote files")
	}
}
