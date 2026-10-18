package users

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	models "github.com/moto-nrw/project-phoenix/models/users"
	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

func NativeGuardianRelationshipValue(value *models.StudentGuardian) *directory.GuardianRelationshipSnapshot {
	if value == nil {
		return nil
	}
	result := &directory.GuardianRelationshipSnapshot{}
	result.ID = value.ID
	result.CreatedAt = value.CreatedAt
	result.UpdatedAt = value.UpdatedAt
	result.TenantID = value.TenantID
	result.StudentID = value.StudentID
	result.GuardianProfileID = value.GuardianProfileID
	result.RelationshipType = value.RelationshipType
	result.GuardianRole = value.GuardianRole
	result.IsPrimary = value.IsPrimary
	result.IsEmergencyContact = value.IsEmergencyContact
	result.CanPickup = value.CanPickup
	result.PickupNotes = value.PickupNotes
	result.EmergencyPriority = value.EmergencyPriority
	result.IsPayer = value.IsPayer
	if value.Permissions != nil {
		result.Permissions, _ = json.Marshal(value.Permissions)
	}
	result.Student = NativeStudentState(value.Student)
	return result
}

func GuardianRelationshipRow(value *directory.GuardianRelationshipSnapshot) (*models.StudentGuardian, error) {
	if value == nil {
		return nil, nil
	}
	result := &models.StudentGuardian{}
	result.ID = value.ID
	result.CreatedAt = value.CreatedAt
	result.UpdatedAt = value.UpdatedAt
	result.TenantID = value.TenantID
	result.StudentID = value.StudentID
	result.GuardianProfileID = value.GuardianProfileID
	result.RelationshipType = value.RelationshipType
	result.GuardianRole = value.GuardianRole
	result.IsPrimary = value.IsPrimary
	result.IsEmergencyContact = value.IsEmergencyContact
	result.CanPickup = value.CanPickup
	result.PickupNotes = value.PickupNotes
	result.EmergencyPriority = value.EmergencyPriority
	result.IsPayer = value.IsPayer
	if value.Permissions != nil {
		decoder := json.NewDecoder(bytes.NewReader(value.Permissions))
		decoder.UseNumber()
		if err := decoder.Decode(&result.Permissions); err != nil {
			return nil, err
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("guardian permissions contain trailing JSON")
		}
	}
	result.Student = StudentStateRow(value.Student)
	return result, nil
}
