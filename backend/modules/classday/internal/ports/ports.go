// Package ports declares the consumer-owned read seams of the class-day
// projection that are not yet served by a public owner facade: the care-day
// derivation, the effective arrival/pickup times and pickup baselines of the
// retained schedule services, the tenant settings, the caller's read access,
// the pure care-plan rules, and the enrollment report the school portal shows.
// modules/classday/compose binds them; the application never sees the legacy
// types behind them.
package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/classday"
)

// CareDay is the derived per-child, per-day care-plan verdict. The values
// mirror the schedule domain's CareDayStatus so the same string reaches the
// exports ("cancelled" doubles as the registered-absence substatus).
type CareDay string

const (
	// CareDayScheduled — the care plan puts the child in the OGS that day.
	CareDayScheduled CareDay = "scheduled"
	// CareDayNotScheduled — the child is not booked into care that day.
	CareDayNotScheduled CareDay = "not_scheduled"
	// CareDayCancelled — somebody explicitly cancelled the day ("Kommt heute
	// nicht"): a reported absence, not a non-booking.
	CareDayCancelled CareDay = "cancelled"
	// CareDayUnknown — no care plan on file at all; treated as expected.
	CareDayUnknown CareDay = "unknown"
)

// Expected reports whether a child with this verdict belongs in the expected
// count. Both "not booked" and "cancelled" say the child is not coming.
func (s CareDay) Expected() bool {
	return s != CareDayNotScheduled && s != CareDayCancelled
}

// CareDays resolves the care-day verdict of many children on one date.
type CareDays interface {
	// ResolveForDate returns the verdict of every requested child. Children
	// without an entry are unknown.
	ResolveForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]CareDay, error)
}

// EffectiveTimes reads the effective (plan plus day exceptions) arrival and
// pickup clock times of the requested children on one date. A nil entry
// means the child has no such time that day.
type EffectiveTimes interface {
	PickupTimes(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*time.Time, error)
	ArrivalTimes(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*time.Time, error)
}

// PickupBaselines returns the recurring (exception-free) pickup time of the
// date's weekday as "HH:MM" per child; children without a plan are absent.
type PickupBaselines interface {
	RegularPickupTimes(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]string, error)
}

// Settings reads the tenant configuration the lists depend on.
type Settings interface {
	// TimetableEnabled resolves the timetable feature flag.
	TimetableEnabled(ctx context.Context) (bool, error)
	// PickupCutoffs resolves the Ganztag short-day and long-day cutoffs as
	// stored ("HH:MM", unvalidated) from one consistent snapshot: the binding
	// holds the shared advisory lock the cutoff writer takes exclusively, so
	// a concurrent two-write change can never hand the reader one cutoff
	// from before and the sibling from after (#1565 review pass 12).
	PickupCutoffs(ctx context.Context) (short, long string, err error)
}

// ReadAccess decides whether the caller may see children by name.
type ReadAccess interface {
	// CanReadStudents is true for administrators and for accounts with a
	// staff record in the tenant. Operational failures propagate so a
	// request fails loudly instead of returning an empty list.
	CanReadStudents(ctx context.Context) (bool, error)
}

// RosterFacts are the roster-row columns the care-day rule reads.
type RosterFacts struct {
	Status             string
	NotScheduled       bool
	ManualStatusAt     *time.Time
	StudentStatusDayID *int64
	PickupExceptionID  *int64
}

// StudentFacts are the student columns the enrollment rule reads.
type StudentFacts struct {
	Status        string
	EnrolledFrom  *timezone.Date
	EnrolledUntil *timezone.Date
}

// Rules are the pure care-plan rules every reader of the roster shares, so
// the slot lists, the day-log rosters and the statistics report cannot drift
// apart (#1565, #2606). They stay with their owners; the binding forwards.
type Rules interface {
	// RowCareDay folds the frozen not_scheduled marker and the live care-day
	// verdict into the single answer every reader shares (#1747).
	RowCareDay(instanceCompleted bool, row RosterFacts, planVerdict CareDay) CareDay
	// EnrolledOn reports whether the student is enrolled on date, given
	// today's calendar day for the immediate-activation exception.
	EnrolledOn(student StudentFacts, date, today timezone.Date) bool
}

// Caller resolves the authenticated account's school-portal identity.
type Caller interface {
	// AssignedClasses returns the school classes assigned to the caller.
	AssignedClasses(ctx context.Context) ([]string, error)
	// StaffID returns the caller's users.staff row;
	// classday.ErrStaffRecordRequired when the account has none.
	StaffID(ctx context.Context) (int64, error)
}

// DayReports builds the per-class day view.
type DayReports interface {
	// ClassDay builds the report; classday.ErrInvalidReportFilter when the
	// class is empty.
	ClassDay(ctx context.Context, schoolClass string, date timezone.Date, actor classday.Actor) (*classday.DayReport, error)
}

// ArrivalExceptions is the write seam for class-wide arrival day exceptions.
// Errors are the classday sentinels.
type ArrivalExceptions interface {
	SchoolMayWrite(ctx context.Context) (bool, error)
	ListForClass(ctx context.Context, schoolClass string, from, to timezone.Date) ([]classday.ArrivalException, error)
	Set(ctx context.Context, in classday.ArrivalExceptionWrite) (*classday.ArrivalException, error)
	Clear(ctx context.Context, schoolClass string, date timezone.Date) error
	EarliestBlockStart(ctx context.Context, schoolClass string, date timezone.Date) (string, error)
}
