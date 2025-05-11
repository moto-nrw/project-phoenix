package users

import (
	"errors"
	"fmt"
)

// Common error variables for the users service
var (
	// ErrPersonNotFound indicates a person could not be found
	ErrPersonNotFound = errors.New("person not found")

	// ErrInvalidPersonData indicates invalid person data
	ErrInvalidPersonData = errors.New("invalid person data")

	// ErrPersonIdentifierRequired indicates missing required identifier
	ErrPersonIdentifierRequired = errors.New("either tag ID or account ID is required")

	// ErrAccountNotFound indicates an account could not be found
	ErrAccountNotFound = errors.New("account not found")

	// ErrRFIDCardNotFound indicates an RFID card could not be found
	ErrRFIDCardNotFound = errors.New("RFID card not found")

	// ErrAccountAlreadyLinked indicates an account is already linked to another person
	ErrAccountAlreadyLinked = errors.New("account is already linked to another person")

	// ErrRFIDCardAlreadyLinked indicates an RFID card is already linked to another person
	ErrRFIDCardAlreadyLinked = errors.New("RFID card is already linked to another person")

	// ErrGuardianNotFound indicates a guardian could not be found
	ErrGuardianNotFound = errors.New("guardian not found")
)

// UsersError represents an error in the users service
type UsersError struct {
	Op  string // Operation that failed
	Err error  // Underlying error
}

// Error returns the error message
func (e *UsersError) Error() string {
	return fmt.Sprintf("users.%s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error
func (e *UsersError) Unwrap() error {
	return e.Err
}
