// internal/typefactory/hooks.go
package typefactory

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/mitchellh/mapstructure"
)

// StringToIntHook converts string values to integers during decoding.
func StringToIntHook(
	from reflect.Kind,
	to reflect.Kind,
	data interface{},
) (interface{}, error) {
	if from == reflect.String && to == reflect.Int {
		str := data.(string)
		if str == "" {
			return 0, nil // Default value for empty string
		}
		value, err := strconv.Atoi(str)
		if err != nil {
			return 0, fmt.Errorf("cannot convert '%s' to int: %v", str, err)
		}
		return value, nil
	}
	return data, nil
}

// StringToBoolHook converts string values to booleans during decoding.
func StringToBoolHook(
	from reflect.Kind,
	to reflect.Kind,
	data interface{},
) (interface{}, error) {
	if from == reflect.String && to == reflect.Bool {
		str := strings.ToLower(data.(string))

		switch str {
		case "true", "1", "yes":
			return true, nil
		case "false", "0", "no":
			return false, nil
		default:
			// Set to default value (false) for invalid input
			if str == "" {
				return false, nil
			} else {
				return true, nil
			}
		}
	}
	return data, nil
}

// stringToMapStringHookFunc converts a string to a map[string]string by returning an empty map
func StringToMapStringHookFunc() mapstructure.DecodeHookFunc {
	return func(
		from reflect.Type,
		to reflect.Type,
		data interface{},
	) (interface{}, error) {
		if from.Kind() == reflect.String && to.Kind() == reflect.Map {
			// You can customize this behavior.
			// For instance, you could parse a JSON string into a map if applicable.
			// Here, we'll return an empty map to skip setting the field.
			return map[string]string{}, nil
		}
		return data, nil
	}
}

// StringToFloatHook converts string values to floats during decoding.
func StringToFloatHook(
	from reflect.Kind,
	to reflect.Kind,
	data interface{},
) (interface{}, error) {
	if from == reflect.String && to == reflect.Float64 {
		str := data.(string)
		if str == "" {
			return 0.0, nil // Default value for empty string
		}
		value, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return 0.0, fmt.Errorf("cannot convert '%s' to float64: %v", str, err)
		}
		return value, nil
	}
	return data, nil
}

// stringToSliceHookFunc converts a single string to a []string
func StringToSliceHookFunc() mapstructure.DecodeHookFunc {
	return func(
		from reflect.Type,
		to reflect.Type,
		data interface{},
	) (interface{}, error) {
		if from.Kind() == reflect.String && to.Kind() == reflect.Slice && to.Elem().Kind() == reflect.String {
			return []string{data.(string)}, nil
		}
		return data, nil
	}
}

// StringToIntHookFunc converts a string to an int, defaulting to 0 if parsing fails
func StringToIntHookFunc() mapstructure.DecodeHookFunc {
	return func(
		from reflect.Type,
		to reflect.Type,
		data interface{},
	) (interface{}, error) {
		if from.Kind() == reflect.String && to.Kind() == reflect.Int {
			str := data.(string)
			i, err := strconv.Atoi(str)
			if err != nil {
				// Default to 0 if parsing fails
				return 0, nil
			}
			return i, nil
		}
		return data, nil
	}
}

// StringToSliceHook converts comma-separated strings to slices of strings.
func StringToSliceHook(
	from reflect.Kind,
	to reflect.Kind,
	data interface{},
) (interface{}, error) {
	if from == reflect.String && to == reflect.Slice {
		str := data.(string)
		if str == "" {
			return []string{}, nil // Return empty slice for empty string
		}
		parts := strings.Split(str, ",")
		for i, part := range parts {
			parts[i] = strings.TrimSpace(part)
		}
		return parts, nil
	}
	return data, nil
}
