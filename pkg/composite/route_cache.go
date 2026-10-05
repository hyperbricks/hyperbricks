package composite

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/typefactory"
	"github.com/mitchellh/mapstructure"
)

const (
	RouteCacheMemory = "mem"
	RouteCacheDisk   = "disk"
)

// RouteCacheConfig is the authoring form of a route's output cache. Expire is
// presence-aware: omission inherits the package lifetime, while 0s disables it.
// A scalar duration is shorthand for an explicit memory policy.
type RouteCacheConfig struct {
	Storage string  `mapstructure:"storage" validate:"omitempty,oneof=mem disk" description:"Storage for rendered output in live mode: mem (default) or disk. Does not change HTTP cache policy." example:"disk"`
	Expire  *string `mapstructure:"expire" description:"Route lifetime as a Go duration; omitted inherits hyperbricks.live.cache. Use 0s to disable caching for this route." example:"30s"`
	scalar  string
	mapping bool
}

// MarshalJSON retains the existing public configuration representation for
// omitted and scalar cache fields. Only explicit policies use the new object.
func (config RouteCacheConfig) MarshalJSON() ([]byte, error) {
	if config.scalar != "" && config.Storage == RouteCacheMemory && config.Expire != nil && *config.Expire == config.scalar {
		return json.Marshal(config.scalar)
	}
	if !config.mapping && config.Storage == "" && config.Expire == nil {
		return json.Marshal("")
	}
	return json.Marshal(struct {
		Storage string  `json:"storage,omitempty"`
		Expire  *string `json:"expire,omitempty"`
	}{Storage: config.Storage, Expire: config.Expire})
}

type RouteCachePolicy struct {
	Storage string
	Expire  time.Duration
}

// ResolveRouteCache resolves only the route's storage and lifetime. Global
// disabling, modes, nocache, and response eligibility remain runtime decisions.
func ResolveRouteCache(raw map[string]interface{}, defaultTTL time.Duration) (RouteCachePolicy, error) {
	policy := RouteCachePolicy{Storage: RouteCacheMemory, Expire: defaultTTL}
	if typ, ok := raw["@type"].(string); ok && typ != "<HYPERMEDIA>" && typ != "<FRAGMENT>" {
		return policy, nil
	}
	value, exists := raw["cache"]
	if !exists {
		return policy, nil
	}
	parsed, err := parseRouteCacheValue(value)
	if err != nil {
		return policy, err
	}
	if parsed.storage != "" {
		policy.Storage = parsed.storage
	}
	if parsed.hasExpire {
		policy.Expire = parsed.expire
	}
	return policy, nil
}

// The runtime reads a policy without constructing pointer-valued authoring
// fields. One parser owns validation, and each duration is parsed only once.
type parsedRouteCache struct {
	storage, expireText, scalar string
	expire                      time.Duration
	hasExpire, mapping          bool
}

// DecodeConfigValue is shared by the factory, route initialization, and direct
// rendering decoders so the scalar and mapping forms have identical semantics.
func (config *RouteCacheConfig) DecodeConfigValue(value interface{}) error {
	parsed, err := parseRouteCacheValue(value)
	if err != nil {
		return err
	}
	decoded := RouteCacheConfig{Storage: parsed.storage, scalar: parsed.scalar, mapping: parsed.mapping}
	if parsed.hasExpire {
		decoded.Expire = &parsed.expireText
	}
	*config = decoded
	return nil
}

func parseRouteCacheValue(value interface{}) (parsedRouteCache, error) {
	var parsed parsedRouteCache
	var err error
	switch typed := value.(type) {
	case RouteCacheConfig:
		parsed.storage, parsed.scalar, parsed.mapping = typed.Storage, typed.scalar, typed.mapping
		if parsed.storage != "" && parsed.storage != RouteCacheMemory && parsed.storage != RouteCacheDisk {
			return parsed, fmt.Errorf("cache.storage must be mem or disk")
		}
		if typed.Expire != nil {
			parsed.expireText, parsed.expire, err = routeCacheDuration(*typed.Expire, "cache.expire")
			parsed.hasExpire = true
		}
	case string:
		parsed.expireText, parsed.expire, err = routeCacheDuration(typed, "cache")
		parsed.storage, parsed.scalar, parsed.hasExpire = RouteCacheMemory, parsed.expireText, true
	case map[string]interface{}:
		parsed.mapping = true
		for key, value := range typed {
			switch key {
			case "@order":
				// Materialized ordered configuration can carry parser metadata.
				continue
			case "storage":
				storage, ok := value.(string)
				if !ok || (storage != RouteCacheMemory && storage != RouteCacheDisk) {
					return parsed, fmt.Errorf("cache.storage must be mem or disk")
				}
				parsed.storage = storage
			case "expire":
				parsed.expireText, parsed.expire, err = routeCacheDuration(value, "cache.expire")
				if err != nil {
					return parsed, err
				}
				parsed.hasExpire = true
			default:
				return parsed, fmt.Errorf("cache contains unknown field %q; expected storage or expire", key)
			}
		}
	default:
		return parsed, fmt.Errorf("cache must be a duration string or a mapping containing storage and expire")
	}
	return parsed, err
}

func routeCacheDuration(value interface{}, field string) (string, time.Duration, error) {
	text, ok := value.(string)
	text = strings.TrimSpace(text)
	if !ok || text == "" || text == "0" {
		return "", 0, fmt.Errorf("%s must be a duration string, such as 30s or 0s", field)
	}
	duration, err := time.ParseDuration(text)
	if err != nil || duration < 0 {
		return "", 0, fmt.Errorf("%s must be zero (disabled) or a positive duration; got %q", field, text)
	}
	return text, duration, nil
}

func (config *HyperMediaConfig) ValidateRawConfig(raw map[string]interface{}) error {
	_, err := ResolveRouteCache(raw, 0)
	return err
}

func (config *FragmentConfig) ValidateRawConfig(raw map[string]interface{}) error {
	_, err := ResolveRouteCache(raw, 0)
	return err
}

func decodeRouteRenderConfig(raw interface{}, target interface{}) error {
	if value, ok := raw.(map[string]interface{}); ok {
		if validator, ok := target.(interface {
			ValidateRawConfig(map[string]interface{}) error
		}); ok {
			if err := validator.ValidateRawConfig(value); err != nil {
				return err
			}
		}
	}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result: target, TagName: "mapstructure", DecodeHook: typefactory.ConfigValueDecodeHookFunc(),
	})
	if err != nil {
		return err
	}
	return decoder.Decode(raw)
}
