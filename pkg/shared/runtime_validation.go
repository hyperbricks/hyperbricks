package shared

import (
	"fmt"
	"net/url"
	"runtime"
	"strconv"
	"strings"
)

// ValidateRuntimeSettings checks package values that are accepted by typed
// decoding but would make startup fail or panic. It is pure and does not apply
// process-wide settings or contact configured services.
func (config *Config) ValidateRuntimeSettings() error {
	if config == nil {
		return fmt.Errorf("runtime configuration is required")
	}
	if config.diskCacheConfigError != nil {
		return config.diskCacheConfigError
	}
	if config.System.MetricsWatchInterval <= 0 {
		return fmt.Errorf("hyperbricks.system.metrics_watch_interval must be greater than zero")
	}
	if config.Live.CacheTime.Duration < 0 {
		return fmt.Errorf("hyperbricks.live.cache must be zero (disabled) or a positive duration")
	}
	if _, err := ResolveGoMaxProcs(config.Server.GoMaxProcs, runtime.NumCPU()); err != nil {
		return err
	}
	return ValidateRuntimeGatewayConfig(config.Server.RuntimeGateway)
}

// ResolveGoMaxProcs validates a package value without changing GOMAXPROCS. A
// zero result means Go-managed automatic parallelism.
func ResolveGoMaxProcs(value any, logicalCPUs int) (int, error) {
	invalid := func() (int, error) {
		return 0, fmt.Errorf("hyperbricks.server.gomaxprocs must be auto or an integer between 1 and %d; got %v", logicalCPUs, value)
	}
	if value == nil {
		return 0, nil
	}
	var count int
	switch typed := value.(type) {
	case int:
		count = typed
	case string:
		typed = strings.TrimSpace(typed)
		if typed == "auto" {
			return 0, nil
		}
		parsed, err := strconv.Atoi(typed)
		if err != nil {
			return invalid()
		}
		count = parsed
	default:
		return invalid()
	}
	if count < 1 || count > logicalCPUs {
		return invalid()
	}
	return count, nil
}

// ValidateRuntimeGatewayConfig validates gateway syntax without resolving or
// contacting the configured endpoint.
func ValidateRuntimeGatewayConfig(config RuntimeGatewayConfig) error {
	if !config.Enabled {
		return nil
	}
	if !hasRuntimeGatewayDomain(config) && !hasRuntimeGatewayHostSuffix(config) {
		return fmt.Errorf("runtime gateway requires runtime domain or host suffix")
	}
	resolver := strings.TrimSpace(config.Resolver)
	if resolver == "" {
		return fmt.Errorf("runtime gateway requires runtime resolver")
	}
	parsed, err := url.Parse(resolver)
	if err != nil {
		return fmt.Errorf("runtime resolver is invalid: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("runtime resolver must use http or https")
	}
	if parsed.Host == "" {
		return fmt.Errorf("runtime resolver host is empty")
	}
	return nil
}

func hasRuntimeGatewayDomain(config RuntimeGatewayConfig) bool {
	values := append([]string{config.Domain}, config.Domains...)
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			item = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(item), "*."), "."), "."))
			if item != "" {
				return true
			}
		}
	}
	return false
}

func hasRuntimeGatewayHostSuffix(config RuntimeGatewayConfig) bool {
	values := append([]string{config.HostSuffix}, config.HostSuffixes...)
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			item = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.ToLower(item), "*"), "."))
			if item != "" {
				return true
			}
		}
	}
	return false
}
