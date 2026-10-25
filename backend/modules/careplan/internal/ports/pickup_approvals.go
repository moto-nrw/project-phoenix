package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type PickupApprovalDirectory interface {
	EnrolledUntil(context.Context, int64) (*calendar.Date, error)
	// ValidatePickupPerson locks and rechecks a one-day collector before the
	// approval writes the dated pickup note.
	ValidatePickupPerson(context.Context, int64, int64) error
	// CurrentStaffID is the caller's staff profile; found is false when the
	// caller has none.
	CurrentStaffID(context.Context) (staffID int64, found bool, err error)
}

type PickupApprovalPresence interface {
	LockStudentAttendance(context.Context, int64) error
	AttendanceCompletion(context.Context, int64, calendar.Date) (open, completed bool, err error)
}

type PickupApprovalExceptions interface {
	FindForDate(context.Context, int64, calendar.Date) (*careplan.PickupException, error)
	Create(context.Context, careplan.PickupException) (int64, error)
	Update(context.Context, careplan.PickupException) error
	IsUniqueViolation(error) bool
	// FindDepartureForDate and SaveDeparture write the day Abholart (#3945)
	// under the same care-day lock as the pickup exception.
	FindDepartureForDate(context.Context, int64, calendar.Date) (*careplan.DepartureException, error)
	SaveDeparture(context.Context, careplan.DepartureException) (int64, error)
	// CreateNote writes the dated pickup note naming the day's collector
	// (#4780).
	CreateNote(context.Context, careplan.PickupNote) (int64, error)
}

type PickupApprovalExcusal interface {
	Preview(context.Context, int64, calendar.Date, time.Time) ([]carerequests.Block, error)
	Sync(context.Context, int64) error
}

type PickupApprovalLocker interface {
	LockStudentAndExceptionDay(context.Context, int64, string) error
}
