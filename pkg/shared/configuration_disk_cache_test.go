package shared

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDiskCachePackageDefaultsAndOverrides(t *testing.T) {
	module := t.TempDir()
	config, err := ValidatePackageConfigBytes([]byte("hyperbricks: {mode: development}"), module)
	if err != nil {
		t.Fatal(err)
	}
	if config.Live.DiskCache != DefaultDiskCacheConfig() {
		t.Fatalf("defaults = %+v", config.Live.DiskCache)
	}
	if config.Directories["cache"] != filepath.Join(module, ".cache") {
		t.Fatalf("cache directory = %q", config.Directories["cache"])
	}
	config, err = ValidatePackageConfigBytes([]byte(`hyperbricks:
  mode: live
  live:
    disk_cache: {max_bytes: 1048576, max_entries: 12, cleanup_interval: 5s}
  directories:
    cache: {path: {base: module, path: private-cache}}
`), module)
	if err != nil {
		t.Fatal(err)
	}
	if config.Live.DiskCache != (DiskCacheConfig{MaxBytes: 1048576, MaxEntries: 12, CleanupInterval: 5 * time.Second}) {
		t.Fatalf("override = %+v", config.Live.DiskCache)
	}
	if config.Directories["cache"] != filepath.Join(module, "private-cache") {
		t.Fatalf("resolved cache path = %q", config.Directories["cache"])
	}
	if config.Live.CacheTime.Duration != 10*time.Minute {
		t.Fatal("disk settings changed legacy TTL default")
	}
}

func TestDiskCachePackageRejectsInvalidSettingsInAllModes(t *testing.T) {
	for _, section := range []string{
		"null", "true", "{max_bytes: 0}", "{max_bytes: -1}", "{max_bytes: 1.5}", "{max_bytes: true}", "{max_bytes: null}",
		"{max_entries: 0}", "{max_entries: -1}", "{max_entries: 1.5}", "{max_entries: null}",
		"{cleanup_interval: 0s}", "{cleanup_interval: -1s}", "{cleanup_interval: 10}", "{cleanup_interval: null}", "{max_byte: 100}",
	} {
		for _, mode := range []string{"live", "development", "debug"} {
			_, err := ValidatePackageConfigBytes([]byte("hyperbricks: {mode: "+mode+", live: {disk_cache: "+section+"}}"), t.TempDir())
			if err == nil || !strings.Contains(err.Error(), "disk_cache") {
				t.Errorf("%s %s: %v", mode, section, err)
			}
		}
	}
	for _, value := range []string{"null", "''", "{}"} {
		_, err := ValidatePackageConfigBytes([]byte("hyperbricks: {directories: {cache: "+value+"}}"), t.TempDir())
		if err == nil || !strings.Contains(err.Error(), "directories.cache") {
			t.Errorf("cache directory %s: %v", value, err)
		}
	}
}

func TestDiskCacheInvalidRuntimeSettingsRetained(t *testing.T) {
	config, err := decodePackageConfig(map[string]interface{}{"hyperbricks": map[string]interface{}{"live": map[string]interface{}{"disk_cache": map[string]interface{}{"max_entries": "0"}}}}, t.TempDir())
	if err == nil {
		t.Fatal("weak runtime decoder accepted invalid new setting")
	}
	if err := config.ValidateRuntimeSettings(); err == nil || !strings.Contains(err.Error(), "disk_cache.max_entries") {
		t.Fatalf("runtime validation lost cache error: %v", err)
	}
}
