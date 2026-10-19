package owners

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func NewParentStudentGuardianReads(source peopledirectory.GuardianRelationshipReads) parentportal.StudentGuardianReads {
	return parentportal.NewStudentGuardianReads(parentportal.GuardianRelationshipCallbacks{
		ByStudent: func(ctx context.Context, id int64) ([]*parentportal.GuardianRelationship, error) {
			return parentGuardianRelationships(source.GuardianRelationshipsForStudent(ctx, id))
		},
		ByProfile: func(ctx context.Context, id int64) ([]*parentportal.GuardianRelationship, error) {
			return parentGuardianRelationships(source.GuardianRelationshipsForProfile(ctx, id))
		},
		Lock: func(ctx context.Context, student, profile int64) (*parentportal.GuardianRelationship, error) {
			value, err := source.GuardianRelationshipForUpdate(ctx, student, profile)
			if err != nil {
				return nil, parentGuardianRelationshipError(err)
			}
			return parentGuardianRelationship(value)
		},
		LinkedChildren: func(ctx context.Context, ids []int64) ([]*parentportal.GuardianLinkedChild, error) {
			rows, err := source.GuardianLinkedChildren(ctx, ids)
			if rows == nil {
				return nil, err
			}
			result := make([]*parentportal.GuardianLinkedChild, len(rows))
			for i, row := range rows {
				if row != nil {
					value := parentportal.GuardianLinkedChild(*row)
					result[i] = &value
				}
			}
			return result, err
		},
		Permission: source.AccountHasStudentPermission,
	})
}

func parentGuardianRelationship(row *peopledirectory.GuardianRelationshipSnapshot) (*parentportal.GuardianRelationship, error) {
	if row == nil {
		return nil, nil
	}
	value := &parentportal.GuardianRelationship{ID: row.ID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, TenantID: row.TenantID, StudentID: row.StudentID, GuardianProfileID: row.GuardianProfileID, RelationshipType: row.RelationshipType, GuardianRole: row.GuardianRole, IsPrimary: row.IsPrimary, IsEmergencyContact: row.IsEmergencyContact, CanPickup: row.CanPickup, PickupNotes: row.PickupNotes, EmergencyPriority: row.EmergencyPriority, IsPayer: row.IsPayer, Permissions: row.Permissions}
	student, err := parentStudentSnapshot(peopledirectory.StudentSnapshotFromState(row.Student))
	if err != nil {
		return nil, err
	}
	return parentportal.WithGuardianRelationshipStudent(value, student), nil
}

func parentGuardianRelationships(rows []*peopledirectory.GuardianRelationshipSnapshot, err error) ([]*parentportal.GuardianRelationship, error) {
	if rows == nil {
		return nil, parentGuardianRelationshipError(err)
	}
	result := make([]*parentportal.GuardianRelationship, len(rows))
	for i, row := range rows {
		value, mapErr := parentGuardianRelationship(row)
		if mapErr != nil {
			return nil, mapErr
		}
		result[i] = value
	}
	return result, parentGuardianRelationshipError(err)
}

func directoryGuardianRelationship(row *parentportal.GuardianRelationship) *peopledirectory.GuardianRelationshipSnapshot {
	if row == nil {
		return nil
	}
	return &peopledirectory.GuardianRelationshipSnapshot{ID: row.ID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, TenantID: row.TenantID, StudentID: row.StudentID, GuardianProfileID: row.GuardianProfileID, RelationshipType: row.RelationshipType, GuardianRole: row.GuardianRole, IsPrimary: row.IsPrimary, IsEmergencyContact: row.IsEmergencyContact, CanPickup: row.CanPickup, PickupNotes: row.PickupNotes, EmergencyPriority: row.EmergencyPriority, IsPayer: row.IsPayer, Permissions: row.Permissions}
}

func parentGuardianRelationshipError(err error) error {
	if errors.Is(err, peopledirectory.ErrStudentGuardianNotFound) {
		return parentportal.WithGuardianRelationshipMissingCause(err)
	}
	return err
}
