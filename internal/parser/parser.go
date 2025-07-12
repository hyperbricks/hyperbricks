package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// isKnownType checks if a string is a known type (case-sensive).
func isKnownType(s string) bool {
	return KnownTypes[strings.ToUpper(s)]
}

var (
	logger   *zap.SugaredLogger
	HbConfig map[string]interface{}
)
var KnownTypes = map[string]bool{
	// this is populated by registerComponent...
}

// GetLogger returns the singleton SugaredLogger instance
func GetLogger() *zap.SugaredLogger {
	return logger
}

func init() {
	// Create a custom configuration for the logger
	config := zap.NewProductionConfig()

	// Set the logging level to ERROR
	config.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)

	// Build the logger
	l, err := config.Build()
	if err != nil {
		panic(err)
	}
	defer l.Sync() // Ensure the logger is flushed on exit

	// Use the configured logger
	logger = l.Sugar()
}

// ParseHyperScript parses HyperBricks input and returns a nested configuration.
// It now supports variable definitions and substitutions.
func ParseHyperScript(input string) map[string]interface{} {
	// Initialize variables map
	variables := make(map[string]string)

	// Strip comments from the cdata and input
	cleanedCDATA := StripCDATAAndStore(input)
	cleanedInput := StripComments(cleanedCDATA)

	lines := strings.Split(cleanedInput, "\n")
	index := 0
	config := map[string]interface{}{}
	parseLines(lines, &index, config, config, variables) // Pass variables map
	return config
}

// parseLines recursively parses lines into the config object.
// Now includes variables map for substitution.
func parseLines(lines []string, index *int, config map[string]interface{}, rootConfig map[string]interface{}, variables map[string]string) {
	// Regex to identify variable placeholders like {{VAR:varname}}
	envPattern := regexp.MustCompile(`{{ENV:([a-zA-Z0-9_]+)}}`)
	varPattern := regexp.MustCompile(`{{VAR:([a-zA-Z0-9_]+)}}`)
	confPattern := regexp.MustCompile(`{{CONF:([a-zA-Z0-9_.]+)}}`)

	for *index < len(lines) {
		line := strings.TrimSpace(lines[*index])
		*index++

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle variable definitions (lines starting with $)
		if strings.HasPrefix(line, "$") {
			// Example: $myname = John
			varParts := strings.SplitN(line, "=", 2)
			if len(varParts) != 2 {
				logger.Warnf("Invalid variable definition: %s", line)
				continue
			}
			varName := strings.TrimSpace(strings.TrimPrefix(varParts[0], "$"))
			varValue := strings.TrimSpace(varParts[1])
			variables[varName] = varValue
			logger.Debugf("Defined variable '%s' with value '%s'", varName, varValue)
			continue
		}

		if line == "}" {
			return // End of current block
		}

		// Modified Block Start Detection
		if strings.HasSuffix(line, "{") {
			var key string

			// Check if the line contains '=' before '{'
			equalsIndex := strings.LastIndex(line, "=")
			if equalsIndex != -1 && equalsIndex < len(line)-1 {
				// Extract the key before '='
				key = strings.TrimSpace(line[:equalsIndex])
			} else {
				// Extract the key by removing the trailing '{'
				key = strings.TrimSpace(strings.TrimSuffix(line, "{"))
			}

			// Split the key into parts for nested maps
			keyParts := strings.Split(key, ".")
			existingValue, err := getNestedValue(config, keyParts)
			var newConfig map[string]interface{}
			if err == nil {
				if existingConfig, ok := existingValue.(map[string]interface{}); ok {
					newConfig = existingConfig
				} else {
					newConfig = map[string]interface{}{}
					setNestedValue(config, keyParts, newConfig)
				}
			} else {
				newConfig = map[string]interface{}{}
				setNestedValue(config, keyParts, newConfig)
			}

			parseLines(lines, index, newConfig, rootConfig, variables) // Recursively parse
			continue
		}

		// Handle assignment by reference syntax: "key < reference_key"
		if strings.Contains(line, "<") && !strings.Contains(line, "=") {
			parts := strings.Split(line, "<")
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				referenceKey := strings.TrimSpace(parts[1])
				keyParts := strings.Split(key, ".")
				err := setReferenceValue(config, keyParts, referenceKey, rootConfig)
				if err != nil {

					logging.GetLogger().Errorf("Error setting reference inhiterence: %v", err)
				}
				continue
			}
		}

		// Handle key-value assignments
		indexEq := strings.Index(line, "=")
		if indexEq == -1 {
			continue // Invalid line
		}
		key := strings.TrimSpace(line[:indexEq])
		value := strings.TrimSpace(line[indexEq+1:])

		// Handle multiline text with delimiters
		if strings.HasPrefix(value, "<<[") {
			valueWithoutPrefix := strings.TrimPrefix(value, "<<[")
			if strings.HasSuffix(valueWithoutPrefix, "]>>") {
				// Opening and closing delimiters are on the same line
				value = strings.TrimSuffix(valueWithoutPrefix, "]>>")
			} else {
				// Collect multiline value until closing delimiter is found
				var multilineValue strings.Builder
				multilineValue.WriteString(valueWithoutPrefix)
				multilineValue.WriteString("\n")

				for *index < len(lines) {
					line := lines[*index]
					*index++

					// Trim only the right side, so leading indentation (if you care about it) remains
					trimmedLine := strings.TrimRight(line, " \t")

					if strings.HasSuffix(trimmedLine, "]>>") {
						// Remove "]>>" from the *trimmed* line
						contentBeforeDelimiter := strings.TrimSuffix(trimmedLine, "]>>")
						multilineValue.WriteString(contentBeforeDelimiter)
						break
					}

					// Otherwise, append the line as is (plus a newline)
					multilineValue.WriteString(line)
					multilineValue.WriteString("\n")
				}
				value = multilineValue.String()
				value = stripCommonLeadingSpaces(value)
			}
		}

		// Check if the value is an array (single-line or multi-line)
		if strings.HasPrefix(value, "[") {
			var arrayElements []interface{}

			// Remove the starting '['
			value = strings.TrimPrefix(value, "[")

			// Check if the line also contains ']'
			if strings.Contains(value, "]") {
				// Single-line array
				value = strings.TrimSuffix(value, "]")
				arrayElements = parseArray(value)
			} else {
				// Multi-line array
				arrayElements = []interface{}{}
				for *index < len(lines) {
					line := strings.TrimSpace(lines[*index])
					*index++

					// Check for closing ']'
					if strings.HasSuffix(line, "]") {
						element := strings.TrimSuffix(line, "]")
						element = strings.TrimSpace(element)
						if element != "" {
							// Remove trailing comma if present
							element = strings.TrimSuffix(element, ",")
							element = strings.TrimSpace(element)
							arrayElements = append(arrayElements, element)
						}
						break
					}

					// Remove trailing comma if present
					line = strings.TrimSuffix(line, ",")
					line = strings.TrimSpace(line)
					if line != "" {
						arrayElements = append(arrayElements, line)
					}
				}
			}

			// Perform variable substitution in each array element
			for i, elem := range arrayElements {
				if elemStr, ok := elem.(string); ok {
					arrayElements[i] = varPattern.ReplaceAllStringFunc(elemStr, func(match string) string {
						// Extract variable name
						submatches := varPattern.FindStringSubmatch(match)
						if len(submatches) != 2 {
							logger.Warnf("Invalid variable placeholder: %s", match)
							return match // Return as-is
						}
						varName := submatches[1]
						if varValue, exists := variables[varName]; exists {
							return varValue
						}
						logger.Warnf("Undefined variable '%s'", varName)
						return match // Return as-is
					})
					arrayElements[i] = envPattern.ReplaceAllStringFunc(elemStr, func(match string) string {
						// Extract variable name
						submatches := envPattern.FindStringSubmatch(match)
						if len(submatches) != 2 {
							logger.Warnf("Invalid variable placeholder: %s", match)
							return match // Return as-is
						}
						varName := submatches[1]
						// lookup....
						value, exists := os.LookupEnv(varName)
						if exists {
							return value
						}

						logger.Warnf("Undefined variable '%s'", varName)
						return match // Return as-is
					})
				}
			}

			// // parsing config variables
			// for i, elem := range arrayElements {
			// 	if elemStr, ok := elem.(string); ok {
			// 		arrayElements[i] = confPattern.ReplaceAllStringFunc(elemStr, func(match string) string {
			// 			// Extract variable name
			// 			submatches := confPattern.FindStringSubmatch(match)
			// 			if len(submatches) != 2 {
			// 				logger.Warnf("Invalid variable placeholder: %s", match)
			// 				return match // Return as-is
			// 			}
			// 			varName := submatches[1]

			// 			// lookup....
			// 			value, err := LookupByPath(HbConfig, varName)
			// 			if err != nil {
			// 				fmt.Printf("Error: %s\n", err)
			// 			} else {
			// 				return value.(string)
			// 			}

			// 			logger.Warnf("Undefined variable '%s'", varName)
			// 			return match // Return as-is
			// 		})
			// 	}
			// }

			keyParts := strings.Split(key, ".")
			setNestedValue(config, keyParts, arrayElements)
			continue
		}
		value = envPattern.ReplaceAllStringFunc(value, func(match string) string {
			// Extract variable name
			submatches := envPattern.FindStringSubmatch(match)
			if len(submatches) != 2 {
				logger.Warnf("Invalid variable placeholder: %s", match)
				return match // Return as-is
			}
			varName := submatches[1]
			// lookup....
			value, exists := os.LookupEnv(varName)
			if exists {
				return value
			}
			logger.Warnf("Undefined variable '%s'", varName)
			return match // Return as-is
		})

		// Perform variable substitution in the value
		value = varPattern.ReplaceAllStringFunc(value, func(match string) string {
			// Extract variable name
			submatches := varPattern.FindStringSubmatch(match)
			if len(submatches) != 2 {
				logger.Warnf("Invalid variable placeholder: %s", match)
				return match // Return as-is
			}
			varName := submatches[1]
			if varValue, exists := variables[varName]; exists {
				return varValue
			}
			logger.Warnf("Undefined variable '%s'", varName)
			return match // Return as-is
		})

		// Perform config substitution
		value = confPattern.ReplaceAllStringFunc(value, func(match string) string {
			// Extract config name
			submatches := confPattern.FindStringSubmatch(match)
			if len(submatches) != 2 {
				logger.Warnf("Invalid config placeholder: %s", match)
				return match // Return as-is
			}
			varName := submatches[1]
			//value, err := LookupByPath(HbConfig, varName)
			keyParts := strings.Split(varName, ".")
			cvalue, err := getNestedValue(HbConfig, keyParts)

			if err != nil {
				fmt.Printf("Error: %s\n", err)
			} else {
				strValue, ok := cvalue.(string)
				if !ok {
					return "err"
				}
				return strValue
			}
			logger.Warnf("Undefined config '%s'", varName)
			return match // Return as-is
		})

		// Check if the value is an array (single-line)
		if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
			parsedArray := parseArray(value)
			keyParts := strings.Split(key, ".")
			setNestedValue(config, keyParts, parsedArray)
		} else if isKnownType(value) {
			// Handle known types by setting "@type"
			keyParts := strings.Split(key, ".")
			setNestedValue(config, keyParts, map[string]interface{}{"@type": strings.ToUpper(value)})
		} else {
			// Regular key-value assignment
			keyParts := strings.Split(key, ".")
			setNestedValue(config, keyParts, value)
		}
	}
}

// LookupByPath retrieves a value from a nested map using a dot-separated key path.
func LookupByPath(data map[string]interface{}, path string) (interface{}, error) {
	keys := strings.Split(path, ".")
	current := data
	fmt.Printf("%v", data)
	for i, key := range keys {
		value, exists := current[key]
		if !exists {
			return nil, fmt.Errorf("key not found: %s", strings.Join(keys[:i+1], "."))
		}

		// If we're at the last key, return the value
		if i == len(keys)-1 {
			return value, nil
		}

		// Check if the next level is also a map[string]interface{}
		next, ok := value.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("key path %s does not lead to a map", strings.Join(keys[:i+1], "."))
		}

		current = next
	}

	return nil, errors.New("unexpected error during lookup")
}

// getNestedValue retrieves the value at a nested key path.
func getNestedValue(config map[string]interface{}, keys []string) (interface{}, error) {
	current := config
	for i, key := range keys {
		if val, exists := current[key]; exists {
			if i == len(keys)-1 {
				return val, nil
			} else {
				if nextConfig, ok := val.(map[string]interface{}); ok {
					current = nextConfig
				} else {
					return nil, fmt.Errorf("expected map at '%s', found %T", strings.Join(keys[:i+1], "."), val)
				}
			}
		} else {
			return nil, fmt.Errorf("key not found: %v", strings.Join(keys[:i+1], "."))
		}
	}
	return nil, fmt.Errorf("key not found: %v", strings.Join(keys, "."))
}

// setReferenceValue sets the target key to a deep copy of the reference key's value.
func setReferenceValue(config map[string]interface{}, keyParts []string, referenceKey string, rootConfig map[string]interface{}) error {
	refValue, err := getNestedValue(rootConfig, strings.Split(referenceKey, "."))
	if err != nil {
		return err
	}
	// Perform a deep copy of the reference value
	copiedValue := deepCopy(refValue)
	setNestedValue(config, keyParts, copiedValue)
	return nil
}

// deepCopy creates a deep copy of the provided value.
func deepCopy(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		copy := map[string]interface{}{}
		for k, val := range v {
			copy[k] = deepCopy(val)
		}
		return copy
	case []interface{}:
		copy := make([]interface{}, len(v))
		for i, item := range v {
			copy[i] = deepCopy(item)
		}
		return copy
	default:
		return v
	}
}

// setNestedValue sets a value in the nested map structure, preserving existing keys.
func setNestedValue(config map[string]interface{}, keys []string, value interface{}) {
	if len(keys) == 0 {
		return
	}

	current := config
	for i, key := range keys {
		if i == len(keys)-1 {
			if existing, ok := current[key].(map[string]interface{}); ok {
				if newMap, ok := value.(map[string]interface{}); ok {
					logger := GetLogger()
					logger.Debugf("Merging new values into existing map for key '%s'", key)

					for k, v := range newMap {
						logger.Debugf("Setting key '%s.%s' to value '%v' in existing map", key, k, v)
						existing[k] = v
					}
					return
				}
			}
			logger := GetLogger()
			logger.Debugf("Setting key '%s' to value '%v' in existing map", key, value)

			current[key] = value
		} else {
			if _, exists := current[key]; !exists {
				current[key] = map[string]interface{}{}
			} else if _, ok := current[key].(map[string]interface{}); !ok {
				current[key] = map[string]interface{}{}
			}
			current = current[key].(map[string]interface{})
		}
	}
}

// stripCommonLeadingSpaces removes the common leading spaces from each line in a multiline string.
func stripCommonLeadingSpaces(s string) string {
	lines := strings.Split(s, "\n")
	minIndent := -1

	// Determine the minimum indentation
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		if len(trimmed) == 0 {
			continue // Skip empty lines
		}
		indent := len(line) - len(trimmed)
		if minIndent == -1 || indent < minIndent {
			minIndent = indent
		}
	}

	// Remove the minimum indentation from each line
	if minIndent > 0 {
		for i, line := range lines {
			if len(line) >= minIndent {
				lines[i] = line[minIndent:]
			}
		}
	}
	return strings.Join(lines, "\n")
}

// parseArray parses a string representation of an array.
func parseArray(value string) []interface{} {
	arrayContent := strings.Trim(value, "[]")
	if arrayContent == "" {
		return []interface{}{}
	}
	items := strings.Split(arrayContent, ",")
	parsedArray := make([]interface{}, 0, len(items))
	for _, item := range items {
		trimmedItem := strings.TrimSpace(item)
		parsedArray = append(parsedArray, trimmedItem)
	}
	return parsedArray
}

// PrintJSON converts the configuration to an indented JSON string.
func PrintJSON(config map[string]interface{}) string {
	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		logger.Errorf("Error converting to JSON: %v", err)
		return ""
	}
	return string(jsonData)
}

// PrintConfig recursively builds a structured representation of the configuration.
func PrintConfig(config map[string]interface{}, level int) string {
	var builder strings.Builder
	buildConfig(&builder, config, level)
	return builder.String()
}

// buildConfig is a helper function that recursively appends configuration data to the builder.
func buildConfig(builder *strings.Builder, config map[string]interface{}, level int) {
	keys := make([]string, 0, len(config))
	for k := range config {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		v := config[k]
		builder.WriteString(strings.Repeat("  ", level))
		builder.WriteString(fmt.Sprintf("%s: ", k))
		switch value := v.(type) {
		case map[string]interface{}:
			builder.WriteString("\n")
			buildConfig(builder, value, level+1)
		case []interface{}:
			builder.WriteString("[\n")
			for _, item := range value {
				builder.WriteString(strings.Repeat("  ", level+1))
				builder.WriteString(fmt.Sprintf("- %v\n", item))
			}
			builder.WriteString(strings.Repeat("  ", level) + "]\n")
		default:
			builder.WriteString(fmt.Sprintf("%v\n", value))
		}
	}
}
