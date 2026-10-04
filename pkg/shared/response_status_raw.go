package shared

import (
	"fmt"
	"reflect"
	"strings"
)

// ValidateRawResponseStatus preserves strict source types before the component
// factory's ordinary weak decoding can coerce policy fields.
func ValidateRawResponseStatus(raw interface{}) error {
	_, err := DecodeResponseStatus(raw)
	return err
}

// DecodeResponseStatus strictly decodes an explicit policy before weak component
// decoding, including callers that must handle a rejected component instance.
func DecodeResponseStatus(raw interface{}) (*ResponseStatusConfig, error) {
	fields, ok := raw.(map[string]interface{})
	if !ok || fields == nil {
		return nil, fmt.Errorf("response_status must be a mapping")
	}
	config := ResponseStatusConfig{}
	for name, value := range fields {
		switch name {
		case "enabled", "required":
			flag, ok := value.(bool)
			if !ok {
				return nil, fmt.Errorf("response_status.%s must be a boolean", name)
			}
			if name == "enabled" {
				config.Enabled = &flag
			} else {
				config.Required = flag
			}
		case "priority":
			priority, ok := responseStatusRawPriority(value)
			if !ok {
				return nil, fmt.Errorf("response_status.priority must be a nonnegative integer")
			}
			config.Priority = priority
		case "map":
			mapping, ok := value.(map[string]interface{})
			if !ok || mapping == nil {
				return nil, fmt.Errorf("response_status.map must be a mapping")
			}
			for source, target := range mapping {
				if literal, ok := target.(string); ok && literal == "ignore" {
					continue
				}
				if _, ok := responseStatusRawPriority(target); !ok {
					return nil, fmt.Errorf("response_status.map.%s must be an integer status or ignore", source)
				}
			}
			config.Map = mapping
		default:
			return nil, fmt.Errorf("unknown response_status field %q", name)
		}
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &config, nil
}

// ValidateResponseStatusFieldRaw also rejects case-confusable field names,
// which the weak decoder would otherwise match to the policy field.
func ValidateResponseStatusFieldRaw(raw map[string]interface{}) error {
	for name, value := range raw {
		if !strings.EqualFold(name, "response_status") {
			continue
		}
		if name != "response_status" {
			return fmt.Errorf("response status field must be named response_status")
		}
		if err := ValidateRawResponseStatus(value); err != nil {
			return err
		}
	}
	return nil
}

func responseStatusRawPriority(value interface{}) (int, bool) {
	if value == nil {
		return 0, false
	}
	number := reflect.ValueOf(value)
	var priority uint64
	switch number.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if number.Int() < 0 {
			return 0, false
		}
		priority = uint64(number.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		priority = number.Uint()
	default:
		return 0, false
	}
	if priority > uint64(^uint(0)>>1) {
		return 0, false
	}
	return int(priority), true
}
