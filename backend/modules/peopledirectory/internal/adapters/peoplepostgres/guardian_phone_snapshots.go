package users

import (
	"context"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	models "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
)

var _ directory.GuardianPhoneReads = (*guardianPhoneNumberRepository)(nil)

func (r *guardianPhoneNumberRepository) GuardianPhonesForProfile(ctx context.Context, id int64) ([]*directory.GuardianProfilePhoneSnapshot, error) {
	rows, err := r.FindByGuardianID(ctx, id)
	return guardianPhoneSnapshots(rows), err
}
func (r *guardianPhoneNumberRepository) GuardianPhonesForProfiles(ctx context.Context, ids []int64) (map[int64][]*directory.GuardianProfilePhoneSnapshot, error) {
	rows, err := r.FindByGuardianIDs(ctx, ids)
	if rows == nil {
		return nil, err
	}
	result := make(map[int64][]*directory.GuardianProfilePhoneSnapshot, len(rows))
	for id, phones := range rows {
		result[id] = guardianPhoneSnapshots(phones)
	}
	return result, err
}
func (*guardianPhoneNumberRepository) NormalizeGuardianPhone(value *directory.GuardianProfilePhoneSnapshot) error {
	row := &models.GuardianPhoneNumber{}
	row.ID = value.ID
	row.TenantID = value.TenantID
	row.CreatedAt = value.CreatedAt
	row.UpdatedAt = value.UpdatedAt
	row.GuardianProfileID = value.GuardianProfileID
	row.PhoneNumber = value.PhoneNumber
	row.PhoneType = models.PhoneType(value.PhoneType)
	row.Label = value.Label
	row.IsPrimary = value.IsPrimary
	row.Priority = value.Priority

	err := row.Validate()
	*value = *NativeGuardianPhone(row)
	return err
}
func (*guardianPhoneNumberRepository) GuardianPhoneTypeOrMobile(input string) string {
	kind := models.PhoneType(input)
	if models.ValidPhoneTypes[kind] {
		return input
	}
	return string(models.PhoneTypeMobile)
}
func guardianPhoneSnapshots(rows []*models.GuardianPhoneNumber) []*directory.GuardianProfilePhoneSnapshot {
	if rows == nil {
		return nil
	}
	result := make([]*directory.GuardianProfilePhoneSnapshot, len(rows))
	for i, row := range rows {
		result[i] = NativeGuardianPhone(row)
	}
	return result
}
func NativeGuardianPhone(row *models.GuardianPhoneNumber) *directory.GuardianProfilePhoneSnapshot {
	if row == nil {
		return nil
	}
	return &directory.GuardianProfilePhoneSnapshot{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, GuardianProfileID: row.GuardianProfileID, PhoneNumber: row.PhoneNumber, PhoneType: string(row.PhoneType), Label: row.Label, IsPrimary: row.IsPrimary, Priority: row.Priority}
}
