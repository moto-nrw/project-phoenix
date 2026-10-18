//go:build testhooks

package users

import (
	"context"
	"encoding/json"
	"errors"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// The fixture facets delegate to the same concrete store, including its
// validation, locks, defaults, clock_timestamp stamps, errors and caller tx.
func (r nativeStudentGuardianRepository) CreateGuardianLinkFixture(ctx context.Context, value *peopledirectory.GuardianLinkFixture) error {
	row, err := guardianLinkFixtureRow(value)
	if err != nil {
		return err
	}
	err = r.rawStudentGuardianRepository.Create(ctx, row)
	if value != nil && row != nil {
		copied, copyErr := guardianLinkFixtureValue(row)
		if copyErr != nil {
			return errors.Join(err, copyErr)
		}
		*value = *copied
	}
	return err
}
func guardianLinkFixtureRow(value *peopledirectory.GuardianLinkFixture) (*usersModels.StudentGuardian, error) {
	if value == nil {
		return nil, nil
	}
	var permissions map[string]any
	if len(value.Permissions) > 0 {
		if err := json.Unmarshal(value.Permissions, &permissions); err != nil {
			return nil, err
		}
	}
	row := &usersModels.StudentGuardian{
		StudentID:          value.StudentID,
		GuardianProfileID:  value.GuardianProfileID,
		RelationshipType:   value.RelationshipType,
		GuardianRole:       value.GuardianRole,
		IsPrimary:          value.IsPrimary,
		IsEmergencyContact: value.IsEmergencyContact,
		CanPickup:          value.CanPickup,
		PickupNotes:        value.PickupNotes,
		EmergencyPriority:  value.EmergencyPriority,
		IsPayer:            value.IsPayer,
		Permissions:        permissions,
	}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row, nil
}
func guardianLinkFixtureValue(row *usersModels.StudentGuardian) (*peopledirectory.GuardianLinkFixture, error) {
	if row == nil {
		return nil, nil
	}
	permissions, err := json.Marshal(row.Permissions)
	if err != nil {
		return nil, err
	}
	return &peopledirectory.GuardianLinkFixture{
		ID:                 row.ID,
		TenantID:           row.TenantID,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
		StudentID:          row.StudentID,
		GuardianProfileID:  row.GuardianProfileID,
		RelationshipType:   row.RelationshipType,
		GuardianRole:       row.GuardianRole,
		IsPrimary:          row.IsPrimary,
		IsEmergencyContact: row.IsEmergencyContact,
		CanPickup:          row.CanPickup,
		PickupNotes:        row.PickupNotes,
		EmergencyPriority:  row.EmergencyPriority,
		IsPayer:            row.IsPayer,
		Permissions:        permissions,
	}, nil
}

func (r nativeGuardianPhoneNumberRepository) CreateGuardianPhoneFixture(ctx context.Context, value *peopledirectory.GuardianPhoneFixture) error {
	row := guardianPhoneFixtureRow(value)
	err := r.rawGuardianPhoneNumberRepository.Create(ctx, row)
	if value != nil && row != nil {
		*value = *guardianPhoneFixtureValue(row)
	}
	return err
}
func guardianPhoneFixtureRow(value *peopledirectory.GuardianPhoneFixture) *usersModels.GuardianPhoneNumber {
	if value == nil {
		return nil
	}
	row := &usersModels.GuardianPhoneNumber{
		GuardianProfileID: value.GuardianProfileID,
		PhoneNumber:       value.PhoneNumber,
		PhoneType:         usersModels.PhoneType(value.PhoneType),
		Label:             value.Label,
		IsPrimary:         value.IsPrimary,
		Priority:          value.Priority,
	}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}
func guardianPhoneFixtureValue(row *usersModels.GuardianPhoneNumber) *peopledirectory.GuardianPhoneFixture {
	if row == nil {
		return nil
	}
	return &peopledirectory.GuardianPhoneFixture{
		ID:                row.ID,
		TenantID:          row.TenantID,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
		GuardianProfileID: row.GuardianProfileID,
		PhoneNumber:       row.PhoneNumber,
		PhoneType:         string(row.PhoneType),
		Label:             row.Label,
		IsPrimary:         row.IsPrimary,
		Priority:          row.Priority,
	}
}

func (r *FamilyProtectionEventRepository) CreateFamilyProtectionFixture(ctx context.Context, value *peopledirectory.FamilyProtectionFixture) error {
	row := familyProtectionFixtureRow(value)
	err := r.Create(ctx, row)
	if value != nil && row != nil {
		*value = *familyProtectionFixtureValue(row)
	}
	return err
}
func familyProtectionFixtureRow(value *peopledirectory.FamilyProtectionFixture) *usersModels.FamilyProtectionEvent {
	if value == nil {
		return nil
	}
	row := &usersModels.FamilyProtectionEvent{
		StudentID:      value.StudentID,
		Enabled:        value.Enabled,
		Reason:         value.Reason,
		ActorAccountID: value.ActorAccountID,
	}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}
func familyProtectionFixtureValue(row *usersModels.FamilyProtectionEvent) *peopledirectory.FamilyProtectionFixture {
	if row == nil {
		return nil
	}
	return &peopledirectory.FamilyProtectionFixture{
		ID:             row.ID,
		TenantID:       row.TenantID,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		StudentID:      row.StudentID,
		Enabled:        row.Enabled,
		Reason:         row.Reason,
		ActorAccountID: row.ActorAccountID,
	}
}

func (r *ParentRequestEventRepository) ListParentRequestEventFixtures(ctx context.Context, kind string, id int64) ([]*peopledirectory.ParentRequestEventFixture, error) {
	rows, err := r.ListForRequest(ctx, kind, id)
	if rows == nil {
		return nil, err
	}
	values := make([]*peopledirectory.ParentRequestEventFixture, len(rows))
	for i, row := range rows {
		if row == nil {
			continue
		}
		payload, encodeErr := json.Marshal(row.Payload)
		if encodeErr != nil {
			return nil, encodeErr
		}
		values[i] = &peopledirectory.ParentRequestEventFixture{
			ID:             row.ID,
			TenantID:       row.TenantID,
			CreatedAt:      row.CreatedAt,
			UpdatedAt:      row.UpdatedAt,
			StudentID:      row.StudentID,
			RequestType:    row.RequestType,
			RequestID:      row.RequestID,
			EventType:      row.EventType,
			ActorAccountID: row.ActorAccountID,
			Version:        row.Version,
			Payload:        payload,
		}
	}
	return values, err
}

var _ peopledirectory.GuardianLinkFixtureWriter = nativeStudentGuardianRepository{}
var _ peopledirectory.GuardianPhoneFixtureWriter = nativeGuardianPhoneNumberRepository{}
