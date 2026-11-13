package application

import (
	"errors"
	"regexp"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// The postal address of the parent who registers (#5625). The version of
// the form the registration comes with decides whether it is asked: not
// asked refuses an address, optional takes all three parts or none,
// required needs all three. A German postal code has five digits; there is
// no address abroad and no address supplement.

// holidayPostalCode matches a German postal code.
var holidayPostalCode = regexp.MustCompile(`^[0-9]{5}$`)

// normalizeHolidayAddress trims the parts; a blank address is absent. It
// copies, so the caller's input is not changed.
func normalizeHolidayAddress(value *enrollment.HolidayAddress) *enrollment.HolidayAddress {
	if value == nil {
		return nil
	}
	address := enrollment.HolidayAddress{
		Street: strings.TrimSpace(value.Street), PostalCode: strings.TrimSpace(value.PostalCode),
		City: strings.TrimSpace(value.City),
	}
	if address.Street == "" && address.PostalCode == "" && address.City == "" {
		return nil
	}
	return &address
}

// validateHolidayAddress checks the address against the building block of
// the form. The errors are keyed by the part, as "guardian_address.street".
func validateHolidayAddress(requirement enrollment.HolidayAddressRequirement, address *enrollment.HolidayAddress) validation.Errors {
	errs := validation.Errors{}
	switch {
	case address == nil && requirement == enrollment.HolidayAddressRequired:
		for _, part := range []string{"street", "postal_code", "city"} {
			errs[part] = errors.New("is required")
		}
		return errs
	case address == nil:
		return nil
	case requirement != enrollment.HolidayAddressOptional && requirement != enrollment.HolidayAddressRequired:
		for part, value := range map[string]string{"street": address.Street, "postal_code": address.PostalCode, "city": address.City} {
			if value != "" {
				errs[part] = errors.New("the form does not ask for an address")
			}
		}
		return errs
	}
	requireHolidayText(errs, "street", address.Street, holidayPersonNameMaxLength)
	requireHolidayText(errs, "city", address.City, holidayPersonNameMaxLength)
	if address.PostalCode == "" {
		errs["postal_code"] = errors.New("is required")
	} else if !holidayPostalCode.MatchString(address.PostalCode) {
		errs["postal_code"] = errors.New("must be five digits")
	}
	return errs
}
