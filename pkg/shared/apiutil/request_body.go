package apiutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var (
	bodyPlaceholder = regexp.MustCompile(`\$([A-Za-z0-9_]+)\b`)
	bodyJSONString  = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)
)

// MapRequestBody substitutes configured API body values. In a JSON object, a
// missing whole-value placeholder omits that member; it is not a literal token
// or a request to clear the upstream value. Raw non-JSON bodies retain textual
// substitution, since property omission has no meaning for those formats.
func MapRequestBody(body string, input map[string]interface{}) (string, error) {
	if !hasBodyPlaceholder(body) {
		return body, nil
	}
	trimmed := strings.TrimSpace(body)
	if trimmed == "" || (!strings.ContainsAny(trimmed[:1], `{["`) && !wholeBodyPlaceholder(trimmed)) {
		return mapRawBody(body, input), nil
	}

	// Bare JSON value placeholders (e.g. {"count":$count}) are not JSON yet.
	// Validate the structure with null in those slots, retaining their exact
	// decoder end offsets. No sentinel strings can collide with user data.
	normalized, bare := normalizeBodyPlaceholders(body)
	if !json.Valid([]byte(normalized)) {
		return "", fmt.Errorf("invalid JSON API body template")
	}
	decoder := json.NewDecoder(strings.NewReader(normalized))
	decoder.UseNumber()
	mapped, missing, err := mapBodyJSONValue(decoder, bare, input)
	if err != nil {
		return "", err
	}
	if missing {
		return "", fmt.Errorf("missing API body placeholder outside an object property")
	}
	return string(mapped), nil
}

func hasBodyPlaceholder(body string) bool {
	if bodyPlaceholder.MatchString(body) {
		return true
	}
	// JSON escapes must not change whether a placeholder is recognized. In
	// particular, an escaped dollar behaves the same with or without another
	// unescaped placeholder elsewhere in the configured body.
	for _, token := range bodyJSONString.FindAllString(body, -1) {
		var value string
		if json.Unmarshal([]byte(token), &value) == nil && bodyPlaceholder.MatchString(value) {
			return true
		}
	}
	return false
}

func wholeBodyPlaceholder(value string) bool {
	return bodyPlaceholder.FindString(value) == value && value != ""
}

func normalizeBodyPlaceholders(body string) (string, map[int64]string) {
	var normalized strings.Builder
	bare := make(map[int64]string)
	inString := false
	for i := 0; i < len(body); {
		if inString {
			normalized.WriteByte(body[i])
			if body[i] == '\\' && i+1 < len(body) {
				i++
				normalized.WriteByte(body[i])
			} else if body[i] == '"' {
				inString = false
			}
			i++
			continue
		}
		if body[i] == '"' {
			inString = true
		} else if body[i] == '$' {
			if loc := bodyPlaceholder.FindStringIndex(body[i:]); loc != nil && loc[0] == 0 {
				normalized.WriteString("null")
				bare[int64(normalized.Len())] = body[i+1 : i+loc[1]]
				i += loc[1]
				continue
			}
		}
		normalized.WriteByte(body[i])
		i++
	}
	return normalized.String(), bare
}

func mapBodyJSONValue(decoder *json.Decoder, bare map[int64]string, input map[string]interface{}) ([]byte, bool, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, false, fmt.Errorf("invalid JSON API body template")
	}
	if key, ok := bare[decoder.InputOffset()]; ok {
		value, exists := input[key]
		if !exists {
			return nil, true, nil
		}
		return marshalBodyValue(value)
	}
	if delimiter, ok := token.(json.Delim); ok {
		var output bytes.Buffer
		output.WriteByte(byte(delimiter))
		for decoder.More() {
			var key []byte
			if delimiter == '{' {
				name, err := decoder.Token()
				if err != nil {
					return nil, false, fmt.Errorf("invalid JSON API body template")
				}
				// Configuration owns property names; only values are mapped.
				key, _ = json.Marshal(name)
			}
			value, missing, err := mapBodyJSONValue(decoder, bare, input)
			if err != nil {
				return nil, false, err
			}
			if missing {
				if delimiter == '{' {
					continue
				}
				return nil, false, fmt.Errorf("missing API body placeholder in an array element")
			}
			if output.Len() > 1 {
				output.WriteByte(',')
			}
			if key != nil {
				output.Write(key)
				output.WriteByte(':')
			}
			output.Write(value)
		}
		closing, err := decoder.Token()
		if err != nil {
			return nil, false, fmt.Errorf("invalid JSON API body template")
		}
		output.WriteByte(byte(closing.(json.Delim)))
		return output.Bytes(), false, nil
	}
	if value, ok := token.(string); ok {
		if wholeBodyPlaceholder(value) {
			mapped, exists := input[value[1:]]
			if !exists {
				return nil, true, nil
			}
			if mapped == nil {
				return marshalBodyValue(nil)
			}
			// Quoted placeholders retain their string position. Use a bare
			// placeholder when the upstream expects a number, array or object.
			return marshalBodyValue(fmt.Sprint(mapped))
		}
		var mappingErr error
		value = bodyPlaceholder.ReplaceAllStringFunc(value, func(match string) string {
			mapped, exists := input[match[1:]]
			if !exists || mapped == nil {
				mappingErr = fmt.Errorf("missing or null API body placeholder inside a JSON string")
				return ""
			}
			return fmt.Sprint(mapped)
		})
		if mappingErr != nil {
			return nil, false, mappingErr
		}
		return marshalBodyValue(value)
	}
	return marshalBodyValue(token)
}

func marshalBodyValue(value interface{}) ([]byte, bool, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, false, fmt.Errorf("invalid API body placeholder value")
	}
	return encoded, false, nil
}

func mapRawBody(body string, input map[string]interface{}) string {
	return bodyPlaceholder.ReplaceAllStringFunc(body, func(match string) string {
		value, exists := input[match[1:]]
		if !exists {
			return match
		}
		if text, ok := value.(string); ok {
			encoded, _ := json.Marshal(text)
			return string(encoded[1 : len(encoded)-1])
		}
		return fmt.Sprint(value)
	})
}
