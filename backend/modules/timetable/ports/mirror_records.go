package ports

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// MirrorInstanceRecords also stages complete owner values in contract suites.
type MirrorInstanceRecords struct {
	*MirrorInstances
	Create   func(context.Context, *timetable.ScheduledInstance) error
	FindByID func(context.Context, any) (*timetable.ScheduledInstance, error)
}
type MirrorParticipantRecords struct {
	*MirrorParticipants
	Create                                           func(context.Context, *timetable.ScheduledParticipant) error
	FindByID                                         func(context.Context, any) (*timetable.ScheduledParticipant, error)
	FindInstancesWithAttendanceByStudentAndDateRange func(context.Context, int64, calendar.Date, calendar.Date) ([]*timetable.StudentWeekEntry, error)
}
