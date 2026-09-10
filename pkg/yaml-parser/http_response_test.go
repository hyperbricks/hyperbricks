package yamlparser_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/composite"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
	"github.com/mitchellh/mapstructure"
)

func TestHTTPResponseAndGuardVariantsMaterializeAndDecode(t *testing.T) {
	for _, routeType := range []string{"hypermedia", "fragment", "api_fragment_render"} {
		for _, structured := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/structured=%t", routeType, structured), func(t *testing.T) {
				response := `
      status: 202
      headers:
        X-Result: accepted
        HX-Retarget: "#status"`
				guard := `
      enabled: true
      require:
        authenticated: true
      on_unauthenticated:
        default:
          status: 303
          headers:
            Location: /login
        variants:
          - when:
              request_headers:
                X-Client: fragment
                Accept: text/html
            response:
              status: 401
              headers:
                X-Navigate: /login
          - when:
              request_headers:
                X-Client: dialog
            response:
              status: 403
              headers:
                X-Dialog: login`
				if structured {
					response = `
      - status: 202
      - headers:
          X-Result: accepted
          HX-Retarget: "#status"`
					guard = `
      - enabled: true
      - require:
          authenticated: true
      - on_unauthenticated:
          - default:
              status: 303
              headers:
                Location: /login
          - variants:
              - when:
                  request_headers:
                    X-Client: fragment
                    Accept: text/html
                response:
                  status: 401
                  headers:
                    X-Navigate: /login
              - when:
                  request_headers:
                    X-Client: dialog
                response:
                  status: 403
                  headers:
                    X-Dialog: login`
				}
				source := fmt.Sprintf("page:\n  - type: %s\n  - route: test\n  - response:%s\n  - guard:%s\n", routeType, response, guard)
				if routeType == "api_fragment_render" {
					source += "  - endpoint: https://api.example.test/data\n  - headers:\n      Authorization: Bearer upstream-token\n"
				}
				result, err := yamlparser.ProcessBytes([]byte(source), yamlparser.Options{})
				if err != nil {
					t.Fatalf("ProcessBytes() error = %v", err)
				}
				page := result.Materialized["page"].(map[string]interface{})
				guardMap := page["guard"].(map[string]interface{})
				actionMap := guardMap["on_unauthenticated"].(map[string]interface{})
				variants, ok := actionMap["variants"].([]interface{})
				if !ok || len(variants) != 2 {
					t.Fatalf("variants = %#v, want two ordered data records", actionMap["variants"])
				}
				for _, value := range variants {
					variant := value.(map[string]interface{})
					if _, ok := variant["@type"]; ok {
						t.Fatalf("variant became a component: %#v", variant)
					}
				}

				var gotResponse composite.HTTPResponseConfig
				var gotGuard *composite.RouteGuardConfig
				switch routeType {
				case "hypermedia":
					var config composite.HyperMediaConfig
					if err := mapstructure.WeakDecode(page, &config); err != nil {
						t.Fatalf("decode hypermedia: %v", err)
					}
					gotResponse, gotGuard = config.Response, config.Guard
				case "fragment":
					var config composite.FragmentConfig
					if err := mapstructure.WeakDecode(page, &config); err != nil {
						t.Fatalf("decode fragment: %v", err)
					}
					gotResponse, gotGuard = config.Response, config.Guard
				case "api_fragment_render":
					var config composite.ApiFragmentRenderConfig
					if err := mapstructure.WeakDecode(page, &config); err != nil {
						t.Fatalf("decode API fragment: %v", err)
					}
					gotResponse, gotGuard = config.Response, config.Guard
					if config.Headers["Authorization"] != "Bearer upstream-token" {
						t.Fatalf("upstream headers = %#v", config.Headers)
					}
				}
				wantResponse := composite.HTTPResponseConfig{
					Status:  202,
					Headers: map[string]string{"X-Result": "accepted", "HX-Retarget": "#status"},
				}
				if !reflect.DeepEqual(gotResponse, wantResponse) {
					t.Fatalf("response = %#v, want %#v", gotResponse, wantResponse)
				}
				if gotGuard == nil || !gotGuard.Enabled || !gotGuard.Require.Authenticated {
					t.Fatalf("guard = %#v, want enabled authenticated guard", gotGuard)
				}
				action := gotGuard.OnUnauthenticated
				if action.Default.Status != 303 || action.Default.Headers["Location"] != "/login" {
					t.Fatalf("default response = %#v", action.Default)
				}
				if len(action.Variants) != 2 {
					t.Fatalf("decoded variants = %#v", action.Variants)
				}
				first, second := action.Variants[0], action.Variants[1]
				if !reflect.DeepEqual(first.When.RequestHeaders, map[string]string{"X-Client": "fragment", "Accept": "text/html"}) ||
					first.Response.Status != 401 || first.Response.Headers["X-Navigate"] != "/login" {
					t.Fatalf("first variant = %#v", first)
				}
				if second.When.RequestHeaders["X-Client"] != "dialog" || second.Response.Status != 403 || second.Response.Headers["X-Dialog"] != "login" {
					t.Fatalf("second variant = %#v", second)
				}
			})
		}
	}
}
