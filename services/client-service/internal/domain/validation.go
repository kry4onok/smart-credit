package domain

type ValidationError struct {
	Cause  error
	Issues []string
}

func (e *ValidationError) Error() string { return e.Cause.Error() }
func (e *ValidationError) Unwrap() error { return e.Cause }

func InvalidFields(cause error, issues ...string) error {
	if len(issues) == 0 {
		return nil
	}
	return &ValidationError{Cause: cause, Issues: issues}
}
