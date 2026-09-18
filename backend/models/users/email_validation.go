package users

import "github.com/moto-nrw/project-phoenix/modules/peopledirectory/contact"

// ErrInvalidEmailFormat retains the legacy sentinel identity.
var ErrInvalidEmailFormat = contact.ErrInvalidEmailFormat

// ValidateOptionalEmail uses the People Directory contact contract.
func ValidateOptionalEmail(value string) error { return contact.ValidateOptionalEmail(value) }
