package application

import (
	"errors"

	users "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// The value translation of the retained student write path (#3349): the
// retained callers carry users.Student rows, the owner's write command takes
// and returns its own record. The composition's write seam carries values
// across with these and decides nothing.

// StudentWriteFromModel reads the plan and the baseline off the model. The
// baseline is only present when the read that produced this struct hydrated the
// plan; without one the owner takes the supplied fields at face value.
func StudentWriteFromModel(student *users.Student) peopledirectory.StudentWrite {
	write := peopledirectory.StudentWrite{
		Record: studentModelToRecord(student),
		Plan: peopledirectory.StudentPlan{
			AllowedDepartureModes: student.AllowedDepartureModes,
			DepartureDays:         student.DepartureDays,
			BusDays:               student.BusDays,
			PickupDays:            student.PickupDays,
			PickupStatus:          student.PickupStatus,
		},
		CompanionNote: student.DepartureCompanionNote,
		NoteSupplied:  student.DepartureCompanionNote != nil,
	}
	if baseline := student.DepartureBaseline; baseline != nil {
		write.Baseline = &peopledirectory.StudentPlan{
			AllowedDepartureModes: baseline.AllowedDepartureModes,
			DepartureDays:         baseline.DepartureDays,
			BusDays:               baseline.BusDays,
			PickupDays:            baseline.PickupDays,
		}
	}
	return write
}

func studentModelToRecord(student *users.Student) peopledirectory.StudentRecord {
	record := peopledirectory.StudentRecord{
		ID: student.ID, TenantID: student.GetTenantID(),
		PersonID: student.PersonID, SchoolClass: student.SchoolClass,
		GroupID: student.GroupID, Status: string(student.Status),

		AddressStreet:     student.AddressStreet,
		AddressCity:       student.AddressCity,
		AddressPostalCode: student.AddressPostalCode,

		ExtraInfo:       student.ExtraInfo,
		SupervisorNotes: student.SupervisorNotes,
		HealthInfo:      student.HealthInfo,
		PickupStatus:    student.PickupStatus,

		DepartureCompanionNote: student.DepartureCompanionNote,

		Sick:         student.Sick,
		SickSince:    student.SickSince,
		Excused:      student.Excused,
		ExcusedSince: student.ExcusedSince,

		PhotoPath:           student.PhotoPath,
		PhotoConsentGivenAt: student.PhotoConsentGivenAt,
		PhotoConsentGivenBy: student.PhotoConsentGivenBy,

		AGBAcceptedAt:            student.AGBAcceptedAt,
		DataProcessingAcceptedAt: student.DataProcessingAcceptedAt,
		EmailContactAcceptedAt:   student.EmailContactAcceptedAt,
	}
	if student.EnrolledFrom != nil {
		record.EnrolledFrom = student.EnrolledFrom.String()
	}
	if student.EnrolledUntil != nil {
		record.EnrolledUntil = student.EnrolledUntil.String()
	}
	return record
}

// ApplyStudentRecord writes the stored row back onto the caller's struct and
// records the persisted plan as the new baseline: the in-memory plan now equals
// the stored one, so reusing the same instance for a second write must not make
// this write's own result look like a pending caller change.
func ApplyStudentRecord(student *users.Student, record peopledirectory.StudentRecord) {
	// Field by field rather than replacing the struct: the caller's value also
	// carries things the row does not — a preloaded Person, the companion days
	// it marked — and a write must not take those away.
	student.ID, student.CreatedAt, student.UpdatedAt = record.ID, record.CreatedAt, record.UpdatedAt
	student.SetTenantID(record.TenantID)
	student.PersonID, student.SchoolClass = record.PersonID, record.SchoolClass
	student.GroupID, student.Status = record.GroupID, users.StudentStatus(record.Status)
	student.EnrolledFrom = users.OptionalCalendarDate(record.EnrolledFrom)
	student.EnrolledUntil = users.OptionalCalendarDate(record.EnrolledUntil)

	student.AllowedDepartureModes = record.AllowedDepartureModes
	student.DepartureDays = record.DepartureDays
	student.BusDays = record.BusDays
	student.PickupDays = record.PickupDays
	student.PickupStatus = record.PickupStatus
	student.DepartureCompanionNote = record.DepartureCompanionNote

	// The in-memory plan now equals the stored one, so it is also the new
	// baseline: reusing the same value for a second write must not make this
	// write's own result look like a pending caller change.
	student.SnapshotDeparturePlan()
}

// RetainedStudentWriteError restates the owner's sentinels in the vocabulary
// the retained callers branch on.
func RetainedStudentWriteError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, peopledirectory.ErrCompanionWouldLoseDeparture):
		return users.ErrCompanionWouldLoseDeparture
	case errors.Is(err, peopledirectory.ErrCompanionLockBusy):
		return users.ErrCompanionLockBusy
	case errors.Is(err, peopledirectory.ErrStudentNotFound):
		return users.ErrStudentRowMissing
	default:
		return err
	}
}

// RetainedStudentReadError is the retained not-found shape of a child read
// under op: a missing or invalid child becomes the repository's missing-row
// error; every other error passes through.
func RetainedStudentReadError(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, peopledirectory.ErrStudentNotFound) || errors.Is(err, peopledirectory.ErrInvalidStudent) {
		return peoplePorts.MissingStudentError(op)
	}
	return err
}
