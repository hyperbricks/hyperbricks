package language

import (
	"strings"
	"testing"
)

func TestFormatDocumentPreservesOrderCommentsAndBlockScalars(t *testing.T) {
	source := "# page comment\npage:\n - type: html # type comment\n - value: |\n    <p>Hello</p>\n - trimspace: true\n"
	formatted, err := FormatDocument(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"# page comment", "# type comment", "value: |", "<p>Hello</p>"} {
		if !strings.Contains(formatted, fragment) {
			t.Fatalf("formatted document lost %q:\n%s", fragment, formatted)
		}
	}
	if strings.Index(formatted, "type:") > strings.Index(formatted, "value:") || strings.Index(formatted, "value:") > strings.Index(formatted, "trimspace:") {
		t.Fatalf("ordered entries changed:\n%s", formatted)
	}
	again, err := FormatDocument(formatted)
	if err != nil {
		t.Fatal(err)
	}
	if again != formatted {
		t.Fatalf("formatter is not idempotent:\nfirst:\n%s\nsecond:\n%s", formatted, again)
	}
}

func TestFormatDocumentPreservesCRLF(t *testing.T) {
	formatted, err := FormatDocument("page:\r\n - type: html\r\n - value: hello\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ReplaceAll(formatted, "\r\n", ""), "\n") || !strings.HasSuffix(formatted, "\r\n") {
		t.Fatalf("line endings changed: %q", formatted)
	}
}

func TestFormatDocumentPreservesQuotedAndFlowStyles(t *testing.T) {
	source := "page:\n - type: 'html'\n - value: \"hello\"\n - attributes: {class: 'hero', title: \"Hi\"}\nvalues: [one, 'two', \"three\"]\n"
	formatted, err := FormatDocument(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"type: 'html'",
		`value: "hello"`,
		`{class: 'hero', title: "Hi"}`,
		`[one, 'two', "three"]`,
	} {
		if !strings.Contains(formatted, fragment) {
			t.Fatalf("formatted document lost presentation style %q:\n%s", fragment, formatted)
		}
	}
	again, err := FormatDocument(formatted)
	if err != nil {
		t.Fatal(err)
	}
	if again != formatted {
		t.Fatalf("styled formatter output is not idempotent:\nfirst:\n%s\nsecond:\n%s", formatted, again)
	}
}

func TestFormatDocumentRefusesAliases(t *testing.T) {
	if _, err := FormatDocument("value: &shared hello\ncopy: *shared\n"); err == nil || !strings.Contains(err.Error(), "aliases") {
		t.Fatalf("alias format error = %v", err)
	}
}

func TestFormatDocumentLeavesWhitespaceOnlySourceUnchanged(t *testing.T) {
	source := " \t \r\n\r\n"
	formatted, err := FormatDocument(source)
	if err != nil {
		t.Fatal(err)
	}
	if formatted != source {
		t.Fatalf("whitespace-only source changed from %q to %q", source, formatted)
	}
}
