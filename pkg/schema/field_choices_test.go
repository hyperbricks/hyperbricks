package schema

import (
	"reflect"
	"testing"
)

func TestFieldAllowedValues(t *testing.T) {
	for _, tc := range []struct {
		rules string
		want  []string
	}{
		{"omitempty,oneof=mem disk", []string{"mem", "disk"}},
		{"required,oneof=one two,min=1", []string{"one", "two"}},
		{"required", nil},
		{"oneof=", nil},
		{"oneof='two words' other", nil},
		{"oneof=mem disk|eq=other", nil},
	} {
		if got := (Field{Validate: tc.rules}).AllowedValues(); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q choices = %v; want %v", tc.rules, got, tc.want)
		}
	}
}
