package active

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// OverviewWorkSchedules provides both range rows and the independent history
// signal used to decide whether an assigned work-time model may stand in.
type OverviewWorkSchedules interface {
	TargetsByStaff(context.Context, []int64, timezone.Date, timezone.Date) (map[int64]WorkScheduleTargets, error)
	FindStaffIDsWithScheduleHistory(context.Context, []int64) (map[int64]bool, error)
}

type OverviewWorkTimeModels interface {
	FindByIDs(context.Context, []int64) ([]*WorkTimeTargetModel, error)
}
