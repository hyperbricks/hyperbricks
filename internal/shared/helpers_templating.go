package shared

import (
	"bytes"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/hairyhenderson/gomplate/v3"
)

// applyTemplate generates output based on the provided template and API data.
func ApplyTemplate(templateStr string, data map[string]interface{}) (string, []error) {
	var errors []error

	// Parse the template string
	tmpl, err := template.New("apiTemplate").Funcs(GetGenericFuncMap()).Parse(templateStr)
	if err != nil {
		errors = append(errors, ComponentError{
			Err:      fmt.Errorf("error parsing template: %v", err).Error(),
			Rejected: false,
		})
		// Handle parsing error gracefully
		return fmt.Sprintf("Error parsing template: %v", err), errors
	}

	// Execute the template with the provided data
	var output bytes.Buffer
	err = tmpl.Execute(&output, data)
	if err != nil {
		// Handle execution error gracefully
		errors = append(errors, ComponentError{

			Err:      fmt.Errorf("error executing template: %v", err).Error(),
			Rejected: false,
		})
		return fmt.Sprintf("error executing template: %v", err), errors
	}

	// Return the rendered output
	return output.String(), errors
}

var rnd = rand.New(rand.NewSource(time.Now().UnixNano()))

func Random(args ...interface{}) interface{} {
	if len(args) == 0 {
		return rnd.Int31()
	}

	val := args[0]
	v := reflect.ValueOf(val)

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		if v.Len() == 0 {
			return nil
		}
		return v.Index(rnd.Intn(v.Len())).Interface()

	case reflect.Map:
		keys := v.MapKeys()
		if len(keys) == 0 {
			return nil
		}
		randKey := keys[rnd.Intn(len(keys))]
		return v.MapIndex(randKey).Interface()

	case reflect.String:
		s := v.String()
		if len(s) == 0 {
			return ""
		}
		return string(s[rnd.Intn(len(s))])

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		max := v.Int()
		if max == 0 {
			return 0
		}
		if max > 0 {
			return rnd.Int63n(max + 1)
		}
		return -rnd.Int63n(-max + 1)

	default:
		return nil
	}
}

// Create a FuncMap with a custom function
var FuncMap = template.FuncMap{
	"random_num": Random,
	"valueOrEmpty": func(value interface{}) string {
		if value == nil {
			return ""
		}
		return fmt.Sprintf("%v", value)
	},
}

var SprigFuncMap = sprig.FuncMap()
var Gomplate = gomplate.CreateFuncs(nil, nil)

// Define individual function maps
var baseFuncs = template.FuncMap{
	"upper": func(s string) string { return strings.ToUpper(s) },
}

var sprigFuncs = template.FuncMap{
	"lower": func(s string) string { return strings.ToLower(s) }, // Replace with actual Sprig function
}

var gomplateFuncs = template.FuncMap{
	"reverse": func(s string) string {
		runes := []rune(s)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return string(runes)
	}, // Replace with actual Gomplate function
}

// Lazy load function map based on context
func GetGenericFuncMap() template.FuncMap {
	funcMap := make(template.FuncMap)

	// Always load base functions
	for k, v := range FuncMap {
		funcMap[k] = v
	}

	for k, v := range sprigFuncs {
		funcMap[k] = v
	}

	for k, v := range gomplateFuncs {
		funcMap[k] = v
	}

	return funcMap
}
