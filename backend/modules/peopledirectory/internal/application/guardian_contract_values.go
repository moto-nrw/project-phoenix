package application

import (
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// The row-level guardian reads (#2663). The application-level guardian
// flows are the guardian provider the module is composed with.

// ObserveGuardianOperation records one provider-backed guardian operation
// next to the module's own reads; the provider runs no directory statements
// of its own, so the statement stats stay zero.

func toPublicGuardians(values []domain.Guardian) []peopledirectory.Guardian {
	result := make([]peopledirectory.Guardian, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.Guardian{
			ID: value.ID, TenantID: value.TenantID, FirstName: value.FirstName, LastName: value.LastName,
			Email: value.Email, AddressStreet: value.AddressStreet, AddressCity: value.AddressCity,
			AddressPostalCode: value.AddressPostalCode, PreferredContactMethod: value.PreferredContactMethod,
			LanguagePreference: value.LanguagePreference, Notes: value.Notes,
			HasAccount: value.HasAccount, AccountID: value.AccountID,
		})
	}
	return result
}

func nonNilGuardianLinks(values []peopledirectory.GuardianLink) []peopledirectory.GuardianLink {
	if values == nil {
		return []peopledirectory.GuardianLink{}
	}
	return values
}
