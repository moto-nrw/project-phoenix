package compose

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// The row-level guardian reads (#2663). The application-level guardian
// flows are bound onto the module by the composition root through
// Module.BindGuardianProvider once the owner's legacy service exists.

func (e engine) ListGuardianLinksByAccount(ctx context.Context, accountID int64) ([]peopledirectory.GuardianLink, error) {
	values, err := e.guardians.ListLinksByAccount(ctx, accountID)
	return toPublicLinks(values), mapError(err)
}

func (e engine) ListGuardiansByAccounts(ctx context.Context, accountIDs []int64) ([]peopledirectory.Guardian, error) {
	values, err := e.guardians.ListByAccounts(ctx, accountIDs)
	return toPublicGuardians(values), mapError(err)
}

func (e engine) ListGuardiansByIDs(ctx context.Context, ids []int64) ([]peopledirectory.Guardian, error) {
	values, err := e.guardians.ListByIDs(ctx, ids)
	return toPublicGuardians(values), mapError(err)
}

// ObserveGuardianOperation records one provider-backed guardian operation
// next to the module's own reads; the provider runs no directory statements
// of its own, so the statement stats stay zero.
func (e engine) ObserveGuardianOperation(operation string, duration time.Duration, err error) {
	e.observe(Observation{Operation: operation, Duration: duration, Err: err})
}

func (e engine) CountGuardianLinks(ctx context.Context, guardianIDs []int64) (map[int64]int, error) {
	values, err := e.guardians.CountLinks(ctx, guardianIDs)
	if values == nil {
		values = map[int64]int{}
	}
	return values, mapError(err)
}

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

func toPublicLinks(values []domain.GuardianLink) []peopledirectory.GuardianLink {
	result := make([]peopledirectory.GuardianLink, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.GuardianLink{
			ID: value.ID, TenantID: value.TenantID, StudentID: value.StudentID, GuardianProfileID: value.GuardianProfileID,
			RelationshipType: value.RelationshipType, GuardianRole: value.GuardianRole,
			IsPrimary: value.IsPrimary, IsEmergencyContact: value.IsEmergencyContact, CanPickup: value.CanPickup,
			PickupNotes: value.PickupNotes, EmergencyPriority: value.EmergencyPriority, IsPayer: value.IsPayer,
			Permissions: value.Permissions,
		})
	}
	return result
}
