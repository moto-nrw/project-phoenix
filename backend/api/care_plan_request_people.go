package api

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// RequestPeople keeps request decisions on the directory's tenant-scoped
// reads and companion-aware write path.
type RequestPeople interface {
	FindStudentRecord(context.Context, int64) (peopledirectory.StudentRecord, error)
	FindStudentRecordForMutation(context.Context, int64) (peopledirectory.StudentRecord, error)
	FindPerson(context.Context, int64) (peopledirectory.Person, error)
	UpdateStudent(context.Context, peopledirectory.StudentWrite) (peopledirectory.StudentRecord, error)
}

func requestDeparturePlan(student peopledirectory.StudentRecord) peopledirectory.StudentPlan {
	return (peopledirectory.StudentPlan{
		AllowedDepartureModes: student.AllowedDepartureModes,
		DepartureDays:         student.DepartureDays,
		BusDays:               student.BusDays,
		PickupDays:            student.PickupDays,
	}).Effective()
}

// Only the retained audit contract needs a model row. Capture the effective
// plan both before the write and from the directory's persisted result.
func requestStudentAuditSnapshot(student peopledirectory.StudentRecord) *peopledirectory.StudentState {
	plan := requestDeparturePlan(student)
	row := &peopledirectory.StudentState{
		Status:                 string(student.Status),
		SupervisorNotes:        student.SupervisorNotes,
		ExtraInfo:              student.ExtraInfo,
		HealthInfo:             student.HealthInfo,
		PickupStatus:           student.PickupStatus,
		DepartureCompanionNote: student.DepartureCompanionNote,
		AllowedDepartureModes:  plan.AllowedDepartureModes,
		DepartureDays:          plan.DepartureDays,
		EnrolledUntil:          peopledirectory.OptionalCalendarDate(student.EnrolledUntil),
	}
	row.ID = student.ID
	return row
}

func requestStudentReadError(err error) error {
	if errors.Is(err, peopledirectory.ErrStudentNotFound) || errors.Is(err, peopledirectory.ErrInvalidStudent) {
		return peoplePorts.MissingStudentError("care request student")
	}
	return err
}

func requestStudentWriteError(err error) error {
	switch {
	case errors.Is(err, peopledirectory.ErrCompanionWouldLoseDeparture):
		return peopledirectory.TranslateStudentWriteError(err)
	case errors.Is(err, peopledirectory.ErrCompanionLockBusy):
		return peopledirectory.TranslateStudentWriteError(err)
	case errors.Is(err, peopledirectory.ErrStudentNotFound):
		return peopledirectory.ErrStudentRowMissing
	default:
		return err
	}
}
