package shared

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type DiskCacheConfig struct {
	MaxBytes        int64         `mapstructure:"max_bytes" description:"Positive maximum total response-body bytes for this runtime's disk cache. Entries larger than the limit are served fresh without caching." example:"268435456"`
	MaxEntries      int           `mapstructure:"max_entries" description:"Positive maximum number of disk response entries; older entries are evicted when the limit is reached." example:"10000"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval" description:"Positive Go duration between expired response cache cleanup passes. Cleanup timing does not extend entry expiry." example:"1m"`
}

func DefaultDiskCacheConfig() DiskCacheConfig {
	return DiskCacheConfig{MaxBytes: 256 << 20, MaxEntries: 10000, CleanupInterval: time.Minute}
}

// decodeDiskCache strictly owns the new section while leaving the existing
// package's weak-decoding and legacy-duration behavior unchanged.
func decodeDiskCache(input interface{}) (DiskCacheConfig, error) {
	config := DefaultDiskCacheConfig()
	root, _ := input.(map[string]interface{})
	if directories, ok := root["directories"].(map[string]interface{}); ok {
		if value, exists := directories["cache"]; exists {
			if path, ok := value.(string); !ok || strings.TrimSpace(path) == "" {
				return config, fmt.Errorf("hyperbricks.directories.cache must resolve to a nonempty path")
			}
		}
	}
	live, _ := root["live"].(map[string]interface{})
	raw, exists := live["disk_cache"]
	if !exists {
		return config, nil
	}
	settings, ok := raw.(map[string]interface{})
	if !ok {
		return config, fmt.Errorf("hyperbricks.live.disk_cache must be a mapping")
	}
	for key, value := range settings {
		field := "hyperbricks.live.disk_cache." + key
		switch key {
		case "max_bytes", "max_entries":
			limit, err := positiveCacheInteger(value, field)
			if err != nil {
				return config, err
			}
			if key == "max_bytes" {
				config.MaxBytes = limit
			} else {
				if int64(int(limit)) != limit {
					return config, fmt.Errorf("%s exceeds the supported integer range", field)
				}
				config.MaxEntries = int(limit)
			}
		case "cleanup_interval":
			text, ok := value.(string)
			duration, err := time.ParseDuration(strings.TrimSpace(text))
			if !ok || err != nil || duration <= 0 {
				return config, fmt.Errorf("%s must be a positive duration", field)
			}
			config.CleanupInterval = duration
		default:
			return config, fmt.Errorf("hyperbricks.live.disk_cache contains unknown field %q", key)
		}
	}
	return config, nil
}

func positiveCacheInteger(value interface{}, field string) (int64, error) {
	var parsed int64
	switch typed := value.(type) {
	case string:
		var err error
		parsed, err = strconv.ParseInt(typed, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%s must be a positive integer", field)
		}
	case int:
		parsed = int64(typed)
	case int64:
		parsed = typed
	default:
		return 0, fmt.Errorf("%s must be a positive integer", field)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", field)
	}
	return parsed, nil
}
