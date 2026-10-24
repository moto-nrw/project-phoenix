package owners

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func NewParentCarePlanPolicy() parentportal.CarePlanPolicy {
	return parentportal.NewCarePlanPolicy(parentportal.CarePlanPolicyCallbacks{
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
