package users

import (
	"context"
	"encoding/json"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	models "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
)

func nativeGuardianRelationship(row *models.StudentGuardian) (*directory.GuardianRelationshipSnapshot, error) {
	if row == nil {
		return nil, nil
	}
	value := &directory.GuardianRelationshipSnapshot{ID: row.ID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, TenantID: row.TenantID, StudentID: row.StudentID, GuardianProfileID: row.GuardianProfileID, RelationshipType: row.RelationshipType, GuardianRole: row.GuardianRole, IsPrimary: row.IsPrimary, IsEmergencyContact: row.IsEmergencyContact, CanPickup: row.CanPickup, PickupNotes: row.PickupNotes, EmergencyPriority: row.EmergencyPriority, IsPayer: row.IsPayer}
	if row.Permissions != nil {
		raw, err := json.Marshal(row.Permissions)
		if err != nil {
			return nil, err
		}
		value.Permissions = raw
	}
	value.Student = NativeStudentState(row.Student)
	return value, nil
}
func nativeGuardianRelationships(rows []*models.StudentGuardian, err error) ([]*directory.GuardianRelationshipSnapshot, error) {
	if rows == nil {
		return nil, err
	}
	result := make([]*directory.GuardianRelationshipSnapshot, len(rows))
	for i, row := range rows {
		value, mapErr := nativeGuardianRelationship(row)
		if mapErr != nil {
			return nil, mapErr
		}
		result[i] = value
	}
	return result, err
}
func (r *GuardianRelationshipRepository) GuardianRelationshipsForStudent(ctx context.Context, id int64) ([]*directory.GuardianRelationshipSnapshot, error) {
	return nativeGuardianRelationships(r.FindByStudentID(ctx, id))
}
func (r *GuardianRelationshipRepository) GuardianRelationshipsForProfile(ctx context.Context, id int64) ([]*directory.GuardianRelationshipSnapshot, error) {
	return nativeGuardianRelationships(r.FindByGuardianProfileID(ctx, id))
}
func (r *GuardianRelationshipRepository) GuardianRelationshipForUpdate(ctx context.Context, studentID, profileID int64) (*directory.GuardianRelationshipSnapshot, error) {
	row, err := r.FindByStudentAndGuardianForUpdate(ctx, studentID, profileID)
	if err != nil {
		return nil, err
	}
	return nativeGuardianRelationship(row)
}
func (r *GuardianRelationshipRepository) GuardianLinkedChildren(ctx context.Context, ids []int64) ([]*directory.GuardianLinkedChildSnapshot, error) {
	rows, err := r.ListLinkedChildrenForGuardians(ctx, ids)
	if rows == nil {
		return nil, err
	}
	result := make([]*directory.GuardianLinkedChildSnapshot, len(rows))
	for i, row := range rows {
		if row != nil {
			result[i] = &directory.GuardianLinkedChildSnapshot{GuardianProfileID: row.GuardianProfileID, StudentID: row.StudentID, FirstName: row.FirstName, LastName: row.LastName}
		}
	}
	return result, err
}
