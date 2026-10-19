package users

import (
	"context"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	users "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
)

// The existing directory store serves the native read facade itself. Each
// operation retains its original query, tenant context, locks and errors.
var _ directory.GuardianProfileReads = (*guardianProfileRepository)(nil)

func (r *guardianProfileRepository) GuardianProfileByID(ctx context.Context, id int64) (*directory.GuardianProfileSnapshot, error) {
	value, err := r.FindByID(ctx, id)
	return NativeGuardianProfile(value), err
}
func (r *guardianProfileRepository) GuardianProfilesByIDs(ctx context.Context, ids []int64) (map[int64]*directory.GuardianProfileSnapshot, error) {
	value, err := r.FindByIDs(ctx, ids)
	if value == nil {
		return nil, err
	}
	result := make(map[int64]*directory.GuardianProfileSnapshot, len(value))
	for id, row := range value {
		result[id] = NativeGuardianProfile(row)
	}
	return result, err
}
func (r *guardianProfileRepository) GuardianProfileByEmail(ctx context.Context, email string) (*directory.GuardianProfileSnapshot, error) {
	value, err := r.FindByEmail(ctx, email)
	return NativeGuardianProfile(value), err
}
func (r *guardianProfileRepository) GuardianProfileByAccount(ctx context.Context, id int64) (*directory.GuardianProfileSnapshot, error) {
	value, err := r.FindByAccountID(ctx, id)
	return NativeGuardianProfile(value), err
}
func (r *guardianProfileRepository) ActiveGuardianPortalProfiles(ctx context.Context, ids []int64) (map[int64]*directory.GuardianProfileSnapshot, error) {
	value, err := r.FindActivePortalProfilesByIDs(ctx, ids)
	if value == nil {
		return nil, err
	}
	result := make(map[int64]*directory.GuardianProfileSnapshot, len(value))
	for id, row := range value {
		result[id] = NativeGuardianProfile(row)
	}
	return result, err
}
func (r *guardianProfileRepository) LockGuardianProfile(ctx context.Context, id int64) error {
	return r.LockByIDForUpdate(ctx, id)
}
func (*guardianProfileRepository) NormalizeGuardianProfile(value *directory.GuardianProfileSnapshot) error {
	row := GuardianProfileRow(value)
	err := row.Validate()
	*value = *NativeGuardianProfile(row)
	return err
}
func NativeGuardianProfile(row *users.GuardianProfile) *directory.GuardianProfileSnapshot {
	if row == nil {
		return nil
	}
	out := &directory.GuardianProfileSnapshot{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, FirstName: row.FirstName, LastName: row.LastName, Email: row.Email, AddressStreet: row.AddressStreet, AddressCity: row.AddressCity, AddressPostalCode: row.AddressPostalCode, AccountID: row.AccountID, HasAccount: row.HasAccount, PreferredContactMethod: row.PreferredContactMethod, LanguagePreference: row.LanguagePreference, PortalLocale: row.PortalLocale, Notes: row.Notes}
	if row.PhoneNumbers != nil {
		out.PhoneNumbers = make([]*directory.GuardianProfilePhoneSnapshot, len(row.PhoneNumbers))
		for i, p := range row.PhoneNumbers {
			if p != nil {
				out.PhoneNumbers[i] = &directory.GuardianProfilePhoneSnapshot{ID: p.ID, TenantID: p.TenantID, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt, GuardianProfileID: p.GuardianProfileID, PhoneNumber: p.PhoneNumber, PhoneType: string(p.PhoneType), Label: p.Label, IsPrimary: p.IsPrimary, Priority: p.Priority}
			}
		}
	}
	return out
}
func GuardianProfileRow(value *directory.GuardianProfileSnapshot) *users.GuardianProfile {
	row := &users.GuardianProfile{}
	row.ID = value.ID
	row.TenantID = value.TenantID
	row.CreatedAt = value.CreatedAt
	row.UpdatedAt = value.UpdatedAt
	row.FirstName = value.FirstName
	row.LastName = value.LastName
	row.Email = value.Email
	row.AddressStreet = value.AddressStreet
	row.AddressCity = value.AddressCity
	row.AddressPostalCode = value.AddressPostalCode
	row.AccountID = value.AccountID
	row.HasAccount = value.HasAccount
	row.PreferredContactMethod = value.PreferredContactMethod
	row.LanguagePreference = value.LanguagePreference
	row.PortalLocale = value.PortalLocale
	row.Notes = value.Notes
	if value.PhoneNumbers != nil {
		row.PhoneNumbers = make([]*users.GuardianPhoneNumber, len(value.PhoneNumbers))
		for i, p := range value.PhoneNumbers {
			if p != nil {
				phone := &users.GuardianPhoneNumber{}
				phone.ID = p.ID
				phone.TenantID = p.TenantID
				phone.CreatedAt = p.CreatedAt
				phone.UpdatedAt = p.UpdatedAt
				phone.GuardianProfileID = p.GuardianProfileID
				phone.PhoneNumber = p.PhoneNumber
				phone.PhoneType = users.PhoneType(p.PhoneType)
				phone.Label = p.Label
				phone.IsPrimary = p.IsPrimary
				phone.Priority = p.Priority
				row.PhoneNumbers[i] = phone
			}
		}
	}
	return row
}
