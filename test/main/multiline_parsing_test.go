package main

import (
	"log"
	"reflect"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/parser"
)

// GetNestedValue retrieves a nested value from a map[string]interface{} using a sequence of keys.
func GetNestedValue(m map[string]interface{}, keys ...string) (interface{}, bool) {
	var current interface{} = m
	for _, key := range keys {
		// Assert current is map[string]interface{} to access the next level
		if m, ok := current.(map[string]interface{}); ok {
			current = m[key]
		} else {
			// If the type assertion fails, the chain is broken, and we return
			log.Printf("Type assertion failed at key '%s'. Current structure: %v", key, current)
			return nil, false
		}
	}
	return current, true
}

func TestMultilineParsing_001(t *testing.T) {
	// Define a sample HyperBricks with a MARKDOWN block and some comments
	hyperBricks := `header_test.header {
	10 = <HTML>
	10.value = <<[
<meta name="viewport" content="width=device-width, initial-scale=1.0">
	]>>
}`

	// Expected output should contain the HTML value after parsing
	expected := map[string]interface{}{
		"header_test": map[string]interface{}{
			"header": map[string]interface{}{
				"10": map[string]interface{}{
					"@type": "<HTML>",
					"value": "\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n\t",
				},
			},
		},
	}

	parser.KnownTypes["<HTML>"] = true

	// Call ParseHyperScript function with the HyperBricks
	parsedConfig := parser.ParseHyperScript(hyperBricks)

	// Inspect structure of parsed output for debugging
	log.Printf("Parsed output structure: %v", parsedConfig)

	// Compare the parsed config with the expected config using reflect.DeepEqual
	if !reflect.DeepEqual(parsedConfig, expected) {
		t.Errorf("Test failed!\nExpected:\n%#v\nGot:\n%#v", expected, parsedConfig)
	}
}

func TestMultilineParsing_002(t *testing.T) {
	// Define a sample HyperBricks
	hyperBricks := `header_test.header {
	10 = <HTML>
	10.value = <<[<meta name="viewport" content="width=device-width, initial-scale=1.0">]>>
}`

	// Expected output should contain the HTML value after parsing
	expected := map[string]interface{}{
		"header_test": map[string]interface{}{
			"header": map[string]interface{}{
				"10": map[string]interface{}{
					"@type": "<HTML>",
					"value": "<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">",
				},
			},
		},
	}

	parser.KnownTypes["<HTML>"] = true
	// Call ParseHyperScript function with the HyperBricks
	parsedConfig := parser.ParseHyperScript(hyperBricks)

	// Inspect structure of parsed output for debugging
	log.Printf("Parsed output structure: %v", parsedConfig)

	// Compare the parsed config with the expected config using reflect.DeepEqual
	if !reflect.DeepEqual(parsedConfig, expected) {
		t.Errorf("Test failed!\nExpected:\n%#v\nGot:\n%#v", expected, parsedConfig)
	}
}

func TestMultilineParsing_003(t *testing.T) {
	// Define a sample HyperBricks with a MARKDOWN block and some comments
	hyperBricks := `header_test.header {
	10 = <HTML>
	10.value = <<[
	<meta name="viewport" content="width=device-width, initial-scale=1.0">

	]>>
}`

	// Expected output should contain the HTML value after parsing
	expected := map[string]interface{}{
		"header_test": map[string]interface{}{
			"header": map[string]interface{}{
				"10": map[string]interface{}{
					"@type": "<HTML>",
					"value": "\n\t<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n\n\t",
				},
			},
		},
	}

	// Call ParseHyperScript function with the HyperBricks
	parser.KnownTypes["<HTML>"] = true
	parsedConfig := parser.ParseHyperScript(hyperBricks)

	// Inspect structure of parsed output for debugging
	log.Printf("Parsed output structure: %v", parsedConfig)

	// Compare the parsed config with the expected config using reflect.DeepEqual
	if !reflect.DeepEqual(parsedConfig, expected) {
		t.Errorf("Test failed!\nExpected:\n%#v\nGot:\n%#v", expected, parsedConfig)
	}
}

func TestRejectArrayBracketsFormingWhenUsedInValue(t *testing.T) {
	// Define a sample HyperBricks with a MARKDOWN block and some comments
	hyperBricks := `
	
	somevalue = this is not [an array] it is just a text

}`

	// Expected output should contain the HTML value after parsing
	expected := map[string]interface{}{
		"somevalue": "this is not [an array] it is just a text",
	}

	// Call ParseHyperScript function with the HyperBricks
	parser.KnownTypes["<HTML>"] = true
	parsedConfig := parser.ParseHyperScript(hyperBricks)

	// Inspect structure of parsed output for debugging
	log.Printf("Parsed output structure: %v", parsedConfig)

	// Compare the parsed config with the expected config using reflect.DeepEqual
	if !reflect.DeepEqual(parsedConfig, expected) {
		t.Errorf("Test failed!\nExpected:\n%#v\nGot:\n%#v", expected, parsedConfig)
	}
}

func TestRejectArrayBracketsFormingWhenUsedInVariableValue(t *testing.T) {

	hyperBricks := `
	
	$grid_background =  bg-[#333] px-2 rounded-lg shadow-md

	top_section = <HTML>
	top_section.value = <<[
		<div class="flex flex-row justify-left items-center {{VAR:grid_background}}"></div>
	]>>

}`

	// Expected output should contain the HTML value after parsing
	expected := map[string]interface{}{
		"top_section": map[string]interface{}{
			"@type": "<HTML>",
			"value": "\n\t\t<div class=\"flex flex-row justify-left items-center bg-[#333] px-2 rounded-lg shadow-md\"></div>\n\t",
		},
	}

	// Call ParseHyperScript function with the HyperBricks
	parser.KnownTypes["<HTML>"] = true
	parsedConfig := parser.ParseHyperScript(hyperBricks)

	// Inspect structure of parsed output for debugging
	log.Printf("Parsed output structure: %v", parsedConfig)

	// Compare the parsed config with the expected config using reflect.DeepEqual
	if !reflect.DeepEqual(parsedConfig, expected) {
		t.Errorf("Test failed!\nExpected:\n%#v\nGot:\n%#v", expected, parsedConfig)
	}
}

func TestMultilineParsing_004(t *testing.T) {
	// Define a sample HyperBricks with a MARKDOWN block and some comments
	hyperBricks := `header_test.header {
	10 = <HTML>
	10.value = <<[<meta name="viewport" content="width=device-width, initial-scale=1.0">
	]>>

}`

	// Expected output should contain the HTML value after parsing
	expected := map[string]interface{}{
		"header_test": map[string]interface{}{
			"header": map[string]interface{}{
				"10": map[string]interface{}{
					"@type": "<HTML>",
					"value": "<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n\t",
				},
			},
		},
	}

	// Call ParseHyperScript function with the HyperBricks
	parser.KnownTypes["<HTML>"] = true
	parsedConfig := parser.ParseHyperScript(hyperBricks)

	// Inspect structure of parsed output for debugging
	log.Printf("Parsed output structure: %v", parsedConfig)

	// Compare the parsed config with the expected config using reflect.DeepEqual
	if !reflect.DeepEqual(parsedConfig, expected) {
		t.Errorf("Test failed!\nExpected:\n%#v\nGot:\n%#v", expected, parsedConfig)
	}
}

func TestMultilineParsing_005(t *testing.T) {
	// spaces after multiline close ]>>
	hyperBricks := `header_test.header {
	10 = <HTML>
	10.value = <<[<meta name="viewport" content="width=device-width, initial-scale=1.0">
	]>>   

}`

	// Expected output should contain the HTML value after parsing
	expected := map[string]interface{}{
		"header_test": map[string]interface{}{
			"header": map[string]interface{}{
				"10": map[string]interface{}{
					"@type": "<HTML>",
					"value": "<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n\t",
				},
			},
		},
	}

	// Call ParseHyperScript function with the HyperBricks
	parser.KnownTypes["<HTML>"] = true
	parsedConfig := parser.ParseHyperScript(hyperBricks)

	// Inspect structure of parsed output for debugging
	log.Printf("Parsed output structure: %v", parsedConfig)

	// Compare the parsed config with the expected config using reflect.DeepEqual
	if !reflect.DeepEqual(parsedConfig, expected) {
		t.Errorf("Test failed!\nExpected:\n%#v\nGot:\n%#v", expected, parsedConfig)
	}
}
