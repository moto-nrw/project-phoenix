package users

import "errors"

// Domain-specific errors for the person service
var (
	// ErrPersonIdentifierRequired indicates that a person must have either a TagID or AccountID
	ErrPersonIdentifierRequired = errors.New("either TagID or AccountID must be set for a person")

	// ErrPersonNotFound indicates that the requested person doesn't exist
	ErrPersonNotFound = errors.New("person not found")

	// ErrAccountNotFound indicates that the referenced account doesn't exist
	ErrAccountNotFound = errors.New("account not found")

	// ErrRFIDCardNotFound indicates that the referenced RFID card doesn't exist
	ErrRFIDCardNotFound = errors.New("RFID card not found")

	// ErrAccountAlreadyLinked indicates that the account is already linked to another person
	ErrAccountAlreadyLinked = errors.New("account already linked to another person")

	// ErrRFIDCardAlreadyLinked indicates that the RFID card is already linked to another person
	ErrRFIDCardAlreadyLinked = errors.New("RFID card already linked to another person")
)
