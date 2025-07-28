// typefactory/typefactory.go
package typefactory

import (
	"fmt"
	"reflect"

	"github.com/mitchellh/mapstructure"
)

// TypeRequest represents a request to create an instance of a type.
type TypeRequest struct {
	TypeName string
	Data     map[string]interface{}
}

// TypeResponse represents the response from TypeFactory.
type TypeResponse struct {
	Instance interface{}
	Warnings []string
	Error    error
}

// TypeFactory is responsible for creating instances of types based on type names.
type TypeFactory struct {
	types map[string]reflect.Type
}

// NewTypeFactory initializes a new TypeFactory.
func NewTypeFactory() *TypeFactory {
	return &TypeFactory{
		types: make(map[string]reflect.Type),
	}
}

// RegisterType registers a new type with the factory.
func (tf *TypeFactory) RegisterType(typeName string, typ reflect.Type) {
	tf.types[typeName] = typ
}

// CreateInstance creates an instance of the requested type and validates it.
func (tf *TypeFactory) CreateInstance(request TypeRequest) (*TypeResponse, error) {

	typ, exists := tf.types[request.TypeName]
	if !exists {
		return nil, fmt.Errorf("type %s not registered", request.TypeName)
	}

	instancePtr := reflect.New(typ)
	instance := instancePtr.Interface()

	// Compose both decode hooks
	combinedHook := mapstructure.ComposeDecodeHookFunc(
		StringToSliceHookFunc(),
		StringToIntHookFunc(),
		StringToMapStringHookFunc(),
	)

	// Set up the decoder with appropriate configuration
	decoderConfig := &mapstructure.DecoderConfig{
		Metadata:         nil,
		DecodeHook:       combinedHook,
		Result:           instance,
		TagName:          "mapstructure",
		WeaklyTypedInput: true,
	}

	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		return nil, err
	}

	err = decoder.Decode(request.Data)
	if err != nil {
		return nil, err
	}

	// Dereference the pointer to get the value
	instanceValue := reflect.ValueOf(instance).Elem().Interface()

	// Perform validation if the instance has a Validate method
	var warnings []string
	if v, ok := instanceValue.(interface{ Validate() []string }); ok {
		warnings = v.Validate()
	}

	return &TypeResponse{
		Instance: instanceValue,
		Warnings: warnings,
	}, nil
}
