// main_test.go
package main

import (
	"reflect"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/parser"
)

func Test_Preprocessing_Tests(t *testing.T) {
	return
	input := `
        $myname = John
        $mylastname = Doe
		$template_001 = {{TEMPLATE:test.html}}

        name = <TEXT>
        name.value = Hello {{VAR:myname}} {{VAR:mylastname}}!

		template = {{VAR:template_001}}

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
	preprocesses, _ := parser.PreprocessHyperScript(input)
	parsedConfig := parser.ParseHyperScript(preprocesses)

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
