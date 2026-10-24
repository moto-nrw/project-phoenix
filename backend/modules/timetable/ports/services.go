package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The services below are what the roots hold of the Timetable application;
// the composition constructs them.

// InstanceLifecycle is the instance lifecycle as the roots wire it: its
// public capability plus the views and announcements other owners and the
// composition bind.
type InstanceLifecycle interface {
	timetable.InstanceLifecycleCapability
	AnnounceStaffingChanged(tenantID int64, source string) error
	CompleteInstanceRow(ctx context.Context, instanceID int64) (*scheduleModels.ActivityInstance, error)
	DeviationLifecycle() DeviationLifecycle
	GetPlannedStudentIDsByDate(ctx context.Context, studentIDs []int64, date calendar.Date) ([]int64, error)
	QueueActivityUpdates(ctx context.Context, touched timetable.TouchedActivities)
	SeriesDeviations() SeriesDeviations
}

// RosterReconciler keeps the future rosters in step with student transitions
// and sourced template enrollments.
type RosterReconciler interface {
	CurrentRosterBaseline(ctx context.Context) (int64, error)
	ReconcileSourcedTemplateRosters(ctx context.Context, templateID int64, studentIDs []int64, from calendar.Date, prior []timetable.RosterEnrollment) (int, int, error)
	RemoveStudentsFromFutureRosters(ctx context.Context, transitionID int64, studentIDs []int64) error
	RestoreStudentsToFutureRosters(ctx context.Context, transitionID int64, studentIDs []int64, baselineInstanceID *int64) error
}

// AttendanceHistory reads a student's planned slots with their attendance.
type AttendanceHistory interface {
	HasPlannedSlots(ctx context.Context, from, to string) (bool, error)
	Slots(ctx context.Context, studentID int64, from, to string) ([]*AttendanceHistorySlot, error)
}

// InstanceStatusSlots marks and releases the slots a status day covers.
type InstanceStatusSlots interface {
	ApplyStatusDay(ctx context.Context, studentID int64, date string, statusDayID int64, substatus string) (int, error)
	ReleaseStatusDay(ctx context.Context, statusDayID int64) (int, error)
}
