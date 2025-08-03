// main_test.go
package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/parser"
)

// normalizeString trims and removes excess whitespace for comparison purposes.
func normalizeString(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func TestStripCDATAAndStore_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedOutput string
		expectedStore  map[string]string
	}{
		{
			name: "hyperbricks page config",
			input: `example = PAGE
example.title = example
example.route = example
example.10 = <TEMPLATE>
example.10 {
    template = <![test_template[<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>{{my_header_text}}</title>
</head>
<body>
    <h1>{{my_header_text}}</h1>
    <p>{{my_text}}</p>
</body>
</html>]]>
    values {
       my_header_text = "Welcome"
       my_text = "This is an inline template example."
    }
}

example.20 = <TEXT>
example.20.value = TEST TEXT`,
			expectedOutput: `example = PAGE
example.title = example
example.route = example
example.10 = <TEMPLATE>
example.10 {
    template = test_template
    values {
       my_header_text = "Welcome"
       my_text = "This is an inline template example."
    }
}

example.20 = <TEXT>
example.20.value = TEST TEXT`,
			expectedStore: map[string]string{
				"test_template": `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>{{my_header_text}}</title>
</head>
<body>
    <h1>{{my_header_text}}</h1>
    <p>{{my_text}}</p>
</body>
</html>`,
			},
		},
		{
			name: "Empty CDATA",
			input: `
            template.10 = <![title[]]>
            `,
			expectedOutput: `
            template.10 = title
            `,
			expectedStore: map[string]string{
				"title": "",
			},
		},
		{
			name: "Malformed CDATA",
			input: `
            template.10 = <![title[Welcome to the Page]>
            `,
			expectedOutput: `
            template.10 = <![title[Welcome to the Page]>
            `,
			expectedStore: map[string]string{},
		},
		{
			name: "Special Characters",
			input: `
            template.10 = <![title[Welcome to the Page & Enjoy!]]>
            `,
			expectedOutput: `
            template.10 = title
            `,
			expectedStore: map[string]string{
				"title": "Welcome to the Page & Enjoy!",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear the global template store

			parser.ClearTemplateStore()

			// Run the StripCDATAAndStore function
			output := parser.StripCDATAAndStore(tt.input)
			outputNormalized := normalizeString(output)
			expectedOutputNormalized := normalizeString(tt.expectedOutput)

			// Check if the output matches the expected output
			if outputNormalized != expectedOutputNormalized {
				t.Errorf("Expected output:\n%s\nGot:\n%s\n", expectedOutputNormalized, outputNormalized)
			}

			// Retrieve the template store and check if it matches the expected store
			store := parser.GetTemplateStore()
			if !reflect.DeepEqual(store, tt.expectedStore) {
				t.Errorf("Expected templateStore:\n%v\nGot:\n%v\n", tt.expectedStore, store)
			}
		})
	}
}

func TestStripCDATAAndStore(t *testing.T) {
	// Test input with multiple CDATA sections and metadata
	input := `
template.10 = <![title[Welcome to the Page]]>
template.20 = <![description[This page is a simple test of the system]]>
other.content = Some other text without CDATA
template.30 = <![footer[Footer content here]]>
`

	parser.ClearTemplateStore()

	// Expected modified input after CDATA replacements
	expectedOutput := `
template.10 = title
template.20 = description
other.content = Some other text without CDATA
template.30 = footer
`

	// Expected values in templateStore after extraction
	expectedTemplates := map[string]string{
		"title":       "Welcome to the Page",
		"description": "This page is a simple test of the system",
		"footer":      "Footer content here",
	}

	// Run the function
	output := parser.StripCDATAAndStore(input)

	// Normalize line endings and trim spaces for comparison
	outputNormalized := normalizeString(output)
	expectedOutputNormalized := normalizeString(expectedOutput)

	// Check if the output matches expected modified input
	if outputNormalized != expectedOutputNormalized {
		t.Errorf("Expected output:\n%s\nGot:\n%s\n", expectedOutputNormalized, outputNormalized)
	}

	// Check if templateStore contains expected templates
	store := parser.GetTemplateStore()
	if !reflect.DeepEqual(store, expectedTemplates) {
		t.Errorf("Expected TemplateStore:\n%v\nGot:\n%v\n", expectedTemplates, store)
	}
}

// Test function for StripComments
func TestStripComments(t *testing.T) {
	// Define test cases
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Basic single-line comment",
			input:    `var x = 10; // This is a comment`,
			expected: `var x = 10; `,
		},
		{
			name:     "URL with // and single-line comment",
			input:    `3.url = https://dummyimage.com/300 // This is an image`,
			expected: `3.url = https://dummyimage.com/300 `,
		},
		{
			name:     "Multi-line comment",
			input:    `var y = 20; /* Multi-line comment */ var z = 30;`,
			expected: `var y = 20;  var z = 30;`,
		},
		{
			name:     "String with // inside",
			input:    `var str = "This is a URL: http://example.com";`,
			expected: `var str = "This is a URL: http://example.com";`,
		},
		{
			name:     "Leave a # direct after =",
			input:    `hx_reselect = #response`,
			expected: `hx_reselect = #response`,
		},
		{
			name:     "Hash comment after code",
			input:    `var a = 5; # A hash comment`,
			expected: `var a = 5; # A hash comment`,
		},
		{
			name:     "String with # inside",
			input:    `var path = "/usr/local/bin"; # Path comment`,
			expected: `var path = "/usr/local/bin"; # Path comment`,
		},
		{
			name: "Complex example with mixed comments",
			input: `3 = IMAGE
3.url = https://dummyimage.com/300 // This is an image
/* A multi-line
comment */
var str = 'She said, "Hello!"'; # Another comment
var path = "/usr/local/bin";`,
			expected: `3 = IMAGE
3.url = https://dummyimage.com/300 
var str = 'She said, "Hello!"'; # Another comment
var path = "/usr/local/bin";`,
		},
	}

	// Run each test case
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.StripComments(tt.input)
			if result != tt.expected {
				t.Errorf("StripComments(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseHyperScript(t *testing.T) {
	input := `
    content = <TREE>
    content.1 = First
    content.2 = Second

    # This is a single-line comment
    page = <HYPERMEDIA>
    page.20 = <TEXT>
    page.20.value = Another text // Another comment

	
    page.10 = <TEXT>
    page.10.value = <<[Hello,
World!
This is a multiline text.]>>
    page.30 = <TREE>
    page.30 {
        1 = <TEXT>
        1.value = custom text /* Inline multi-line comment */
       
        6 < page.10
        7 < content
    }
    someArray = [a, b, c, d]
    `
	parser.KnownTypes["<TEXT>"] = true
	parser.KnownTypes["<HYPERMEDIA>"] = true
	parser.KnownTypes["<TREE>"] = true
	// Parse the input

	parsedConfig := parser.ParseHyperScript(parser.StripComments(input))

	// Construct expected configuration as a nested ConfigObject
	expected := map[string]interface{}{
		"content": map[string]interface{}{
			"@type": "<TREE>",
			"1":     "First",
			"2":     "Second",
		},
		"page": map[string]interface{}{
			"@type": "<HYPERMEDIA>",
			"10": map[string]interface{}{
				"@type": "<TEXT>",
				"value": "Hello,\nWorld!\nThis is a multiline text.",
			},
			"20": map[string]interface{}{
				"@type": "<TEXT>",
				"value": "Another text",
			},
			"30": map[string]interface{}{
				"@type": "<TREE>",
				"1": map[string]interface{}{
					"@type": "<TEXT>",
					"value": "custom text",
				},
				"6": map[string]interface{}{
					"@type": "<TEXT>",
					"value": "Hello,\nWorld!\nThis is a multiline text.",
				},
				"7": map[string]interface{}{
					"@type": "<TREE>",
					"1":     "First",
					"2":     "Second",
				},
			},
		},
		"someArray": []interface{}{"a", "b", "c", "d"},
	}

	// Compare the parsed config with the expected config using reflect.DeepEqual
	if !reflect.DeepEqual(parsedConfig, expected) {
		t.Errorf("Test failed!\nExpected:\n%#v\nGot:\n%#v", expected, parsedConfig)
	}
}

func TestParseHyperScriptWithVariables(t *testing.T) {
	input := `
    $myname = John
	$mylastname = Doe

	name = <TEXT>
	name.value = Hello {{VAR:myname}} {{VAR:mylastname}}!

	greeting {
		message = <TEXT>
		message.value = Welcome, {{VAR:myname}}!
	}
    `
	parser.KnownTypes["<TEXT>"] = true
	// Parse the input
	parsedConfig := parser.ParseHyperScript(input)

	// Construct expected configuration as a nested ConfigObject
	expected := map[string]interface{}{
		"greeting": map[string]interface{}{
			"message": map[string]interface{}{
				"@type": "<TEXT>", "value": "Welcome, John!"}},
		"name": map[string]interface{}{
			"@type": "<TEXT>",
			"value": "Hello John Doe!",
		},
	}

	// Compare the parsed config with the expected config using reflect.DeepEqual
	if !reflect.DeepEqual(parsedConfig, expected) {
		t.Errorf("Test failed!\nExpected:\n%#v\nGot:\n%#v", expected, parsedConfig)
	}
}

func TestParseLines_HandleBothSyntaxes(t *testing.T) {
	input := `
        $myname = John
        $mylastname = Doe

        name = <TEXT>
        name.value = Hello {{VAR:myname}} {{VAR:mylastname}}!

        greeting {
            message = <TEXT>
            message.value = Welcome, {{VAR:myname}}!
        }

        myvalues {
            just_funny = hehe
            just_laughing = laughing
        }

        othervalues = {
            key1 = value1
            key2 = value2
        }
    `
	parser.KnownTypes["<TEXT>"] = true
	// Parse the input using the modified parser
	parsedConfig := parser.ParseHyperScript(input)

	// Construct the expected configuration as a nested map
	expected := map[string]interface{}{
		"name": map[string]interface{}{
			"@type": "<TEXT>",
			"value": "Hello John Doe!",
		},
		"greeting": map[string]interface{}{
			"message": map[string]interface{}{
				"@type": "<TEXT>",
				"value": "Welcome, John!",
			},
		},
		"myvalues": map[string]interface{}{
			"just_funny":    "hehe",
			"just_laughing": "laughing",
		},
		"othervalues": map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		},
	}

	// Compare the parsed config with the expected config using reflect.DeepEqual
	if !reflect.DeepEqual(parsedConfig, expected) {
		t.Errorf("Test failed!\nExpected:\n%#v\nGot:\n%#v", expected, parsedConfig)
	}
}
func TestParseLines_NestedBlocksAndArrays(t *testing.T) {
	input := `
        $appName = HyperBricks
        $version = 1.0.0

        app = {
            name = {{VAR:appName}}
            version = {{VAR:version}}
            features {
                feature1 = Enabled
                feature2 = Disabled
            }
            supported_languages = [
                Go,
                Python,
                JavaScript
            ]
        }
    `

	// Parse the input using the parser
	parsedConfig := parser.ParseHyperScript(input)

	// Construct the expected configuration as a nested map
	expected := map[string]interface{}{
		"app": map[string]interface{}{
			"name":    "HyperBricks",
			"version": "1.0.0",
			"features": map[string]interface{}{
				"feature1": "Enabled",
				"feature2": "Disabled",
			},
			"supported_languages": []interface{}{"Go", "Python", "JavaScript"},
		},
	}

	// Compare the parsed config with the expected config using reflect.DeepEqual
	if !reflect.DeepEqual(parsedConfig, expected) {
		t.Errorf("Test failed!\nExpected:\n%#v\nGot:\n%#v", expected, parsedConfig)
	}
}
