package shared

import (
	"fmt"
	"os"
	"reflect"
)

var (
	Write      bool
	Outputpath string = "../../cmd/hyperbricks-docs/hyperbricks-examples/"
)

// Helper function to combine the main configuration string with a property-specific test case.
func JoinTestCode(hyperbricks string, propertyTest string) []string {
	return []string{fmt.Sprintf(hyperbricks, propertyTest), propertyTest}
}

// Helper function to convert a struct to a map for easy validation in test cases.
func StructToMap(data interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	val := reflect.ValueOf(data)
	typ := reflect.TypeOf(data)

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		value := val.Field(i)
		result[field.Name] = value.Interface()
	}

	return result
}

func StructToMapRecursive(input interface{}) map[string]interface{} {
	output := make(map[string]interface{})

	v := reflect.ValueOf(input)
	t := reflect.TypeOf(input)

	if v.Kind() == reflect.Ptr {
		v = v.Elem()
		t = t.Elem()
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Use the `mapstructure` tag if present
		tag := field.Tag.Get("mapstructure")
		if tag == "" || tag == ",squash" {
			tag = field.Name
		}

		// Convert the field value
		if fieldValue.CanInterface() {
			output[tag] = fieldValue.Interface()
		}
	}

	return output
}

// Helper function to write test outputs to a file for documentation purposes.
func WriteToFile(filename, content string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	return err
}

// cloneMap is a helper to do a shallow clone of map[string]interface{}
func CloneMap(source map[string]interface{}) map[string]interface{} {
	dest := make(map[string]interface{}, len(source))
	for k, v := range source {
		dest[k] = v
	}
	return dest
}
