package shared

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePackageConfigBytesWithResourceReaderUsesCallerBoundary(t *testing.T) {
	module := t.TempDir()
	outside := filepath.Join(t.TempDir(), "mode.txt")
	if err := os.WriteFile(outside, []byte("live"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := []byte("hyperbricks:\n  mode: {file: '" + filepath.ToSlash(outside) + "'}\n")
	// The existing helper retains its established runtime reader.
	if config, err := ValidatePackageConfigBytes(source, module); err != nil || config.Mode != LIVE_MODE {
		t.Fatalf("legacy configuration reader changed: config=%#v error=%v", config, err)
	}
	reads := 0
	_, err := ValidatePackageConfigBytesWithResourceReader(source, module, func(path string) ([]byte, error) {
		reads++
		if path != outside {
			t.Fatalf("reader path = %q, want %q", path, outside)
		}
		return nil, errors.New("outside editor resource boundary")
	})
	if reads != 1 {
		t.Fatalf("confined reader called %d times, want 1", reads)
	}
	// A denied mode resolver becomes empty and fails the same configuration
	// contract. If a direct disk fallback occurred this would validate as live.
	if err == nil {
		t.Fatal("denied resource silently fell back to disk")
	}
}

func TestValidatePackageConfigBytesWithResourceReaderPreservesValidation(t *testing.T) {
	module := t.TempDir()
	resource := filepath.Join(module, "resources", "mode.txt")
	config, err := ValidatePackageConfigBytesWithResourceReader([]byte(`hyperbricks:
  mode: {file: {base: resources, path: mode.txt}}
  server:
    port: 9012
`), module, func(path string) ([]byte, error) {
		if path != resource {
			t.Fatalf("path base changed: got %q, want %q", path, resource)
		}
		return []byte("live"), nil
	})
	if err != nil || config.Mode != LIVE_MODE || config.Server.Port != 9012 {
		t.Fatalf("read-only config validation: config=%#v error=%v", config, err)
	}
	if _, err := ValidatePackageConfigBytesWithResourceReader([]byte("hyperbricks: {mode: unknown}"), module, func(string) ([]byte, error) {
		t.Fatal("no file resolver exists")
		return nil, nil
	}); err == nil {
		t.Fatal("strict runtime mode validation was bypassed")
	}
	if _, err := ValidatePackageConfigBytesWithResourceReader([]byte("hyperbricks: {mode: live}"), module, nil); err == nil {
		t.Fatal("read-only editor API must require an explicit resource reader")
	}
}
