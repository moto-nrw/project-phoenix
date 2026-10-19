package care

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CarePlanError identifies only the owner's refusals this workflow consumes.
type CarePlanError string

const (
	CarePlanCourseRequestsDisabled         CarePlanError = "CourseRequestsDisabled"
	CarePlanOfferingChangeDisabled         CarePlanError = "OfferingChangeDisabled"
	CarePlanManualPartialAbsenceConflict   CarePlanError = "ManualPartialAbsenceConflict"
	CarePlanGuardianPickupExceptionRaced   CarePlanError = "GuardianPickupExceptionRaced"
	CarePlanPickupExceptionStaffOwned      CarePlanError = "PickupExceptionStaffOwned"
	CarePlanStudentDataRequestFieldPending CarePlanError = "StudentDataRequestFieldPending"
)

// SameDayCutoff retains an owner-bound judgment, including its lazy clock.
// Its zero value has no cutoff.
type SameDayCutoff struct {
	Clock  string
	Now    time.Time
	closed func(calendar.Date) bool
}

func (c SameDayCutoff) Closed(date calendar.Date) bool {
	if c.Clock == "" {
		return false
	}
	return c.closed(date)
}
func NewCutoffSnapshot(clock string, now time.Time, closed func(calendar.Date) bool) SameDayCutoff {
	return SameDayCutoff{Clock: clock, Now: now, closed: closed}
}

type CarePlanPolicy interface {
	NewSameDayCutoff(string, time.Time) (SameDayCutoff, error)
	NewSameDayCutoffAt(string, func() time.Time) (SameDayCutoff, error)
	BookingViewDate(calendar.Date, calendar.Date) calendar.Date
	Error(CarePlanError) error
}

// These values are stored protocol vocabulary, not business predicates.
type CareDayStatus string

const (
	CareDayScheduled                 CareDayStatus = "scheduled"
	CareDayNotScheduled              CareDayStatus = "not_scheduled"
	CareDayUnknown                   CareDayStatus = "unknown"
	CourseRequestsReasonNoPermission               = "no_permission"
	CourseRequestsReasonSchoolOff                  = "school_disabled"
)

func (p PickupException) HasManualPartialAbsence() bool { return p.ManualPartialAbsence }
