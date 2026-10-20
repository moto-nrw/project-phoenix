package application

import (
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func toDomainGuardianContact(record peopledirectory.GuardianContactRecord) domain.GuardianContact {
	return domain.GuardianContact{
		FirstName: record.FirstName, LastName: record.LastName, Email: record.Email,
		AddressStreet: record.AddressStreet, AddressCity: record.AddressCity, PostalCode: record.AddressPostalCode,
		PreferredContactMethod: record.PreferredContactMethod, LanguagePreference: record.LanguagePreference,
	}
}

// mapGuardianPortalError translates the portal commands' domain outcomes;
// the e-mail conflict keeps the database error in its chain.
func mapGuardianPortalError(err error) error {
	var invalid *domain.GuardianInvalidError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &invalid):
		return &peopledirectory.InvalidGuardianError{Reason: invalid.Reason}
	case errors.Is(err, domain.ErrGuardianEmailTaken):
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianEmailTaken, err)
	case errors.Is(err, domain.ErrGuardianNotFound):
		return peopledirectory.ErrGuardianNotFound
	case errors.Is(err, domain.ErrGuardianPhoneNotFound):
		return peopledirectory.ErrGuardianPhoneNotFound
	default:
		return MapError(err)
	}
}
