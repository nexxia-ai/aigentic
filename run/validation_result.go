package run

type ValidationResult struct {
	Values           any
	Message          string
	ValidationErrors []error
}
