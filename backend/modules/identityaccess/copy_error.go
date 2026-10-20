package identityaccess

// copyError preserves role-validation copy displayed in staff import rows.
// It stays in this owner rather than adding a cross-owner dependency.
type copyError struct{ message string }

func (e *copyError) Error() string { return e.message }
func (e *copyError) Unwrap() error { return nil }

func newCopyError(message string) error {
	return &copyError{message: message}
}
