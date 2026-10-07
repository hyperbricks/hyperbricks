package shared

import (
	"strings"
	"testing"
)

func TestLifecycleHooksStrictValidation(t *testing.T) {
	cases := []struct{ source, want string }{
		{"hyperbricks: {hooks: {finish: [{name: done, command: [echo, 123]}]}}", "must be a string"},
		{"hyperbricks: {hooks: {before_start: []}, development: {hooks: {before_start: []}}}", "also declared"},
		{"hyperbricks: {hooks: {finish: [{name: done, command: [echo], env: {HB_OUTCOME: fake}}]}}", "reserved"},
		{"hyperbricks: {hooks: {finish: [{name: duplicate, command: [echo]}], after_static: [{name: duplicate, command: [echo]}]}}", "duplicates"},
	}
	for _, tc := range cases {
		_, err := ValidatePackageConfigBytes([]byte(tc.source), t.TempDir())
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: %v", tc.source, err)
		}
	}
	config, err := ValidatePackageConfigBytes([]byte("hyperbricks: {hooks: {before_start: [{name: prepare, command: [echo]}], finish: [{name: done, command: [echo]}]}}"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(config.HookTasks("before_start")) != 1 || !config.HasDevelopmentProcesses() {
		t.Fatal("new startup hooks not normalized")
	}
}
