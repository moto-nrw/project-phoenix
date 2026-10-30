package application

import (
	"errors"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/contact"
)

// A blank optional card is absent. Copy before normalizing so the caller's
// input is not mutated; existing submissions need no second guardian.
func normalizeHolidaySecondGuardian(value *enrollment.HolidayGuardianInput) *enrollment.HolidayGuardianInput {
	if value == nil {
		return nil
	}
	guardian := enrollment.HolidayGuardianInput{
		FirstName: strings.TrimSpace(value.FirstName), LastName: strings.TrimSpace(value.LastName),
		Email: strings.ToLower(strings.TrimSpace(value.Email)), Phone: strings.TrimSpace(value.Phone),
	}
	if guardian.FirstName == "" && guardian.LastName == "" && guardian.Email == "" && guardian.Phone == "" {
		return nil
	}
	return &guardian
}

func validateHolidaySecondGuardian(guardian enrollment.HolidayGuardianInput) validation.Errors {
	errs := validation.Errors{}
	requireHolidayText(errs, "first_name", guardian.FirstName, holidayPersonNameMaxLength)
	requireHolidayText(errs, "last_name", guardian.LastName, holidayPersonNameMaxLength)
	limitHolidayText(errs, "email", guardian.Email, holidayEmailMaxLength)
	limitHolidayText(errs, "phone", guardian.Phone, holidayPhoneMaxLength)
	if contact.ValidateOptionalEmail(guardian.Email) != nil {
		errs["email"] = errors.New("e-mail address is invalid")
	}
	if contact.ValidateOptionalPhone(guardian.Phone) != nil {
		errs["phone"] = errors.New("phone number is invalid")
	}
	return errs
}
