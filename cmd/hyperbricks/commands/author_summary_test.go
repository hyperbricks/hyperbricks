package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestAuthorSummaryPreservesOperationsAndRecovery(t *testing.T) {
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
	op := spec
	op.Version, op.Revision = 0, ""
	p, err := prepareAuthorBatch(root, "", authorSpec{Version: 2, Operations: []authorSpec{op}})
	if err != nil {
		t.Fatal(err)
	}
	spec.Brick.Properties["ordinary_data"] = map[string]interface{}{"type": "html", "properties": map[string]interface{}{"value": "Diagnostic marker"}}
	for _, failure := range []error{nil, fmt.Errorf("write failed; completed files retained: [page.yaml]")} {
		var out bytes.Buffer
		cmd := NewAuthorCommand()
		cmd.SetOut(&out)
		if err := reportAuthorApply(cmd, p, failure, false, true, false, spec, true); err != nil {
			t.Fatal(err)
		}
		var result map[string]interface{}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result["input_revision"] != ctx.Revision || result["changes"] == nil || result["operations"] == nil || result["diagnostics"] == nil {
			t.Fatalf("lost summary: %s", out.String())
		}
		for _, key := range []string{"plan", "emissions", "yaml", "before", "after"} {
			if strings.Contains(out.String(), `"`+key+`"`) {
				t.Fatalf("repeated source: %s", out.String())
			}
		}
		if failure != nil && (result["status"] != "error" || result["error"] != failure.Error() || ExitCode != 1) {
			t.Fatal("lost recovery error")
		}
	}
}

func TestAuthorSummaryAppliesThroughCLI(t *testing.T) {
	root := starterAuthorModule(t)
	ctx, err := authorProject(root, "")
	if err != nil {
		t.Fatal(err)
	}
	focus, err := focusAuthorContext(ctx, "scaffold_page")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(focus.Example)
	if err != nil {
		t.Fatal(err)
	}
	cmd := NewAuthorCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetIn(bytes.NewReader(raw))
	cmd.SetArgs([]string{"apply", "-m", root, "--spec", "-", "--summary", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["status"] != "created" || result["operations"] == nil || result["plan"] != nil {
		t.Fatalf("unexpected summary: %s", out.String())
	}
	after, err := authorProject(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision == ctx.Revision {
		t.Fatal("apply did not write")
	}
	if _, err := inspectAuthorContext(after, focus.Example.Brick.Name); err != nil {
		t.Fatal(err)
	}
}

func TestAuthorSummaryCannotSuppressPreview(t *testing.T) {
	for _, other := range []string{"--dry-run", "--compact"} {
		cmd := NewAuthorCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"apply", "--summary", other, "--json"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "summary") {
			t.Fatalf("flags not rejected: err=%v output=%s", err, out.String())
		}
	}
}
