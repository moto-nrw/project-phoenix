package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

type PickupSchedules interface{ care.PickupSchedules }
type PickupSchedulesCallbacks struct {
	GetEffectivePickupTimeForDate func(ctx context.Context, id int64, date calendar.Date) (*EffectivePickupTimeSnapshot, error)
	GetStudentPickupSchedules     func(ctx context.Context, id int64) ([]*PickupScheduleSnapshot, error)
}

func NewPickupSchedules(callbacks PickupSchedulesCallbacks) PickupSchedules {
	return pickupSchedules{callbacks}
}

type pickupSchedules struct{ callbacks PickupSchedulesCallbacks }

func (a pickupSchedules) GetEffectivePickupTimeForDate(ctx context.Context, id int64, date calendar.Date) (*care.EffectivePickupTime, error) {
	value, err := a.callbacks.GetEffectivePickupTimeForDate(ctx, id, date)
	return carePlanValuePointer(value, carePlanToAppEffectivePickupTime), err
}
func (a pickupSchedules) GetStudentPickupSchedules(ctx context.Context, id int64) ([]*care.PickupSchedule, error) {
	value, err := a.callbacks.GetStudentPickupSchedules(ctx, id)
	return carePlanValueSlice(value, func(v *PickupScheduleSnapshot) *care.PickupSchedule {
		return carePlanValuePointer(v, carePlanToAppPickupSchedule)
	}), err
}
