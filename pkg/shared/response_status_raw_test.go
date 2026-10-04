package shared

import "testing"

func TestValidateRawResponseStatusStrictTypes(t *testing.T) {
	for name, raw := range map[string]interface{}{
		"null block":         nil,
		"scalar block":       "false",
		"list block":         []interface{}{},
		"unknown field":      map[string]interface{}{"requred": true},
		"quoted enabled":     map[string]interface{}{"enabled": "false"},
		"quoted required":    map[string]interface{}{"required": "true"},
		"null enabled":       map[string]interface{}{"enabled": nil},
		"quoted priority":    map[string]interface{}{"priority": "100"},
		"float priority":     map[string]interface{}{"priority": 100.0},
		"negative priority":  map[string]interface{}{"priority": -1},
		"overflow priority":  map[string]interface{}{"priority": ^uint64(0)},
		"null map":           map[string]interface{}{"map": nil},
		"list map":           map[string]interface{}{"map": []interface{}{404}},
		"bad source":         map[string]interface{}{"map": map[string]interface{}{"200": 404}},
		"bad target":         map[string]interface{}{"map": map[string]interface{}{"404": 401}},
		"string target":      map[string]interface{}{"map": map[string]interface{}{"404": "404"}},
		"float target":       map[string]interface{}{"map": map[string]interface{}{"404": 404.0}},
		"disabled malformed": map[string]interface{}{"enabled": false, "required": "true"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateRawResponseStatus(raw); err == nil {
				t.Fatalf("accepted malformed response_status: %#v", raw)
			}
		})
	}
	for name, raw := range map[string]interface{}{
		"required only": map[string]interface{}{"required": true},
		"disabled":      map[string]interface{}{"enabled": false},
		"mapping":       map[string]interface{}{"priority": 100, "required": true, "map": map[string]interface{}{"404": 404, "409": "ignore", "503": 503}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateRawResponseStatus(raw); err != nil {
				t.Fatalf("rejected valid response_status: %v", err)
			}
		})
	}
}

func TestValidateResponseStatusFieldRaw(t *testing.T) {
	if err := ValidateResponseStatusFieldRaw(map[string]interface{}{"other": true}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateResponseStatusFieldRaw(map[string]interface{}{"Response_Status": map[string]interface{}{"enabled": false}}); err == nil {
		t.Fatal("accepted case-confusable response status field")
	}
}
