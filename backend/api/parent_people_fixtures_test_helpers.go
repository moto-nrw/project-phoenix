//go:build testhooks

package api

import (
	"context"
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/tenant"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// These test-only bindings retain the exact stores the fixture graph already
// holds. Native fixture values do not change the production reader DTO debt.
func (f *PeopleRepositorySuiteFactory) CreateGuardianLinkFixture(ctx context.Context, value *peopledirectory.GuardianLinkFixture) error {
	return f.StudentGuardian.(peopledirectory.GuardianLinkFixtureWriter).CreateGuardianLinkFixture(ctx, value)
}
func (f *PeopleRepositorySuiteFactory) CreateGuardianPhoneFixture(ctx context.Context, value *peopledirectory.GuardianPhoneFixture) error {
	return tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		if value == nil {
			return f.GuardianPhoneNumber.Create(txCtx, nil)
		}
		phone := peopledirectory.GuardianProfilePhoneSnapshot(*value)
		err := f.GuardianPhoneNumber.Create(txCtx, &phone)
		*value = peopledirectory.GuardianPhoneFixture(phone)
		return err
	})
}
func (f *PeopleRepositorySuiteFactory) CreateFamilyProtectionFixture(ctx context.Context, value *peopledirectory.FamilyProtectionFixture) error {
	return f.FamilyProtection.(peopledirectory.FamilyProtectionFixtureWriter).CreateFamilyProtectionFixture(ctx, value)
}

// ParentRequestEventFixtureReads observes the same append-only ledger through
// native values; workflow construction still receives its original writer.
type ParentRequestEventFixtureReads struct {
	source peopledirectory.ParentRequestEventFixtureSource
}

func NewParentRequestEventFixtureReads(source peopledirectory.ParentRequestEvents) ParentRequestEventFixtureReads {
	return ParentRequestEventFixtureReads{source: source.(peopledirectory.ParentRequestEventFixtureSource)}
}
func (r ParentRequestEventFixtureReads) ListForRequest(ctx context.Context, kind string, id int64) ([]*ParentRequestEventFixture, error) {
	values, err := r.source.ListParentRequestEventFixtures(ctx, kind, id)
	if values == nil {
		return nil, err
	}
	rows := make([]*ParentRequestEventFixture, len(values))
	for i, value := range values {
		if value == nil {
			continue
		}
		var payload map[string]any
		if decodeErr := json.Unmarshal(value.Payload, &payload); decodeErr != nil {
			return nil, decodeErr
		}
		rows[i] = &ParentRequestEventFixture{ParentRequestEventFixture: *value, Payload: payload}
	}
	return rows, err
}

// NewParentGuardianProfileReadFixture retains the locale stub's complete
// profile values and inert operations through the native workflow callbacks.
func NewParentGuardianProfileReadFixture(find func(context.Context, int64) (*peopledirectory.GuardianProfileFixture, error)) parentportal.GuardianProfiles {
	return parentportal.NewGuardianProfileReads(parentportal.GuardianProfileReadCallbacks{
		FindByAccountID: func(ctx context.Context, id int64) (*parentportal.GuardianProfile, error) {
			value, err := find(ctx, id)
			if value == nil {
				return nil, err
			}
			profile := &peopledirectory.GuardianProfileSnapshot{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, FirstName: value.FirstName, LastName: value.LastName, Email: value.Email, AddressStreet: value.AddressStreet, AddressCity: value.AddressCity, AddressPostalCode: value.AddressPostalCode, AccountID: value.AccountID, HasAccount: value.HasAccount, PreferredContactMethod: value.PreferredContactMethod, LanguagePreference: value.LanguagePreference, PortalLocale: value.PortalLocale, Notes: value.Notes}
			if value.PhoneNumbers != nil {
				profile.PhoneNumbers = make([]*peopledirectory.GuardianProfilePhoneSnapshot, len(value.PhoneNumbers))
				for i, p := range value.PhoneNumbers {
					if p != nil {
						phone := peopledirectory.GuardianProfilePhoneSnapshot(*p)
						profile.PhoneNumbers[i] = &phone
					}
				}
			}
			return owners.ParentGuardianProfile(profile), err
		},
		FindByID:                      func(context.Context, int64) (*parentportal.GuardianProfile, error) { return nil, nil },
		FindByIDs:                     func(context.Context, []int64) (map[int64]*parentportal.GuardianProfile, error) { return nil, nil },
		FindByEmail:                   func(context.Context, string) (*parentportal.GuardianProfile, error) { return nil, nil },
		FindActivePortalProfilesByIDs: func(context.Context, []int64) (map[int64]*parentportal.GuardianProfile, error) { return nil, nil },
		LockByIDForUpdate:             func(context.Context, int64) error { return nil },
		Normalize: func(value *parentportal.GuardianProfile) error {
			row := &peopledirectory.GuardianProfileSnapshot{FirstName: value.FirstName, LastName: value.LastName, Email: value.Email, PreferredContactMethod: value.PreferredContactMethod}
			err := row.Validate()
			value.FirstName, value.LastName, value.Email = row.FirstName, row.LastName, row.Email
			return err
		},
	})
}

// ParentPersistedFixtureProtocol binds the composed owner scalar facet.
func ParentPersistedFixtureProtocol(source peopledirectory.Capability) peopledirectory.ParentFixtureProtocol {
	return source.(peopledirectory.ParentFixtureProtocols).ParentFixtureProtocol()
}

// ParentRequestEventFixture is the root test view of the owner's typed JSON
// payload. It retains all metadata while decoding the same persisted JSON for
// the existing ledger assertions.
type ParentRequestEventFixture struct {
	peopledirectory.ParentRequestEventFixture
	Payload map[string]any
}

// ParentGuardianRoleFixtureJSON serializes the same canonical role preset for
// the native owner's stored-JSON permission input. Presets contain JSON values.
func ParentGuardianRoleFixtureJSON(role string) (string, json.RawMessage) {
	canonical, permissions := ParentGuardianRoleFixture(role)
	encoded, err := json.Marshal(permissions)
	if err != nil {
		panic(err)
	}
	return canonical, encoded
}
