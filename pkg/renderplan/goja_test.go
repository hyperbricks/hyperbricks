package renderplan_test

import (
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/renderplan"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
)

func TestGojaPlanParityIsolationAndReplacement(t *testing.T) {
	manager := newTestRenderManager()
	manager.RegisterComponent(component.GojaRenderConfigGetName(), &component.GojaRenderer{}, reflect.TypeOf(component.GojaRenderConfig{}))
	makeRoute := func(prefix string) map[string]interface{} {
		return headlessTemplateRoute(map[string]interface{}{"content": map[string]interface{}{
			"@type":  component.GojaRenderConfigGetName(),
			"script": `var count = 0; function main(input) { count++; return {rid: input.query.rid, count: count, prefix: input.values.prefix}; }`,
			"inline": `<p>{{.Data.prefix}}:{{.Data.rid}}:{{.Data.count}}</p>`,
			"values": map[string]interface{}{"prefix": prefix}, "querykeys": []string{"rid"},
		}})
	}
	old := makeRoute("old")
	before := shared.CloneMapDeep(old)
	oldPlan, err := renderplan.Compile(manager, old, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(old, before) {
		t.Fatal("compilation modified raw config")
	}
	newRoute := makeRoute("new")
	newPlan, err := renderplan.Compile(manager, newRoute, nil)
	if err != nil {
		t.Fatal(err)
	}
	child := newRoute["template"].(map[string]interface{})["values"].(map[string]interface{})["content"].(map[string]interface{})
	response, err := manager.MakeInstance(typefactory.TypeRequest{TypeName: component.GojaRenderConfigGetName(), Data: child})
	if err != nil {
		t.Fatal(err)
	}
	child[component.GojaPreparedKey] = component.PrepareGojaRender(response.Instance.(component.GojaRenderConfig), nil)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rid := fmt.Sprintf("rid-%d", i)
			ctx := requestContext(rid)
			got, errs := newPlan.Render(ctx)
			want := "<p>new:" + rid + ":1</p>"
			legacy, legacyErrs := manager.Render(newRoute["@type"].(string), newRoute, ctx)
			if got != want || legacy != want || len(errs) != 0 || len(legacyErrs) != 0 {
				t.Errorf("new plan=%q legacy=%q expected=%q errors=%v/%v", got, legacy, want, errs, legacyErrs)
			}
			got, errs = oldPlan.Render(ctx)
			if got != "<p>old:"+rid+":1</p>" || len(errs) != 0 {
				t.Errorf("old snapshot changed: %q %v", got, errs)
			}
		}(i)
	}
	wg.Wait()
}

func TestGojaInvalidScriptIsConfigurationError(t *testing.T) {
	manager := newTestRenderManager()
	manager.RegisterComponent(component.GojaRenderConfigGetName(), &component.GojaRenderer{}, reflect.TypeOf(component.GojaRenderConfig{}))
	raw := headlessTemplateRoute(map[string]interface{}{"content": map[string]interface{}{
		"@type": component.GojaRenderConfigGetName(), "script": "function main(", "inline": "ok",
	}})
	if _, err := renderplan.Compile(manager, raw, nil); err == nil {
		t.Fatal("invalid script compiled successfully")
	}
}
