package main

import (
	"testing"
)

// ComponentError represents an error specific to components.
type ComponentError struct {
	Message string
}

func (e ComponentError) Error() string {
	return e.Message
}
func Test_ErrorWrapping(t *testing.T) {
	err := []error{
		ComponentError{Message: "invalid type for ComponentRenderer"},
		ComponentError{Message: "Another message ComponentRenderer"},
	}
	e := err[0].(ComponentError)
	if e.Message != "invalid type for ComponentRenderer" {
		t.Error("Dit werkt, of niet?")
	}
}
