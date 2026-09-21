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
	ActingStaffID(context.Context) (int64, error)
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
}

type PickupApprovalExcusal interface {
	Preview(context.Context, int64, calendar.Date, time.Time) ([]carerequests.Block, error)
	Sync(context.Context, int64) error
}

type PickupApprovalLocker interface {
	LockStudentAndExceptionDay(context.Context, int64, string) error
}
