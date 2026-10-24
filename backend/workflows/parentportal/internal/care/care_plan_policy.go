package care

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
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

// SameDayCutoff is Care Plan's same-day cutoff, passed unchanged into its
// request commands.
type SameDayCutoff = carerequests.SameDayCutoff

type CarePlanPolicy interface {
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
