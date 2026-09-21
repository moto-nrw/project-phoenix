package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type PickupExcusalStore interface {
	FindByID(context.Context, int64) (*careplan.PickupException, error)
	FindByStudentIDAndDate(context.Context, int64, calendar.Date) (*careplan.PickupException, error)
	FindUpcomingByStudentID(context.Context, int64) ([]*careplan.PickupException, error)
	Update(context.Context, *careplan.PickupException) error
}
type PartialAbsenceBlocks interface {
	ApplyPartialAbsence(context.Context, int64) (int, error)
	ReleasePartialAbsence(context.Context, int64) (int, error)
}
type PartialAbsencePreview interface {
	FindPartialAbsenceBlocks(context.Context, int64, calendar.Date, time.Time) ([]carerequests.Block, error)
}
type PickupExcusalLocker interface {
	LockStudent(context.Context, int64) error
	LockStudentAndExceptionDay(context.Context, int64, string) error
	IsNotFound(error) bool
}
type PickupDayExtension struct {
	StudentID         int64
	PickupExceptionID int64
	Date              string
	PreviousPickup    string
	Pickup            string
}
type PickupWeekdayExtension struct {
	StudentID      int64
	Weekday        int
	EffectiveFrom  string
	PreviousPickup string
	Pickup         string
}
type PickupExtensions interface {
	ClearPickupDayExtension(context.Context, int64, string) error
	RecordPickupDayExtension(context.Context, PickupDayExtension) error
	ClearPickupWeekdayExtension(context.Context, int64, int) error
	RecordPickupWeekdayExtension(context.Context, PickupWeekdayExtension) error
}
