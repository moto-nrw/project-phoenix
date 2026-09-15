package peopledirectory

import "github.com/moto-nrw/project-phoenix/modules/peopledirectory/contact"

// Person gender values are shared by the directory and import contracts.
const (
	GenderFemale    = "female"
	GenderMale      = "male"
	GenderDiverse   = "diverse"
	PhoneTypeMobile = "mobile"
	PhoneTypeHome   = "home"
	PhoneTypeWork   = "work"
	PhoneTypeOther  = "other"
)

// ValidateOptionalPhone uses the same format validation as directory writes.
func ValidateOptionalPhone(value string) error { return contact.ValidateOptionalPhone(value) }

func IsValidEmailFormat(value string) bool { return contact.IsValidEmailFormat(value) }
