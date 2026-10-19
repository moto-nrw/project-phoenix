package owners

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// BindParentGuardianProfiles binds the workflow's own snapshot callbacks to
// People Directory's public, complete-profile facet.
func BindParentGuardianProfiles(source peopledirectory.GuardianProfileReads) parentportal.GuardianProfiles {
	c := parentportal.GuardianProfileReadCallbacks{LockByIDForUpdate: func(ctx context.Context, id int64) error {
		return parentGuardianProfileError(source.LockGuardianProfile(ctx, id))
	}}
	c.FindByID = func(ctx context.Context, id int64) (*parentportal.GuardianProfile, error) {
		value, err := source.GuardianProfileByID(ctx, id)
		return ParentGuardianProfile(value), parentGuardianProfileError(err)
	}
	c.FindByIDs = func(ctx context.Context, ids []int64) (map[int64]*parentportal.GuardianProfile, error) {
		value, err := source.GuardianProfilesByIDs(ctx, ids)
		if value == nil {
			return nil, parentGuardianProfileError(err)
		}
		result := make(map[int64]*parentportal.GuardianProfile, len(value))
		for id, row := range value {
			result[id] = ParentGuardianProfile(row)
		}
		return result, parentGuardianProfileError(err)
	}
	c.FindByEmail = func(ctx context.Context, email string) (*parentportal.GuardianProfile, error) {
		value, err := source.GuardianProfileByEmail(ctx, email)
		return ParentGuardianProfile(value), parentGuardianProfileError(err)
	}
	c.FindByAccountID = func(ctx context.Context, id int64) (*parentportal.GuardianProfile, error) {
		value, err := source.GuardianProfileByAccount(ctx, id)
		return ParentGuardianProfile(value), parentGuardianProfileError(err)
	}
	c.FindActivePortalProfilesByIDs = func(ctx context.Context, ids []int64) (map[int64]*parentportal.GuardianProfile, error) {
		value, err := source.ActiveGuardianPortalProfiles(ctx, ids)
		if value == nil {
			return nil, parentGuardianProfileError(err)
		}
		result := make(map[int64]*parentportal.GuardianProfile, len(value))
		for id, row := range value {
			result[id] = ParentGuardianProfile(row)
		}
		return result, parentGuardianProfileError(err)
	}
	c.Normalize = func(value *parentportal.GuardianProfile) error {
		row := directoryGuardianProfile(value)
		err := source.NormalizeGuardianProfile(row)
		*value = *ParentGuardianProfile(row)
		return err
	}
	return parentportal.NewGuardianProfileReads(c)
}

func ParentGuardianProfile(row *peopledirectory.GuardianProfileSnapshot) *parentportal.GuardianProfile {
	if row == nil {
		return nil
	}
	out := &parentportal.GuardianProfile{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, FirstName: row.FirstName, LastName: row.LastName, Email: row.Email, AddressStreet: row.AddressStreet, AddressCity: row.AddressCity, AddressPostalCode: row.AddressPostalCode, AccountID: row.AccountID, HasAccount: row.HasAccount, PreferredContactMethod: row.PreferredContactMethod, LanguagePreference: row.LanguagePreference, PortalLocale: row.PortalLocale, Notes: row.Notes}
	var phones []*parentportal.GuardianProfilePhone
	if row.PhoneNumbers != nil {
		phones = make([]*parentportal.GuardianProfilePhone, len(row.PhoneNumbers))
		for i, p := range row.PhoneNumbers {
			if p != nil {
				value := parentportal.GuardianProfilePhone(*p)
				phones[i] = &value
			}
		}
	}
	return parentportal.WithGuardianProfilePhones(out, phones)
}

func directoryGuardianProfile(value *parentportal.GuardianProfile) *peopledirectory.GuardianProfileSnapshot {
	row := &peopledirectory.GuardianProfileSnapshot{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, FirstName: value.FirstName, LastName: value.LastName, Email: value.Email, AddressStreet: value.AddressStreet, AddressCity: value.AddressCity, AddressPostalCode: value.AddressPostalCode, AccountID: value.AccountID, HasAccount: value.HasAccount, PreferredContactMethod: value.PreferredContactMethod, LanguagePreference: value.LanguagePreference, PortalLocale: value.PortalLocale, Notes: value.Notes}
	if value.PhoneNumbers != nil {
		row.PhoneNumbers = make([]*peopledirectory.GuardianProfilePhoneSnapshot, len(value.PhoneNumbers))
		for i, p := range value.PhoneNumbers {
			if p != nil {
				phone := peopledirectory.GuardianProfilePhoneSnapshot(*p)
				row.PhoneNumbers[i] = &phone
			}
		}
	}
	return row
}

// NewParentGuardianProfileReads constructs the owner facade over retained reads.
func NewParentGuardianProfileReads(source peopledirectory.GuardianProfileReads) parentportal.GuardianProfiles {
	return BindParentGuardianProfiles(source)
}

func parentGuardianProfileError(err error) error {
	if errors.Is(err, peopledirectory.ErrGuardianProfileNotFound) {
		return parentportal.WithGuardianProfileMissingCause(err)
	}
	return err
}
