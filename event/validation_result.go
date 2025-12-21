package event

// ValidationResult represents the result of validating tool input parameters.
type ValidationResult struct {
	Values           any
	Message          string
	ValidationErrors []error
}
