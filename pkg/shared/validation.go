package shared

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

// use a single instance of Validate, it caches struct info
var validate *validator.Validate

func init() {
	validate = validator.New(validator.WithRequiredStructEnabled())
}

type PathKeyer interface {
	GetPath() string
	GetKey() string
	GetConfigType() string
}

func (c ComponentRendererConfig) GetPath() string       { return c.HyperBricksPath }
func (c ComponentRendererConfig) GetKey() string        { return c.HyperBricksKey }
func (c ComponentRendererConfig) GetConfigType() string { return c.ConfigType }

func Validate(config interface{}) []error {
	var errors []error

	err := validate.Struct(config)
	if err != nil {
		pk, ok := config.(PathKeyer)
		if !ok {
			return []error{fmt.Errorf("config does not implement PathKeyer")}
		}
		for _, err := range err.(validator.ValidationErrors) {

			switch err.ActualTag() {

			case "required":

				errors = append(errors, ComponentError{
					Path:     pk.GetPath(),
					Key:      pk.GetKey(),
					Err:      fmt.Sprintf("%s: is required\n", err.Field()),
					Type:     pk.GetConfigType(),
					Rejected: false,
				})

				// case "lte":
				// 	errors = append(errors, ComponentError{
				// 		Path:     pk.GetPath(),
				// 		Key:      pk.GetKey(),
				// 		Err:      fmt.Sprintf("%s: is way to old!\n", err.Field()),
				// 		Type:     pk.GetConfigType(),
				// 		Rejected: false,
				// 	})
			}

		}
		return errors
	}
	return nil
}

// validator error fields:
// fmt.Println(err.Namespace())
// fmt.Println(err.Field()) // <<<-----
// fmt.Println(err.StructNamespace())
// fmt.Println(err.StructField())
// fmt.Println(err.Tag())
// fmt.Println(err.ActualTag())
// fmt.Println(err.Kind())
// fmt.Println(err.Type()) // <<<-----
// fmt.Println(err.Value())
// fmt.Println(err.Param())
// fmt.Println()
