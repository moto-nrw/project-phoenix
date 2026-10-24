package importpkg

import (
	"errors"
	"fmt"
)

// copyError carries text shown verbatim in an import row, not diagnostic detail.
// The cause stays separate so Unwrap, Is and As retain their original behavior.
type copyError struct {
	message string
	cause   error
}

func (e *copyError) Error() string { return e.message }
func (e *copyError) Unwrap() error { return e.cause }

func newCopyError(message string) error {
	return &copyError{message: message}
}

func copyErrorf(format string, args ...any) error {
	formatted := fmt.Errorf(format, args...)
	return &copyError{message: formatted.Error(), cause: errors.Unwrap(formatted)}
}
