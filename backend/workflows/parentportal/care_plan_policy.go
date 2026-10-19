package parentportal

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

type CarePlanError care.CarePlanError

const (
	CarePlanCourseRequestsDisabled         CarePlanError = CarePlanError(care.CarePlanCourseRequestsDisabled)
	CarePlanOfferingChangeDisabled         CarePlanError = CarePlanError(care.CarePlanOfferingChangeDisabled)
	CarePlanManualPartialAbsenceConflict   CarePlanError = CarePlanError(care.CarePlanManualPartialAbsenceConflict)
	CarePlanGuardianPickupExceptionRaced   CarePlanError = CarePlanError(care.CarePlanGuardianPickupExceptionRaced)
	CarePlanPickupExceptionStaffOwned      CarePlanError = CarePlanError(care.CarePlanPickupExceptionStaffOwned)
	CarePlanStudentDataRequestFieldPending CarePlanError = CarePlanError(care.CarePlanStudentDataRequestFieldPending)
)

type SameDayCutoffSnapshot struct {
	Clock  string
	Now    time.Time
	Closed func(calendar.Date) bool
}
type CarePlanPolicyCallbacks struct {
	NewSameDayCutoff   func(string, time.Time) (SameDayCutoffSnapshot, error)
	NewSameDayCutoffAt func(string, func() time.Time) (SameDayCutoffSnapshot, error)
	BookingViewDate    func(calendar.Date, calendar.Date) calendar.Date
	Error              func(CarePlanError) error
}
type CarePlanPolicy interface{ care.CarePlanPolicy }

func NewCarePlanPolicy(callbacks CarePlanPolicyCallbacks) CarePlanPolicy {
	return carePlanPolicy{callbacks}
}

type carePlanPolicy struct{ callbacks CarePlanPolicyCallbacks }

func (p carePlanPolicy) NewSameDayCutoff(clock string, now time.Time) (care.SameDayCutoff, error) {
	v, err := p.callbacks.NewSameDayCutoff(clock, now)
	return care.NewCutoffSnapshot(v.Clock, v.Now, v.Closed), err
}
func (p carePlanPolicy) NewSameDayCutoffAt(clock string, now func() time.Time) (care.SameDayCutoff, error) {
	v, err := p.callbacks.NewSameDayCutoffAt(clock, now)
	return care.NewCutoffSnapshot(v.Clock, v.Now, v.Closed), err
}
func (p carePlanPolicy) BookingViewDate(today, end calendar.Date) calendar.Date {
	return p.callbacks.BookingViewDate(today, end)
}
func (p carePlanPolicy) Error(kind care.CarePlanError) error {
	return p.callbacks.Error(CarePlanError(kind))
}
