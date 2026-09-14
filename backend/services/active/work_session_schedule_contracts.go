package active

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/config"
)

// WorkSessionSchedules is the schedule capability used by work-session
// enforcement, summaries, and effective-dated schedule replacement.
type WorkSessionSchedules interface {
	GetByStaffIDAndDate(context.Context, int64, config.CalendarDate) ([]*config.StaffWorkSchedule, error)
	FindByStaffIDsValidInRange(context.Context, []int64, config.CalendarDate, config.CalendarDate) ([]*config.StaffWorkSchedule, error)
	GetCurrentByStaffID(context.Context, int64) ([]*config.StaffWorkSchedule, error)
	ReplaceSchedule(context.Context, int64, []*config.StaffWorkSchedule, config.CalendarDate) error
}

type WorkSessionTimeModels interface {
	FindByID(context.Context, int64) (*config.WorkTimeModel, error)
	Create(context.Context, *config.WorkTimeModel, []*config.WorkTimeModelEntry) error
}
