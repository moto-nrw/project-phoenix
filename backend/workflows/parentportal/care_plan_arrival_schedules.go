package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

type ArrivalSchedules interface{ care.ArrivalSchedules }
type ArrivalSchedulesCallbacks struct {
	GetStudentArrivalExceptionForDate   func(ctx context.Context, id int64, date calendar.Date) (*ArrivalExceptionSnapshot, error)
	GetStudentArrivalScheduleForWeekday func(ctx context.Context, id int64, weekday int) (*ArrivalScheduleSnapshot, error)
	GetStudentArrivalSchedules          func(ctx context.Context, id int64) ([]*ArrivalScheduleSnapshot, error)
}

func NewArrivalSchedules(callbacks ArrivalSchedulesCallbacks) ArrivalSchedules {
	return arrivalSchedules{callbacks}
}

type arrivalSchedules struct{ callbacks ArrivalSchedulesCallbacks }

func (a arrivalSchedules) GetStudentArrivalExceptionForDate(ctx context.Context, id int64, date calendar.Date) (*care.ArrivalException, error) {
	return a.callbacks.GetStudentArrivalExceptionForDate(ctx, id, date)
}
func (a arrivalSchedules) GetStudentArrivalScheduleForWeekday(ctx context.Context, id int64, weekday int) (*care.ArrivalSchedule, error) {
	return a.callbacks.GetStudentArrivalScheduleForWeekday(ctx, id, weekday)
}
func (a arrivalSchedules) GetStudentArrivalSchedules(ctx context.Context, id int64) ([]*care.ArrivalSchedule, error) {
	return a.callbacks.GetStudentArrivalSchedules(ctx, id)
}
