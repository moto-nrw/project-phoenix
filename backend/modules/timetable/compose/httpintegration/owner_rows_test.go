package httpintegration_test

import (
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The shared fixtures return their own row mappings (#2748). These carry a
// fixture row into the owner model the Timetable repositories write, field
// for field.

func ownerPickupException(row *testpkg.StudentPickupException) *scheduleModels.StudentPickupException {
	exception := &scheduleModels.StudentPickupException{
		StudentID:             row.StudentID,
		ExceptionDate:         calendar.Date(row.ExceptionDate),
		PickupTime:            row.PickupTime,
		Reason:                row.Reason,
		ExcusedFrom:           row.ExcusedFrom,
		ExcusedReason:         row.ExcusedReason,
		ExcusedCreatedBy:      row.ExcusedCreatedBy,
		ExcusedOwnsPickupTime: row.ExcusedOwnsPickupTime,
		ExcusedAuto:           row.ExcusedAuto,
		Source:                row.Source,
		CreatedBy:             row.CreatedBy,
		CreatedByGuardian:     row.CreatedByGuardian,
	}
	exception.ID, exception.CreatedAt, exception.UpdatedAt = row.ID, row.CreatedAt, row.UpdatedAt
	exception.TenantID = row.TenantID
	return exception
}

func ownerInstanceStudent(row *testpkg.InstanceStudent) *scheduleModels.InstanceStudent {
	student := &scheduleModels.InstanceStudent{
		InstanceID:         row.InstanceID,
		StudentID:          row.StudentID,
		RoomID:             row.RoomID,
		Status:             row.Status,
		Substatus:          row.Substatus,
		Note:               row.Note,
		CheckedInAt:        row.CheckedInAt,
		CheckedOutAt:       row.CheckedOutAt,
		IsUnplanned:        row.IsUnplanned,
		NotScheduled:       row.NotScheduled,
		ManualStatusAt:     row.ManualStatusAt,
		StudentStatusDayID: row.StudentStatusDayID,
		PickupExceptionID:  row.PickupExceptionID,
	}
	student.ID, student.CreatedAt, student.UpdatedAt = row.ID, row.CreatedAt, row.UpdatedAt
	student.TenantID = row.TenantID
	return student
}
