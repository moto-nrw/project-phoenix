package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// The owner record capabilities the roots bind when they build the Care Plan
// baseline, approval and excusal services (#4084).

// ArrivalBaselineRecords supplies the owner's records for the arrival
// baseline projection.
type ArrivalBaselineRecords interface {
	ListArrivalSchedules(context.Context, careplan.StudentScheduleFilter) ([]careplan.ArrivalSchedule, error)
	ListCareOfferings(context.Context, careplan.CareOfferingFilter) ([]careplan.CareOffering, error)
}

// PickupBaselineRecords supplies the owner's records for baseline projection.
type PickupBaselineRecords interface {
	ListPickupSchedules(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupSchedule, error)
	ListCareOfferings(context.Context, careplan.CareOfferingFilter) ([]careplan.CareOffering, error)
}

// PickupApprovalRecords supplies the owner's pickup and departure exceptions
// the request approvals write.
type PickupApprovalRecords interface {
	ListPickupExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupException, error)
	CreatePickupException(context.Context, careplan.PickupException) (careplan.PickupException, error)
	UpdatePickupException(context.Context, careplan.PickupException) error
	ListDepartureExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.DepartureException, error)
	SaveDepartureException(context.Context, careplan.DepartureException) (careplan.DepartureException, error)
	CreatePickupNote(context.Context, careplan.PickupNote) (careplan.PickupNote, error)
}

// PickupExcusalRecords is the existing owner persistence capability needed by
// the derived and manual excusal commands.
type PickupExcusalRecords interface {
	FindPickupException(context.Context, int64, bool) (careplan.PickupException, error)
	ListPickupExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupException, error)
	CreatePickupException(context.Context, careplan.PickupException) (careplan.PickupException, error)
	UpdatePickupException(context.Context, careplan.PickupException) error
	DeletePickupException(context.Context, int64) error
	ListStudentStatusDays(context.Context, careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, error)
	ListExcusedAbsenceRequests(context.Context, careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, error)
}
