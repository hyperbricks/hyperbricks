package logging

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestModuleLocations(t *testing.T) {
	t.Chdir(t.TempDir())
	root, err := filepath.Abs("modules/demo")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ file, want string }{
		{filepath.Join(root, "profiles/development.hyperbricks.yaml"), "profiles/development.hyperbricks.yaml"},
		{"modules/demo/templates/hero.html", "templates/hero.html"},
		{"./modules/demo/templates/hero.html", "templates/hero.html"},
		{root, "."},
		{"logs/runtime.jsonl", "../../logs/runtime.jsonl"},
		{"", ""},
	} {
		if got := ModulePath(root, test.file); got != test.want {
			t.Fatalf("ModulePath(%q)=%q want %q", test.file, got, test.want)
		}
	}
	for _, prefix := range []string{root, "modules/demo", "./modules/demo"} {
		message := "open " + prefix + "/hyperbricks/page.hyperbricks.yaml:8:3: invalid source"
		if got := ModuleText(root, message); got != "open hyperbricks/page.hyperbricks.yaml:8:3: invalid source" {
			t.Fatalf("message=%q", got)
		}
	}
	for _, message := range []string{"hyperbricks/page.hyperbricks.yaml", "https://api.example.test/data", root + "-other/page.yaml"} {
		if got := ModuleText(root, message); got != message {
			t.Fatalf("unrelated location rewritten: %q", got)
		}
	}
	if got := ModuleText(root, "template: "+root+"/templates/hero.html:12:7"); strings.Contains(got, root) {
		t.Fatalf("absolute module in embedded error: %q", got)
	}
}
