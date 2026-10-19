package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	carePlanPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// RequestPickupPlans is the schedule surface used by request decisions.
// Requests neither edit free-standing notes nor manage unrelated exceptions.
type RequestPickupPlans interface {
	GetStudentPickupSchedules(context.Context, int64) ([]*careplan.PickupSchedule, error)
	DeleteStudentPickupSchedule(context.Context, int64) error
	UpsertStudentPickupSchedule(context.Context, *careplan.PickupSchedule) error
	HasBookedOfferingPickupForWeekday(context.Context, int64, int) (bool, error)
	GetEffectivePickupTimeForDate(context.Context, int64, timezone.Date) (*careplan.EffectivePickupTime, error)
}

// RequestArrivalPlans is the weekly arrival surface used by approvals.
type RequestArrivalPlans interface {
	GetStudentArrivalSchedules(context.Context, int64) ([]*careplan.ArrivalSchedule, error)
	DeleteStudentArrivalSchedule(context.Context, int64) error
	UpsertStudentArrivalSchedule(context.Context, *careplan.ArrivalSchedule) error
}

type RequestRecords interface {
	compose.RequestSubmissionRecords
	compose.RequestReadRecords
	compose.RequestEditRecords
	compose.RequestDecisionRecords
	compose.RequestCorrectionRecords
	carePlanPorts.PickupApprovalRecords
}
