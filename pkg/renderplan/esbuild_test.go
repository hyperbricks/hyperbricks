package renderplan_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/renderplan"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestEsbuildPlanIsLazyAndMatchesLegacy(t *testing.T) {
	root := t.TempDir()
	entry, output := filepath.Join(root, "main.js"), filepath.Join(root, "public", "bundle.js")
	if err := os.WriteFile(entry, []byte(`console.log("before");`), 0644); err != nil {
		t.Fatal(err)
	}
	manager := newTestRenderManager()
	renderer := component.NewEsbuildRenderer(filepath.Join(root, "public"))
	manager.RegisterComponent(component.EsbuildConfigGetName(), renderer, reflect.TypeOf(component.EsbuildConfig{}))
	config := component.EsbuildConfig{Entry: entry, Outfile: output, Cache: false, Component: shared.Component{Enclose: `<script src="|"></script>`}}
	child := map[string]interface{}{"@type": component.EsbuildConfigGetName(), "entry": entry, "outfile": output, "cache": false, "enclose": config.Enclose, component.EsbuildPreparedKey: renderer.Prepare(config)}
	raw := headlessTemplateRoute(map[string]interface{}{"content": child})
	plan, err := renderplan.Compile(manager, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("plan compilation built asset eagerly")
	}
	for i, message := range []string{"before", "after"} {
		if err := os.WriteFile(entry, []byte(`console.log("`+message+`");`), 0644); err != nil {
			t.Fatal(err)
		}
		compiled, errs := plan.Render(context.Background())
		legacy, legacyErrs := manager.Render(raw["@type"].(string), raw, context.Background())
		if len(errs) != 0 || len(legacyErrs) != 0 || compiled != legacy || compiled != `<script src="/static/bundle.js"></script>` {
			t.Fatalf("round %d: %s %s %v %v", i, compiled, legacy, errs, legacyErrs)
		}
		data, err := os.ReadFile(output)
		if err != nil || !strings.Contains(string(data), message) {
			t.Fatal("plan froze an uncached build")
		}
	}
}
