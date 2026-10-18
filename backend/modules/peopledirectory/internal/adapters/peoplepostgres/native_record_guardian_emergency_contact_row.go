package users

import (
	models "github.com/moto-nrw/project-phoenix/models/users"
	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

func NativeGuardianEmergencyContactRow(value *models.GuardianEmergencyContactRow) *directory.GuardianEmergencyContactRow {
	if value == nil {
		return nil
	}
	result := &directory.GuardianEmergencyContactRow{}
	result.StudentID = value.StudentID
	result.GuardianProfileID = value.GuardianProfileID
	result.FirstName = value.FirstName
	result.LastName = value.LastName
	result.Email = value.Email
	result.PhoneNumber = value.PhoneNumber
	result.RelationshipType = value.RelationshipType
	result.PickupNotes = value.PickupNotes
	result.CanPickup = value.CanPickup
	result.IsEmergencyContact = value.IsEmergencyContact
	return result
}
