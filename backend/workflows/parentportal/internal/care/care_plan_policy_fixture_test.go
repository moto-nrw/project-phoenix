package care

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type carePlanFixturePolicy struct{}

func (carePlanFixturePolicy) NewSameDayCutoff(clock string, now time.Time) (SameDayCutoff, error) {
	v, err := careplan.NewSameDayCutoff(clock, now)
	return NewCutoffSnapshot(v.Clock, v.Now, v.Closed), err
}
func (carePlanFixturePolicy) NewSameDayCutoffAt(clock string, now func() time.Time) (SameDayCutoff, error) {
	v, err := careplan.NewSameDayCutoffAt(clock, now)
	return NewCutoffSnapshot(v.Clock, v.Now, v.Closed), err
}
func (carePlanFixturePolicy) BookingViewDate(today, end calendar.Date) calendar.Date {
	return careplan.BookingViewDate(today, end)
}
func (carePlanFixturePolicy) Error(kind CarePlanError) error {
	switch kind {
	case CarePlanCourseRequestsDisabled:
		return careplan.ErrCourseRequestsDisabled
	case CarePlanOfferingChangeDisabled:
		return careplan.ErrOfferingChangeDisabled
	case CarePlanManualPartialAbsenceConflict:
		return careplan.ErrManualPartialAbsenceConflict
	case CarePlanGuardianPickupExceptionRaced:
		return careplan.ErrGuardianPickupExceptionRaced
	case CarePlanPickupExceptionStaffOwned:
		return careplan.ErrPickupExceptionStaffOwned
	case CarePlanStudentDataRequestFieldPending:
		return careplan.ErrStudentDataRequestFieldPending
	default:
		panic("unknown Care Plan error")
	}
}
