package users

import "github.com/moto-nrw/project-phoenix/modules/peopledirectory/contact"

// ErrInvalidPhoneFormat retains the legacy sentinel identity.
var ErrInvalidPhoneFormat = contact.ErrInvalidPhoneFormat

// ValidateOptionalPhone uses the People Directory contact contract.
func ValidateOptionalPhone(value string) error { return contact.ValidateOptionalPhone(value) }
