package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// PickupAdjustmentSchedules reads the stored weekly pickup and arrival rows
// a pickup adjustment compares with and replaces.
type PickupAdjustmentSchedules interface {
	ListPickupSchedules(ctx context.Context, filter careplan.StudentScheduleFilter) ([]careplan.PickupSchedule, error)
	ListArrivalSchedules(ctx context.Context, filter careplan.StudentScheduleFilter) ([]careplan.ArrivalSchedule, error)
}

// PickupAdjustmentStudents locks the People Directory students a pickup
// adjustment writes for.
type PickupAdjustmentStudents interface {
	// LockStudent takes the student row FOR UPDATE; a missing student is
	// (nil, nil).
	LockStudent(ctx context.Context, id int64) (*careplan.ScheduleStudent, error)
	// LockStudents takes the rows FOR UPDATE in the given order and returns
	// the existing ones by id.
	LockStudents(ctx context.Context, ids []int64) (map[int64]careplan.ScheduleStudent, error)
}

// PickupPlanAudit records a changed weekly pickup plan in the student's
// Änderungsprotokoll.
type PickupPlanAudit interface {
	RecordPickupPlanForActor(ctx context.Context, studentID int64, before, after, label, reason string, actorAccountID int64) error
}

// PickupAdjustmentSettings resolves whether a deviating plan needs an
// explicit resolution.
type PickupAdjustmentSettings interface {
	PickupOfferingReviewRequired(ctx context.Context) (bool, error)
}

// TenantUnitOfWork runs a pickup adjustment in the tenant transaction and
// asks the request transaction to roll back after a failed write.
type TenantUnitOfWork interface {
	TenantID(ctx context.Context) int64
	RunInTenantTx(ctx context.Context, fn func(context.Context) error) error
	MarkRollback(ctx context.Context)
}
