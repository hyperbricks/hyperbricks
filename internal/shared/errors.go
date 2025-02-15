package shared

// ComponentError represents an error associated with a component.
type ComponentError struct {
	File     string // Hyperbricks file
	Err      string // A descriptive error message
	Key      string // Current key of the component where the error occured
	Path     string // Path of the component in the hierarchy
	Rejected bool   // Rendering is rejected
	Type     string // Which HyperBricks type?
	Level    string // INFO, WARNING, otherwise ERROR
}

// ComponentError represents an error associated with a component.
type CompositeError struct {
	Err      string // A descriptive error message
	Key      string // Key of the component
	Path     string // Path of the component in the hierarchy
	Rejected bool   // Rendering is rejected
	Type     string // Which HyperBricks type?
	Level    string // INFO, WARNING, otherwise ERROR
}

// Error implements the error interface for ComponentError.
func (e CompositeError) Error() string {
	return e.Err
}

// Error implements the error interface for ComponentError.
func (e ComponentError) Error() string {
	return e.Err
}
