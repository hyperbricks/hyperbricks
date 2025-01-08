package main

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/hyperbricks/hyperbricks/internal/composite"
)

// FieldDoc represents a field documentation entry
type _FieldDoc struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Mapstructure string `json:"mapstructure"`
	Validate     string `json:"validate"`
	Description  string `json:"description"`
	Example      string `json:"example"`
}

func Discover(input any) {

	// Get the type of the struct
	mainType := reflect.TypeOf(input)

	// Loop through the fields
	for i := 0; i < mainType.NumField(); i++ {
		field := mainType.Field(i)

		// Check if the field is an embedded struct
		if field.Anonymous {
			fmt.Printf("Field %s is an embedded struct of type %s\n", field.Name, field.Type)
		} else {
			fmt.Printf("Field %s is not embedded\n", field.Name)
		}
	}
}

func GenerateTheDocs(input any) []_FieldDoc {

	t := reflect.TypeOf(input)

	// Dereference pointer types if necessary
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil
	}

	var docs []_FieldDoc

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fmt.Printf("%v: %v\n", field.Name, field.Tag.Get("description"))

		fieldDoc := _FieldDoc{
			Name:         field.Type.PkgPath(),
			Type:         field.Tag.Get("@type"),
			Mapstructure: field.Tag.Get("mapstructure"),
			Validate:     field.Tag.Get("validate"),
			Description:  field.Tag.Get("description"),
			Example:      field.Tag.Get("example"),
		}

		fmt.Printf("field:%s \n", fieldDoc.Mapstructure)
		fmt.Printf("type:%s \n", fieldDoc.Type)
		fmt.Printf("description:%s \n", fieldDoc.Description)
		fmt.Printf("validation:%s \n", fieldDoc.Validate)
		fmt.Printf("example:%s \n", fieldDoc.Example)
		docs = append(docs, fieldDoc)

	}
	return docs
}

func Test_Api(t *testing.T) {
	apiConfig := composite.HyperMediaConfig{}
	GenerateTheDocs(apiConfig)
	//Discover(apiConfig)
}
