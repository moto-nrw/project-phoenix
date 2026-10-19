package identityaccess

import "errors"

// Operator input identities retain the current diagnostic text.
var (
	ErrOperatorEmailInvalid        = errors.New("invalid email format")
	ErrOperatorEmailTooLong        = errors.New("email address too long")
	ErrOperatorPasswordTooWeak     = errors.New("password doesn't meet complexity requirements")
	ErrOperatorDisplayNameRequired = errors.New("display name is required")
	ErrOperatorDisplayNameTooLong  = errors.New("display name must not exceed 100 characters")
)
