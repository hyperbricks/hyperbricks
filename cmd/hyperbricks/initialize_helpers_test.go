package main

import (
	"reflect"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestResolveDevelopmentWatchDirectoriesDefaultsToHyperbricksAndTemplates(t *testing.T) {
	config := &shared.Config{
		Directories: map[string]string{
			"hyperbricks": "modules/demo/hyperbricks",
			"templates":   "modules/demo/templates",
			"resources":   "modules/demo/resources",
		},
	}

	got := resolveDevelopmentWatchDirectories(config)
	want := []string{"modules/demo/hyperbricks", "modules/demo/templates"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("watch directories = %#v, want %#v", got, want)
	}
}

func TestResolveDevelopmentWatchDirectoriesUsesConfiguredWatchDirs(t *testing.T) {
	config := &shared.Config{
		Development: shared.DevelopmentConfig{
			WatchDirs: []string{"hyperbricks", "templates", "resources", "templates", " "},
		},
		Directories: map[string]string{
			"hyperbricks": "modules/demo/hyperbricks",
			"templates":   "modules/demo/templates",
			"resources":   "modules/demo/resources",
		},
	}

	got := resolveDevelopmentWatchDirectories(config)
	want := []string{"modules/demo/hyperbricks", "modules/demo/templates", "modules/demo/resources"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("watch directories = %#v, want %#v", got, want)
	}
}

func TestResolveDevelopmentWatchDirectoriesAllowsRelativeCustomPaths(t *testing.T) {
	config := &shared.Config{
		Development: shared.DevelopmentConfig{
			WatchDirs: []string{"custom"},
		},
		Directories: map[string]string{},
	}

	got := resolveDevelopmentWatchDirectories(config)
	want := []string{"custom"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("watch directories = %#v, want %#v", got, want)
	}
}
