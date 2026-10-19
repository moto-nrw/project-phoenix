package httpintegration_test

import (
	fixtures "github.com/moto-nrw/project-phoenix/modules/timetable/fixturetest"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The shared fixtures return their own row mappings (#2748). These carry a
// fixture row into the owner model the Timetable repositories write, field
// for field.

func ownerPickupException(row *testpkg.StudentPickupException) *fixtures.StudentPickupException {
	exception := fixtures.NewStudentPickupException(func(fixtureRow *fixtures.StudentPickupException) {
		fixtureRow.StudentID = row.StudentID
		fixtureRow.ExceptionDate = calendar.Date(row.ExceptionDate)
		fixtureRow.PickupTime = row.PickupTime
		fixtureRow.Reason = row.Reason
		fixtureRow.ExcusedFrom = row.ExcusedFrom
		fixtureRow.ExcusedReason = row.ExcusedReason
		fixtureRow.ExcusedCreatedBy = row.ExcusedCreatedBy
		fixtureRow.ExcusedOwnsPickupTime = row.ExcusedOwnsPickupTime
		fixtureRow.ExcusedAuto = row.ExcusedAuto
		fixtureRow.Source = row.Source
		fixtureRow.CreatedBy = row.CreatedBy
		fixtureRow.CreatedByGuardian = row.CreatedByGuardian
	})

	exception.ID, exception.CreatedAt, exception.UpdatedAt = row.ID, row.CreatedAt, row.UpdatedAt
	exception.TenantID = row.TenantID
	return exception
}

func ownerInstanceStudent(row *testpkg.InstanceStudent) *fixtures.InstanceStudent {
	student := fixtures.NewInstanceStudent(func(fixtureRow *fixtures.InstanceStudent) {
		fixtureRow.InstanceID = row.InstanceID
		fixtureRow.StudentID = row.StudentID
		fixtureRow.RoomID = row.RoomID
		fixtureRow.Status = row.Status
		fixtureRow.Substatus = row.Substatus
		fixtureRow.Note = row.Note
		fixtureRow.CheckedInAt = row.CheckedInAt
		fixtureRow.CheckedOutAt = row.CheckedOutAt
		fixtureRow.IsUnplanned = row.IsUnplanned
		fixtureRow.NotScheduled = row.NotScheduled
		fixtureRow.ManualStatusAt = row.ManualStatusAt
		fixtureRow.StudentStatusDayID = row.StudentStatusDayID
		fixtureRow.PickupExceptionID = row.PickupExceptionID
	})

	student.ID, student.CreatedAt, student.UpdatedAt = row.ID, row.CreatedAt, row.UpdatedAt
	student.TenantID = row.TenantID
	return student
}
