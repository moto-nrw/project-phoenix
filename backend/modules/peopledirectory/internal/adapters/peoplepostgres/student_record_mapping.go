package users

import (
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// RetainedStudentModel reconstructs the complete retained read value, including
// canonical departure hydration and its update baseline. Mapping persistence
// values belongs to the owner adapter, not the composition.
func RetainedStudentModel(record peopleModule.StudentRecord) *userModels.Student {
	student := &userModels.Student{
		PersonID: record.PersonID, SchoolClass: record.SchoolClass, GroupID: record.GroupID,
		Status: userModels.StudentStatus(record.Status),

		AddressStreet:     record.AddressStreet,
		AddressCity:       record.AddressCity,
		AddressPostalCode: record.AddressPostalCode,

		ExtraInfo:       record.ExtraInfo,
		SupervisorNotes: record.SupervisorNotes,
		HealthInfo:      record.HealthInfo,
		PickupStatus:    record.PickupStatus,

		DepartureDays:          record.DepartureDays,
		AllowedDepartureModes:  record.AllowedDepartureModes,
		PickupDays:             record.PickupDays,
		BusDays:                record.BusDays,
		DepartureCompanionNote: record.DepartureCompanionNote,

		Sick:         record.Sick,
		SickSince:    record.SickSince,
		Excused:      record.Excused,
		ExcusedSince: record.ExcusedSince,

		PhotoPath:           record.PhotoPath,
		PhotoConsentGivenAt: record.PhotoConsentGivenAt,
		PhotoConsentGivenBy: record.PhotoConsentGivenBy,

		AGBAcceptedAt:            record.AGBAcceptedAt,
		DataProcessingAcceptedAt: record.DataProcessingAcceptedAt,
		EmailContactAcceptedAt:   record.EmailContactAcceptedAt,
	}
	student.ID, student.CreatedAt, student.UpdatedAt = record.ID, record.CreatedAt, record.UpdatedAt
	student.SetTenantID(record.TenantID)
	student.EnrolledFrom = userModels.OptionalCalendarDate(record.EnrolledFrom)
	student.EnrolledUntil = userModels.OptionalCalendarDate(record.EnrolledUntil)
	hydrateRetainedDeparturePlan(student)
	return student
}

func hydrateRetainedDeparturePlan(student *userModels.Student) {
	effective := userModels.DeparturePlan{
		AllowedDepartureModes: student.AllowedDepartureModes,
		DepartureDays:         student.DepartureDays,
		BusDays:               student.BusDays,
		PickupDays:            student.PickupDays,
	}.Effective()
	student.AllowedDepartureModes = effective.AllowedDepartureModes
	student.DepartureDays = effective.DepartureDays
	student.BusDays = effective.BusDays
	student.PickupDays = effective.PickupDays
	student.SnapshotDeparturePlan()
}
