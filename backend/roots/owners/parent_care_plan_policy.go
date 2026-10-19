package owners

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func NewParentCarePlanPolicy() parentportal.CarePlanPolicy {
	return parentportal.NewCarePlanPolicy(parentportal.CarePlanPolicyCallbacks{
		NewSameDayCutoff: func(clock string, now time.Time) (parentportal.SameDayCutoffSnapshot, error) {
			v, err := careplan.NewSameDayCutoff(clock, now)
			return parentCutoffSnapshot(v), err
		},
		NewSameDayCutoffAt: func(clock string, now func() time.Time) (parentportal.SameDayCutoffSnapshot, error) {
			v, err := careplan.NewSameDayCutoffAt(clock, now)
			return parentCutoffSnapshot(v), err
		},
		BookingViewDate: careplan.BookingViewDate,
		Error: func(kind parentportal.CarePlanError) error {
			switch kind {
			case parentportal.CarePlanCourseRequestsDisabled:
				return careplan.ErrCourseRequestsDisabled
			case parentportal.CarePlanOfferingChangeDisabled:
				return careplan.ErrOfferingChangeDisabled
			case parentportal.CarePlanManualPartialAbsenceConflict:
				return careplan.ErrManualPartialAbsenceConflict
			case parentportal.CarePlanGuardianPickupExceptionRaced:
				return careplan.ErrGuardianPickupExceptionRaced
			case parentportal.CarePlanPickupExceptionStaffOwned:
				return careplan.ErrPickupExceptionStaffOwned
			case parentportal.CarePlanStudentDataRequestFieldPending:
				return careplan.ErrStudentDataRequestFieldPending
			default:
				panic("parent: unknown Care Plan error kind")
			}
		},
	})
}

func parentCutoffSnapshot(value careplan.SameDayCutoff) parentportal.SameDayCutoffSnapshot {
	return parentportal.SameDayCutoffSnapshot{Clock: value.Clock, Now: value.Now, Closed: value.Closed}
}
