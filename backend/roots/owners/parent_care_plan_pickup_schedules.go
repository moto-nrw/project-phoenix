package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func NewParentPickupSchedules(source careplan.PickupScheduleService) parentportal.PickupSchedules {
	if source == nil {
		return nil
	}
	return parentportal.NewPickupSchedules(parentportal.PickupSchedulesCallbacks{
		GetEffectivePickupTimeForDate: func(ctx context.Context, id int64, date calendar.Date) (*parentportal.EffectivePickupTimeSnapshot, error) {
			value, err := source.GetEffectivePickupTimeForDate(ctx, id, date)
			return parentCarePlanPointer(value, parentCarePlanFromOwnerEffectivePickupTime), err
		},
		GetStudentPickupSchedules: func(ctx context.Context, id int64) ([]*parentportal.PickupScheduleSnapshot, error) {
			value, err := source.GetStudentPickupSchedules(ctx, id)
			return parentCarePlanSlice(value, func(value *careplan.PickupSchedule) *parentportal.PickupScheduleSnapshot {
				return parentCarePlanPointer(value, parentCarePlanFromOwnerPickupSchedule)
			}), err
		},
	})
}
