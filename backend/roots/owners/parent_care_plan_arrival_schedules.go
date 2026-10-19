package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func NewParentArrivalSchedules(source careplan.ArrivalScheduleService) parentportal.ArrivalSchedules {
	if source == nil {
		return nil
	}
	return parentportal.NewArrivalSchedules(parentportal.ArrivalSchedulesCallbacks{
		GetStudentArrivalExceptionForDate: func(ctx context.Context, id int64, date calendar.Date) (*parentportal.ArrivalExceptionSnapshot, error) {
			value, err := source.GetStudentArrivalExceptionForDate(ctx, id, date)
			return parentCarePlanPointer(value, parentCarePlanFromOwnerArrivalException), err
		},
		GetStudentArrivalScheduleForWeekday: func(ctx context.Context, id int64, weekday int) (*parentportal.ArrivalScheduleSnapshot, error) {
			value, err := source.GetStudentArrivalScheduleForWeekday(ctx, id, weekday)
			return parentCarePlanPointer(value, parentCarePlanFromOwnerArrivalSchedule), err
		},
		GetStudentArrivalSchedules: func(ctx context.Context, id int64) ([]*parentportal.ArrivalScheduleSnapshot, error) {
			value, err := source.GetStudentArrivalSchedules(ctx, id)
			return parentCarePlanSlice(value, func(value *careplan.ArrivalSchedule) *parentportal.ArrivalScheduleSnapshot {
				return parentCarePlanPointer(value, parentCarePlanFromOwnerArrivalSchedule)
			}), err
		},
	})
}
