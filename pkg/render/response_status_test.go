package render_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/render"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestRequiredAPIResponseStatusOnFactoryFailure(t *testing.T) {
	manager := render.NewRenderManager()
	manager.RegisterComponent(component.APIConfigGetName(), &component.APIRenderer{}, reflect.TypeOf(component.APIConfig{}))
	manager.RegisterComponent(composite.ApiFragmentRenderConfigGetName(), &composite.ApiFragmentRenderer{}, reflect.TypeOf(composite.ApiFragmentRenderConfig{}))
	for _, kind := range []string{component.APIConfigGetName(), composite.ApiFragmentRenderConfigGetName()} {
		for _, test := range []struct {
			name   string
			policy map[string]interface{}
			want   int
		}{
			{"omitted", nil, 0},
			{"optional", map[string]interface{}{}, 0},
			{"disabled", map[string]interface{}{"enabled": false, "required": true}, 0},
			{"required", map[string]interface{}{"required": true}, 500},
		} {
			t.Run(kind+"/"+test.name, func(t *testing.T) {
				capture := &shared.ResponseStatusCapture{}
				ctx := context.WithValue(context.Background(), shared.ResponseStatusCaptureKey, capture)
				config := map[string]interface{}{"@type": kind, "forwardtoken": false}
				if test.policy != nil {
					config["response_status"] = test.policy
				}
				_, diagnostics := manager.Render(kind, config, ctx)
				if len(diagnostics) == 0 {
					t.Fatal("invalid raw API input must fail before rendering")
				}
				status, _ := capture.Result()
				if status != test.want {
					t.Fatalf("status = %d, want %d", status, test.want)
				}
			})
		}
	}
}
