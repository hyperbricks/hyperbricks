package shared

import (
	"bytes"
	"fmt"
	"html/template"
	"math/rand"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/sprig/v3"
)

// applyTemplate generates output based on the provided template and API data.
func ApplyTemplate(templateStr string, data map[string]interface{}) (string, []error) {
	var errors []error

	// Parse the template string
	tmpl, err := ParsedGenericTemplate(templateStr)
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
	"random": Random,
	"safe": func(value interface{}) template.HTML {
		switch typed := value.(type) {
		case template.HTML:
			return typed
		case string:
			return template.HTML(typed)
		default:
			return template.HTML(fmt.Sprintf("%v", typed))
		}
	},
	"valueOrEmpty": func(value interface{}) string {
		if value == nil {
			return ""
		}
		return fmt.Sprintf("%v", value)
	},
}

var sprigFuncs = sprig.GenericFuncMap()

// Lazy load function map based on context
func GetGenericFuncMap() template.FuncMap {
	funcMap := sprigFuncs

	// Always load base functions
	for k, v := range FuncMap {
		funcMap[k] = v
	}

	return funcMap
}

var baseTemplate = template.New("hyperbricks-generic-template").Funcs(GetGenericFuncMap())
var genericTemplateCache sync.Map

// // Why use clone ?
// Prevents concurrent modification: Clone() creates a separate instance per goroutine, avoiding map modification issues.
// Better performance: Reusing a base template reduces overhead compared to creating a new one every time.
// Cleaner & safer: No need for sync.Mutex, and avoids unnecessary allocations.
func GenericTemplate() *template.Template {
	tmpl, _ := baseTemplate.Clone() // Clone ensures thread safety
	return tmpl
}

func ParsedGenericTemplate(templateStr string) (*template.Template, error) {
	return ParsedNamedTemplate("hyperbricks-generic-template", templateStr)
}

func ParsedNamedTemplate(name, templateStr string) (*template.Template, error) {
	if name == "" {
		name = "hyperbricks-generic-template"
	}
	key := struct{ Name, Source string }{name, templateStr}
	if value, ok := genericTemplateCache.Load(key); ok {
		return value.(*template.Template), nil
	}

	base := GenericTemplate()
	if name != base.Name() {
		base = base.New(name)
	}
	tmpl, err := base.Parse(templateStr)
	if err != nil {
		return nil, err
	}
	if name != "hyperbricks-generic-template" && tmpl.Tree != nil && strings.TrimSpace(tmpl.Tree.Root.String()) == "" {
		if declared := tmpl.Lookup("hyperbricks-generic-template"); declared != nil && declared.Tree != nil && declared.Tree.Root != nil {
			tmpl = declared
		}
	}

	value, _ := genericTemplateCache.LoadOrStore(key, tmpl)
	return value.(*template.Template), nil
}
