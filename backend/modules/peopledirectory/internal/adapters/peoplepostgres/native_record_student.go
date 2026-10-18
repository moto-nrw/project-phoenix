package users

import (
	models "github.com/moto-nrw/project-phoenix/models/users"
	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

func NativeStudentState(value *models.Student) *directory.StudentState {
	if value == nil {
		return nil
	}
	result := &directory.StudentState{}
	result.ID = value.ID
	result.CreatedAt = value.CreatedAt
	result.UpdatedAt = value.UpdatedAt
	result.TenantID = value.TenantID
	result.PersonID = value.PersonID
	result.SchoolClass = value.SchoolClass
	result.GroupID = value.GroupID
	result.AddressStreet = value.AddressStreet
	result.AddressCity = value.AddressCity
	result.AddressPostalCode = value.AddressPostalCode
	result.ExtraInfo = value.ExtraInfo
	result.SupervisorNotes = value.SupervisorNotes
	result.HealthInfo = value.HealthInfo
	result.PickupStatus = value.PickupStatus
	result.DepartureDays = value.DepartureDays
	result.AllowedDepartureModes = value.AllowedDepartureModes
	result.DepartureCompanionNote = value.DepartureCompanionNote
	result.DepartureCompanionDays = value.DepartureCompanionDays
	if value.DepartureBaseline != nil {
		b := value.DepartureBaseline
		result.DepartureBaseline = &directory.StudentPlan{AllowedDepartureModes: b.AllowedDepartureModes, DepartureDays: b.DepartureDays, PickupDays: b.PickupDays, BusDays: b.BusDays}
	}
	result.PickupDays = value.PickupDays
	result.BusDays = value.BusDays
	result.Sick = value.Sick
	result.SickSince = value.SickSince
	result.Excused = value.Excused
	result.ExcusedSince = value.ExcusedSince
	result.Status = string(value.Status)
	result.EnrolledFrom = value.EnrolledFrom
	result.EnrolledUntil = value.EnrolledUntil
	result.PhotoPath = value.PhotoPath
	result.PhotoConsentGivenAt = value.PhotoConsentGivenAt
	result.PhotoConsentGivenBy = value.PhotoConsentGivenBy
	result.AGBAcceptedAt = value.AGBAcceptedAt
	result.DataProcessingAcceptedAt = value.DataProcessingAcceptedAt
	result.EmailContactAcceptedAt = value.EmailContactAcceptedAt
	result.Person = NativeDirectoryPerson(value.Person)
	return result
}

func StudentStateRow(value *directory.StudentState) *models.Student {
	if value == nil {
		return nil
	}
	result := &models.Student{}
	result.ID = value.ID
	result.CreatedAt = value.CreatedAt
	result.UpdatedAt = value.UpdatedAt
	result.TenantID = value.TenantID
	result.PersonID = value.PersonID
	result.SchoolClass = value.SchoolClass
	result.GroupID = value.GroupID
	result.AddressStreet = value.AddressStreet
	result.AddressCity = value.AddressCity
	result.AddressPostalCode = value.AddressPostalCode
	result.ExtraInfo = value.ExtraInfo
	result.SupervisorNotes = value.SupervisorNotes
	result.HealthInfo = value.HealthInfo
	result.PickupStatus = value.PickupStatus
	result.DepartureDays = value.DepartureDays
	result.AllowedDepartureModes = value.AllowedDepartureModes
	result.DepartureCompanionNote = value.DepartureCompanionNote
	result.DepartureCompanionDays = value.DepartureCompanionDays
	if value.DepartureBaseline != nil {
		b := value.DepartureBaseline
		result.DepartureBaseline = &models.DeparturePlanSnapshot{AllowedDepartureModes: b.AllowedDepartureModes, DepartureDays: b.DepartureDays, PickupDays: b.PickupDays, BusDays: b.BusDays}
	}
	result.PickupDays = value.PickupDays
	result.BusDays = value.BusDays
	result.Sick = value.Sick
	result.SickSince = value.SickSince
	result.Excused = value.Excused
	result.ExcusedSince = value.ExcusedSince
	result.Status = models.StudentStatus(value.Status)
	result.EnrolledFrom = value.EnrolledFrom
	result.EnrolledUntil = value.EnrolledUntil
	result.PhotoPath = value.PhotoPath
	result.PhotoConsentGivenAt = value.PhotoConsentGivenAt
	result.PhotoConsentGivenBy = value.PhotoConsentGivenBy
	result.AGBAcceptedAt = value.AGBAcceptedAt
	result.DataProcessingAcceptedAt = value.DataProcessingAcceptedAt
	result.EmailContactAcceptedAt = value.EmailContactAcceptedAt
	result.Person = DirectoryPersonRow(value.Person)
	return result
}
