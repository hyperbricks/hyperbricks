package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldFastSpecSelectsLibraryVariantsAndUniqueName(t *testing.T) {
	root := scaffoldFixtureModule(t)
	fast, err := scaffoldFastSpec(scaffoldSpec{
		Module:       root,
		Type:         "markdown",
		TemplateMode: "file",
		File:         "components.hyperbricks.yaml",
	})
	if err != nil {
		t.Fatal(err)
	}
	if fast.Starter != "markdown_file" || fast.Type != "markdown" || fast.Name != "markdown_3b7e" {
		t.Fatalf("unexpected fast spec: %+v", fast)
	}

	scaffoldWrite(t, filepath.Join(root, "source", "components.hyperbricks.yaml"), "markdown_3b7e:\n  - type: text\n  - value: occupied\n")
	fast, err = scaffoldFastSpec(scaffoldSpec{
		Module:       root,
		Type:         "markdown",
		TemplateMode: "file",
		File:         "components.hyperbricks.yaml",
	})
	if err != nil {
		t.Fatal(err)
	}
	if fast.Name != "markdown_3b7e_2" {
		t.Fatalf("name did not advance around existing root: %s", fast.Name)
	}
}

func TestScaffoldNonInteractiveWritesLibraryPlan(t *testing.T) {
	root := scaffoldFixtureModule(t)
	fast, err := scaffoldFastSpec(scaffoldSpec{
		Module:       root,
		Type:         "template",
		TemplateMode: "file",
		File:         "components.hyperbricks.yaml",
		Name:         "card_template",
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := prepareScaffoldLibrary(fast)
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.apply(); err != nil {
		t.Fatal(err)
	}
	config := scaffoldRead(t, filepath.Join(root, "source", "components.hyperbricks.yaml"))
	if !strings.Contains(config, "card_template:") || !strings.Contains(config, "template_2sa5.html") {
		t.Fatalf("non-interactive output missing template root or file reference:\n%s", config)
	}
	if _, err := os.Stat(filepath.Join(root, "views", "template_2sa5.html")); err != nil {
		t.Fatal(err)
	}
}

func TestScaffoldStarterFlagValidation(t *testing.T) {
	for _, test := range []struct {
		typeName string
		source   string
		want     string
		bad      bool
	}{
		{typeName: "template", source: "inline", want: "template_inline"},
		{typeName: "template", source: "file", want: "template_file"},
		{typeName: "markdown", source: "file", want: "markdown_file"},
		{typeName: "image", source: "file", bad: true},
	} {
		got, err := scaffoldStarterFlag(test.typeName, test.source)
		if test.bad {
			if err == nil {
				t.Fatalf("scaffoldStarterFlag(%q, %q) accepted invalid source", test.typeName, test.source)
			}
			continue
		}
		if err != nil || got != test.want {
			t.Fatalf("scaffoldStarterFlag(%q, %q) = %q, %v; want %q", test.typeName, test.source, got, err, test.want)
		}
	}
}

func TestScaffoldCommandNonInteractivePreviewAndApply(t *testing.T) {
	root := scaffoldFixtureModule(t)
	args := []string{
		"--module", root,
		"--type", "html",
		"--file", "components.hyperbricks.yaml",
		"--name", "status_message",
		"--dry-run",
	}

	var preview bytes.Buffer
	cmd := NewScaffoldCommand()
	cmd.SetOut(&preview)
	cmd.SetErr(&preview)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(preview.String(), "preview: status_message") {
		t.Fatalf("unexpected scaffold preview: %s", preview.String())
	}
	if _, err := os.Stat(filepath.Join(root, "source", "components.hyperbricks.yaml")); !os.IsNotExist(err) {
		t.Fatalf("preview wrote destination: %v", err)
	}

	var applied bytes.Buffer
	cmd = NewScaffoldCommand()
	cmd.SetOut(&applied)
	cmd.SetErr(&applied)
	cmd.SetArgs(args[:len(args)-1])
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(applied.String(), "created: status_message") {
		t.Fatalf("unexpected scaffold apply output: %s", applied.String())
	}
	content := scaffoldRead(t, filepath.Join(root, "source", "components.hyperbricks.yaml"))
	if !strings.Contains(content, "status_message:") || !strings.Contains(content, "type: html") {
		t.Fatalf("non-interactive scaffold output missing root:\n%s", content)
	}
}

func TestSpaceCommandNonInteractivePreviewAndApply(t *testing.T) {
	root := scaffoldFixtureModule(t)
	args := []string{
		"--module", root,
		"--source", "base",
		"--name", "first_space",
		"--title", "First Space",
		"--route", "first-space",
		"--dry-run",
	}

	var preview bytes.Buffer
	cmd := NewSpaceCommand()
	cmd.SetOut(&preview)
	cmd.SetErr(&preview)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(preview.String(), "preview: first_space") {
		t.Fatalf("unexpected space preview: %s", preview.String())
	}
	spaceFile := filepath.Join(root, "source", "spaces", "base", "first_space.hyperbricks.yaml")
	if _, err := os.Stat(spaceFile); !os.IsNotExist(err) {
		t.Fatalf("space preview wrote destination: %v", err)
	}

	var applied bytes.Buffer
	cmd = NewSpaceCommand()
	cmd.SetOut(&applied)
	cmd.SetErr(&applied)
	cmd.SetArgs(args[:len(args)-1])
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(applied.String(), "created: first_space") {
		t.Fatalf("unexpected space apply output: %s", applied.String())
	}
	content := scaffoldRead(t, spaceFile)
	if !strings.Contains(content, "inherit: base") || !strings.Contains(content, "route: first-space") || !strings.Contains(content, "title: First Space") {
		t.Fatalf("space output missing inheritance or metadata:\n%s", content)
	}
}
