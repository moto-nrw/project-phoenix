package users

import (
	"context"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	models "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
)

func nativeFamilyProtectionEvent(v *models.FamilyProtectionEvent) *directory.GuardianFamilyProtectionRecord {
	if v == nil {
		return nil
	}
	return &directory.GuardianFamilyProtectionRecord{ID: v.ID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, TenantID: v.TenantID, StudentID: v.StudentID, Enabled: v.Enabled, Reason: v.Reason, ActorAccountID: v.ActorAccountID}
}
func nativeParentRequestShareEvent(v *models.ParentRequestShareEvent) *directory.ParentRequestShareRecord {
	if v == nil {
		return nil
	}
	return &directory.ParentRequestShareRecord{ID: v.ID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, TenantID: v.TenantID, StudentID: v.StudentID, RequestType: v.RequestType, RequestID: v.RequestID, AuthorAccountID: v.AuthorAccountID, RecipientAccountIDs: v.RecipientAccountIDs}
}
func retainedParentRequestShareRecord(v *directory.ParentRequestShareRecord) *models.ParentRequestShareEvent {
	if v == nil {
		return nil
	}
	row := &models.ParentRequestShareEvent{StudentID: v.StudentID, RequestType: v.RequestType, RequestID: v.RequestID, AuthorAccountID: v.AuthorAccountID, RecipientAccountIDs: v.RecipientAccountIDs}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = v.ID, v.TenantID, v.CreatedAt, v.UpdatedAt
	return row
}
func (r *FamilyProtectionEventRepository) GuardianFamilyProtectionRecords(ctx context.Context, ids []int64) (map[int64]*directory.GuardianFamilyProtectionRecord, error) {
	rows, err := r.CurrentForStudents(ctx, ids)
	if rows == nil {
		return nil, err
	}
	out := make(map[int64]*directory.GuardianFamilyProtectionRecord, len(rows))
	for id, row := range rows {
		out[id] = nativeFamilyProtectionEvent(row)
	}
	return out, err
}
func (r *ParentRequestShareEventRepository) AppendParentRequestShare(ctx context.Context, value *directory.ParentRequestShareRecord) error {
	row := retainedParentRequestShareRecord(value)
	err := r.Create(ctx, row)
	if value != nil {
		*value = *nativeParentRequestShareEvent(row)
	}
	return err
}
func (r *ParentRequestShareEventRepository) ParentRequestSharesForStudent(ctx context.Context, id int64) ([]*directory.ParentRequestShareRecord, error) {
	rows, err := r.CurrentForStudent(ctx, id)
	if rows == nil {
		return nil, err
	}
	out := make([]*directory.ParentRequestShareRecord, len(rows))
	for i, row := range rows {
		out[i] = nativeParentRequestShareEvent(row)
	}
	return out, err
}
