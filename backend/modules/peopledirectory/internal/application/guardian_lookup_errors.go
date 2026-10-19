package application

// guardianLookupError retains the lookup's wire diagnostic and cause while
// exposing the refusal kind to the capability mapper.
type guardianLookupError struct {
	kind, cause error
	message     string
}

func (e guardianLookupError) Error() string        { return e.message }
func (e guardianLookupError) Unwrap() error        { return e.cause }
func (e guardianLookupError) Is(target error) bool { return target == e.kind }
func guardianLookupFailure(kind, cause error) error {
	return guardianLookupError{kind: kind, cause: cause, message: cause.Error()}
}
