package component

import (
	"fmt"
	"os"
	"reflect"
)

var (
	write      bool
	outputpath string = "../../cmd/typerscript-docs/typerscript-examples/"
)

func joinTestCode(typerscript string, propertyTest string) []string {
	return []string{fmt.Sprintf(typerscript, propertyTest), propertyTest}
}

func structToMap(data interface{}) map[string]interface{} {
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

func writeToFile(filename, content string) error {
	// Create or overwrite the file
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write the content to the file
	_, err = file.WriteString(content)
	return err
}
